# Process Lifecycle

Describes the complete sequence of internal process creation, containment, and termination in nsjail.

## Startup Initialization

The following initialization steps are performed before the main loop begins:

1. Daemonize (`daemon(1, 0)`) — only when the `daemon` flag is enabled
2. Log configuration parameters (`cmdline::logParams()`)
3. Set up signal handlers (`setSigHandlers()`)
4. Set file descriptor limit (`setFDLimit()`)
5. Set up a 1-second interval timer (`setTimer()`) — returns early in EXECVE mode
6. Auto-detect cgroup v2 (when `detect_cgroupv2` is enabled)
7. Set up the cgroup v2 parent cgroup (`cgroup2::setup()`) — enable controllers in the root cgroup's `cgroup.subtree_control`, and move nsjail itself into a `NSJAIL_SELF.{pid}` child cgroup if necessary (cgroup v2 only)
8. Compile the seccomp policy (`sandbox::preparePolicy()`)

## Non-EXECVE Mode (ONCE / RERUN / LISTEN) Lifecycle

### 1. Process Creation (`subproc.cc`)

1. `net::limitConns()` — check connection count limit (LISTEN mode)
2. Build clone flags from configured namespaces
3. Create a parent-child synchronization channel with `socketpair(AF_UNIX, SOCK_STREAM | SOCK_CLOEXEC)`
4. `cloneProc()` — three-stage fallback:
   1. Attempt `clone3()` + `CLONE_CLEAR_SIGHAND` (Linux 5.5 and above)
   2. If step 1 fails (regardless of error type): retry `clone3()` without `CLONE_CLEAR_SIGHAND`
   3. If step 2 fails with `ENOSYS` (clone3 not supported): fall back to legacy `clone()` using a `setjmp`/`longjmp` trick with the midpoint of a 128 KiB static stack buffer (`&cloneStack[sizeof(cloneStack) / 2]`) (midpoint used to avoid dependence on stack growth direction)

### 2. Child Process Side (`newProc()`)

1. Write to `/proc/self/oom_score_adj` — adjust OOM killer score (when configured)
2. `contain::setupFD()` — dup2 for stdin/stdout/stderr
3. `resetEnv()` — reset signals in the `nssigs` array (SIGINT, SIGQUIT, SIGUSR1, SIGALRM, SIGCHLD, SIGTERM, SIGTTIN, SIGTTOU, SIGPIPE) to `SIG_DFL` and unblock all signals
4. `net::initChildPreSync()` — initialize nstun child-side (when user networking is configured)
5. Wait to read sync signal (`'D'`) from socketpair (non-EXECVE mode only)
6. `contain::containProc()` — set up all namespaces and resources (described below)
7. Clear environment variables (when `keep_env` is false)
8. Set configured environment variables
9. `sandbox::applyPolicy()` — apply seccomp-bpf
10. Send seccomp unotify notification FD to parent via the synchronization channel (when seccomp unotify is configured)
11. Wait to read a second sync signal (`'D'`) from the parent (when seccomp unotify is configured)
12. `execv()` or `execveat()`

### 3. Parent Process Side (`initParent()` + `runChild()`)

1. `net::initParent()` — move/clone interfaces, start pasta, initialize nstun parent-side
2. `cgroup::initNsFromParent()` or `cgroup2::initNsFromParent()` — create and configure cgroup
3. `user::initNsFromParent()` — set `setgroups` to `"deny"`, write to `/proc/PID/gid_map`, then write to `/proc/PID/uid_map`
4. Send sync character `'D'` to unblock the child process
5. Receive seccomp unotify notification FD from the child (when seccomp unotify is configured)
6. After `initParent()` returns, wait inside `runChild()` for an error response from the child: if the child fails to start, it sends the error character `'E'`

### 4. Child Process Execution

After receiving the sync signal, execution proceeds in order: `containProc()` → apply seccomp → `execv()`.

## EXECVE Mode Lifecycle

1. `unshare()` is called instead of `clone()` to isolate namespaces within the current process
2. `newProc()` runs directly in the current process:
   - There is no parent-child synchronization channel, so the sync wait is skipped
   - user/cgroup initialization is performed inline inside `newProc()` (in non-EXECVE mode, the parent process handles this)
3. If `CLONE_NEWPID` is enabled, a dummy init process is spawned via `subproc::cloneProc(CLONE_FS, 0)` (note: not `fork()`)
4. The `ITIMER_REAL` timer is not set (in EXECVE mode, `setTimer()` returns early). Time limit enforcement relies on `rlimit_cpu` (CPU time)

