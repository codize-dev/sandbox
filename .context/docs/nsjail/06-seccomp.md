# System Call Filtering with Seccomp-BPF

nsjail uses seccomp-bpf (Secure Computing Mode with Berkeley Packet Filter) to filter system calls. Policies are written using the [Kafel](https://github.com/google/kafel) language.

## Configuration Fields

| Field | Type | Default | CLI | Description |
|-------|------|---------|-----|-------------|
| `seccomp_policy_file` | string | — | `--seccomp_policy FILE` / `-P FILE` | Path to a Kafel policy file |
| `seccomp_string` | repeated string | — | `--seccomp_string POLICY` | Inline Kafel policy string (multiple values are concatenated with newlines and compiled together as a single policy) |
| `seccomp_log` | bool | `false` | `--seccomp_log` | Use `SECCOMP_FILTER_FLAG_LOG` (kernel 4.14 or later) |
| `seccomp_unotify` | bool | `false` | `--seccomp_unotify` | Use `SECCOMP_RET_USER_NOTIF` to trace accessed files and network sockets |
| `seccomp_unotify_report` | string | — | `--seccomp_unotify_report FILE` | File to write the seccomp_unotify report to |

`seccomp_policy_file` and `seccomp_string` cannot be specified at the same time.

## How seccomp-bpf Works

The Linux kernel's seccomp BPF mode attaches a BPF program to a process so that it is invoked on every system call. The BPF program receives the following `seccomp_data` struct:

```c
struct seccomp_data {
    int nr;                    // system call number
    __u32 arch;                // architecture
    __u64 instruction_pointer; // instruction pointer
    __u64 args[6];             // first 6 arguments
};
```

The BPF program returns one of the following values:

| Return Value | Behavior |
|-------------|----------|
| `SECCOMP_RET_ALLOW` | Allow the system call |
| `SECCOMP_RET_KILL` / `SECCOMP_RET_KILL_THREAD` | Kill the thread |
| `SECCOMP_RET_KILL_PROCESS` | Kill the entire process |
| `SECCOMP_RET_ERRNO(n)` | Return error n to userspace |
| `SECCOMP_RET_TRAP(n)` | Send SIGSYS |
| `SECCOMP_RET_TRACE(n)` | Notify the ptrace tracer |
| `SECCOMP_RET_LOG` | Log and allow (kernel 4.14 or later) |
| `SECCOMP_RET_USER_NOTIF` | Notify a userspace supervisor |

## nsjail Implementation

### Policy Compilation (`sandbox.cc: preparePolicy()`)

- Called once at startup
- Compiles the policy into `struct sock_fprog` BPF bytecode using the Kafel library
- The compiled result is stored in `nsj->seccomp_fprog`
- When `seccomp_unotify` is enabled, it also compiles an internally-generated unotify Kafel policy (built by `unotify::buildKafelPolicy()`) into `nsj->seccomp_unotify_fprog`

### Resource Release (`sandbox.cc: closePolicy()`)

`closePolicy()` is called when nsjail exits to free the memory of the compiled BPF filter (`nsj->seccomp_fprog`) and the unotify BPF filter (`nsj->seccomp_unotify_fprog`).

### Policy Application (`sandbox.cc: applyPolicy()`)

`applyPolicy(nsj_t* nsj, int pipefd)` is called inside the child process after all confinement setup is complete, near the end before `execv()`. This is the **final step**. Note that when `seccomp_unotify` is enabled, there is a sync wait between `applyPolicy()` and `execv()`. It first calls `installUnotifyFilter()` to install the unotify filter when both `pipefd != -1` and `seccomp_unotify` is enabled, then delegates to `prepareAndCommit()` for the standard seccomp filter.

- Without `seccomp_log` (via `prepareAndCommit()`):
  ```c
  prctl(PR_SET_NO_NEW_PRIVS, 1);
  prctl(PR_SET_SECCOMP, SECCOMP_MODE_FILTER, &fprog);
  ```

- With `seccomp_log` (via `prepareAndCommit()`):
  ```c
  prctl(PR_SET_NO_NEW_PRIVS, 1);
  syscall(__NR_seccomp, SECCOMP_SET_MODE_FILTER,
          SECCOMP_FILTER_FLAG_TSYNC | SECCOMP_FILTER_FLAG_LOG, &fprog);
  ```

`SECCOMP_FILTER_FLAG_TSYNC` synchronizes the filter to all threads.

### Unotify Filter Installation (`sandbox.cc: installUnotifyFilter()`)

`installUnotifyFilter(nsj_t* nsj, int pipefd)` installs a separate BPF filter for the seccomp user notification feature. It is called by `applyPolicy()` before the standard seccomp filter when `seccomp_unotify` is enabled.

- Sets `PR_SET_NO_NEW_PRIVS` unconditionally
- Installs the unotify BPF program (`nsj->seccomp_unotify_fprog`) via `seccomp(SECCOMP_SET_MODE_FILTER, SECCOMP_FILTER_FLAG_NEW_LISTENER, ...)`, which returns a notification file descriptor
- Sets `FD_CLOEXEC` on the notification fd
- Sends the notification fd to the parent process via `util::sendFd(pipefd, unotif_fd)` so the parent can handle `SECCOMP_RET_USER_NOTIF` events

### Reporting seccomp Violations

When a seccomp violation (SIGSYS) is caught, the `seccompViolation()` function in `subproc.cc` reads the system call number and arguments from `/proc/PID/syscall` and records them in the log.

## Relationship with no_new_privs

- `prctl(PR_SET_NO_NEW_PRIVS, 1)` is required before installing an unprivileged seccomp filter
- Prevents privilege escalation through setuid binaries
- Inherited by all child processes through `execve()`
- Both `prepareAndCommit()` and `installUnotifyFilter()` in `sandbox.cc` call `prctl(PR_SET_NO_NEW_PRIVS, 1)` when actually installing a filter — they do not check the `disable_no_new_privs` setting, but they do return early if no filter needs to be installed
- The `disable_no_new_privs: true` / `--disable_no_new_privs` config only affects `containDropPrivs()` in `contain.cc`, which is called earlier in the child process setup sequence. When set, it skips the `PR_SET_NO_NEW_PRIVS` call in that function, but the seccomp code path will still set it later (allowing use of setuid binaries only in the window before the seccomp filter is applied)

## Example Kafel Policy

```
// Make geteuid return -1337
ERRNO(1337) { geteuid }

// Block ptrace and sched_setaffinity with EPERM
ERRNO(1) { ptrace, sched_setaffinity }

// Kill process on syslog
KILL_PROCESS { syslog }

// Allow everything else
DEFAULT ALLOW
```

For details on the Kafel language, see [07-kafel.md](07-kafel.md).
