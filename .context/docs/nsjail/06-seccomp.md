# System Call Filtering with Seccomp-BPF

nsjail uses seccomp-bpf (Secure Computing Mode with Berkeley Packet Filter) to filter system calls. Policies are written using the [Kafel](https://github.com/google/kafel) language.

## Configuration Fields

| Field | Type | Default | CLI | Description |
|-------|------|---------|-----|-------------|
| `seccomp_policy_file` | string | — | `-P FILE` | Path to a Kafel policy file |
| `seccomp_string` | repeated string | — | `--seccomp_string POLICY` | Inline Kafel policy string (multiple values accepted, but due to `kafel_set_input_string()` overwriting the input pointer in the internal implementation, **only the last specified string is actually compiled**) |
| `seccomp_log` | bool | `false` | `--seccomp_log` | Use `SECCOMP_FILTER_FLAG_LOG` (kernel 4.14 or later) |

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

### Resource Release (`sandbox.cc: closePolicy()`)

`closePolicy()` is called when nsjail exits to free the memory of the compiled BPF filter.

### Policy Application (`sandbox.cc: applyPolicy()`)

Called inside the child process after all confinement setup is complete (immediately before `execv()`). This is the **final step**.

- Without `seccomp_log`:
  ```c
  prctl(PR_SET_NO_NEW_PRIVS, 1);
  prctl(PR_SET_SECCOMP, SECCOMP_MODE_FILTER, &fprog);
  ```

- With `seccomp_log`:
  ```c
  prctl(PR_SET_NO_NEW_PRIVS, 1);
  syscall(__NR_seccomp, SECCOMP_SET_MODE_FILTER,
          SECCOMP_FILTER_FLAG_TSYNC | SECCOMP_FILTER_FLAG_LOG, &fprog);
  ```

`SECCOMP_FILTER_FLAG_TSYNC` synchronizes the filter to all threads.

### Reporting seccomp Violations

When a seccomp violation (SIGSYS) is caught, the system call number and arguments are read from `/proc/PID/syscall` and recorded in the log.

## Relationship with no_new_privs

- `prctl(PR_SET_NO_NEW_PRIVS, 1)` is set by default before the seccomp filter is applied
- This allows installation of an unprivileged seccomp filter
- Prevents privilege escalation through setuid binaries
- Inherited by all child processes through `execve()`
- Can be disabled with `disable_no_new_privs: true` / `--disable_no_new_privs` (allows use of setuid binaries, but installing a seccomp filter then requires root or `CAP_SYS_ADMIN`)

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