## Containment Sequence (`contain.cc: containProc()`)

Executed sequentially inside the child process:

### Step 1: User Namespace Initialization

`containUserNs()` → `user::initNs()` → `user::initNsFromChild()`:

1. `prctl(PR_SET_SECUREBITS, SECBIT_KEEP_CAPS | SECBIT_NO_SETUID_FIXUP)` — retain capabilities across uid/gid changes
2. `setresgid()` — set primary GID
3. `setgroups()` — set supplementary GIDs
4. `setresuid()` — set primary UID
5. `prctl(PR_SET_SECUREBITS, 0)` — reset securebits

### Step 2: PID Namespace Initialization

`containInitPidNs()` → `pid::initNs()`:

- Only when in EXECVE mode and `clone_newpid` is enabled:
  1. Spawn a dummy init process via `subproc::cloneProc(CLONE_FS, 0)`
  2. In the child process (init), configure the following:
     - `prctl(PR_SET_PDEATHSIG, SIGKILL)` — auto-terminate when parent dies
     - `prctl(PR_SET_NAME, "ns-init")` — set process name
     - `prctl(PR_SET_DUMPABLE, 0)` — disable core dumps
     - `sigaction(SIGCHLD, SA_NOCLDWAIT | SA_NOCLDSTOP)` — auto-reap zombie processes
     - Wait in an infinite `pause()` loop

### Step 3: Mount Namespace Initialization

`containInitMountNs()` → `mnt::initNs()`:

1. Privatize the root
2. Mount tmpfs on a temporary working directory
3. Process all mount entries
4. `pivot_root()` (or `mount(MS_MOVE)` + `chroot()`)
5. Remount read-only
6. `chdir(cwd)`

See [04-filesystem.md](04-filesystem.md) for details.

### Step 4: Network Namespace Initialization

`containInitNetNs()` → `net::initNs()` → `net::initNsFromChild()`:

1. Bring up `lo` (loopback) (when `iface_no_lo` is false)
2. Configure MACVLAN IP/mask/gateway (when `macvlan_iface` is set)
3. Apply traffic rules (when traffic rules are configured)

### Step 5: UTS Namespace Initialization

`containInitUtsNs()` → `uts::initNs()`:

- `sethostname(hostname)`

### Step 6: Cgroup Namespace Initialization

`containInitCgroupNs()` → `cgroup::initNs()`:

- no-op (cgroup setup is performed from the parent process)

### Step 7: Privilege Dropping

`containDropPrivs()`:

1. `prctl(PR_SET_NO_NEW_PRIVS, 1)` (unless disabled)
2. `caps::initNs()` — drop/retain capabilities

### Step 8: CPU Affinity Configuration

`containCPU()` → `cpu::initCpu()`:

- If `max_cpus > 0`, apply `sched_setaffinity` to up to `max_cpus` randomly selected CPUs

### Step 9: TSC Disabling

`containTSC()`:

- If `disable_tsc` is true, apply `prctl(PR_SET_TSC, PR_TSC_SIGSEGV)` (x86 only)

### Step 10: Resource Limit Configuration

`containSetLimits()`:

- Set 10 rlimits via `prlimit64` (skipped when `disable_rl` is true)

### Step 11: Environment Preparation

`containPrepareEnv()`:

1. `prctl(PR_SET_PDEATHSIG, SIGKILL)` — kill when parent dies
2. Apply `personality()` flags
3. `setpriority(PRIO_PROCESS, 0, nice_level)` — set nice value
4. `setsid()` (when `skip_setsid` is false)

### Step 12: File Descriptor close-on-exec

`containMakeFdsCOE()`:

- Set close-on-exec on all fds not in `openfds`
- Three methods tried in order:
  1. `close_range(CLOSE_RANGE_CLOEXEC)` — first marks ALL fds close-on-exec, then re-enables fds in `openfds` by clearing `FD_CLOEXEC`
  2. Read `/proc/self/fd`
  3. Naive loop over fd 0-1023

## Process Reaping and Time Limits

### Event Loop

The parent process's `reapProc()` function monitors process state:

1. A 1-second `ITIMER_REAL` timer fires `SIGALRM`
2. On each tick, check elapsed time for running processes
3. When `time_limit` is exceeded:
   - Send `SIGCONT` (to handle stopped processes)
   - Send `SIGKILL`

### Displaying Process List via SIGUSR1 / SIGQUIT

Sending `SIGUSR1` or `SIGQUIT` to the nsjail process invokes `displayProc()`, which prints a list of all running sandbox processes (PID, remote host, elapsed time, remaining time).

### Reason for SIGCONT → SIGKILL Order

A comment in the source code explains that stopped or namespaced processes may not respond to `SIGKILL` (possibly a kernel bug), so `SIGCONT` is sent first.

### Child Process Termination Handling

`reapProc()` performs the following cleanup:

1. Detect process termination with `waitid(P_ALL, 0, &si, WNOHANG | WNOWAIT | WEXITED)`, then reap via `wait4()` inside the internal `reapProc(nsj, pid)`
2. Remove cgroup directory (`cgroup::finishFromParent()` / `cgroup2::finishFromParent()`)
3. Call `removeProc()`:
   - Terminate the pasta process if present (send `SIGKILL`, then `waitpid()` to reap it)
   - Close `pid_syscall_fd`
   - Remove the entry from the process map

### Seccomp Violation Detection

When a process is added (`addProc()`), a fd to `/proc/PID/syscall` is opened in advance and cached in `pid_syscall_fd`. When a process exits with `SIGSYS` (seccomp violation), `seccompViolation()` reads the system call number, arguments, SP, and PC from the cached fd and outputs a detailed log.

### Abnormal pasta Termination

If the pasta process exits unexpectedly, `SIGKILL` is sent to the corresponding jail process.

### Forward Signals

When `forward_signals` is true, fatal signals received by nsjail (e.g., `SIGTERM`, `SIGINT`) are forwarded to all running sandbox child processes before nsjail itself exits.

### Shutdown Cleanup

During shutdown, the following cleanup steps are performed:

1. `killAndReapAll()` — send `SIGKILL` to all running sandbox child processes and reap them via `waitpid()`
2. `sandbox::closePolicy()` — free the compiled seccomp policy
3. `unotify::stop()` — stop the seccomp unotify listener thread if active

## LISTEN Mode-Specific Behavior

### Pipe Relay (`nsjail.cc: pipeTraffic()`)

Data transfer between the TCP socket and the sandbox process's stdin/stdout:

1. Monitor fds with `poll()` — 3 fds per connection (connfd, pipe in, pipe out) plus the listen fd
2. Transfer data with zero-copy using `splice()`
3. Teardown on `POLLERR`/`POLLHUP` or when `splice()` returns 0

### Handling of stderr

In LISTEN mode, the child process's stdout and stderr are both connected to the same pipe (`out[1]`). This means stderr output is merged into stdout and sent to the TCP client.

### Behavior on Connection Disconnect

When a TCP connection is disconnected (when `POLLERR` / `POLLHUP` is detected or `splice()` returns 0):

1. All fds for the socket and pipe are closed
2. `SIGKILL` is sent to the corresponding child process
3. The pipe entry is cleaned up

## Logging (`logs.cc`)

### Log Levels

| Level | Value | CLI |
|-------|-------|-----|
| `DEBUG` | 0 | `-v` / `--verbose` |
| `INFO` | 1 | (default) |
| `WARNING` | 2 | `-q` / `--quiet` |
| `ERROR` | 3 | — |
| `FATAL` | 4 | `-Q` / `--really_quiet` |
| `HELP` | 5 | (for usage output, internal) |
| `HELP_BOLD` | 6 | (for bold usage output, internal) |

### Log Destinations

| Field | CLI | Description |
|-------|-----|-------------|
| `log_file` | `-l FILE` | Log file path |
| `log_fd` | `-L FD` | File descriptor for logging |

- Default: log output to `dup(STDERR_FILENO)` (to survive fd reassignment)
- When `--daemon` is used and no log file is specified: `/var/log/nsjail.log` is created automatically
- `FATAL` level logs call `_exit(0xff)`
- TTY color codes are suppressed when the `NO_COLOR` environment variable is set

### Log Format

DEBUG/WARNING/ERROR/FATAL:
```
[D][2024-01-01T12:00:00+0000][12345] functionName():123 message
```
or, when TID differs from PID:
```
[D][2024-01-01T12:00:00+0000][12345/67890] functionName():123 message
```

INFO:
```
[I][2024-01-01T12:00:00+0000] message
```

`[D]`/`[I]`/`[W]`/`[E]`/`[F]` indicate the log level. `[PID]` or `[PID/TID]` (TID is only included when it differs from PID) shows the process and thread IDs. Only the INFO level omits PID/TID, function name, and line number (`print_funcline = false`).

## TTY State Save and Restore

At startup, the terminal settings (`struct termios`) are saved via `getTC(STDIN_FILENO)`, and restored at exit via `setTC()` (unless in `daemon` mode). Even if a sandbox process modifies the terminal settings, nsjail restores the original state on exit.
