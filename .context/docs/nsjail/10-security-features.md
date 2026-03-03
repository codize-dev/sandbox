# Security Features

nsjail provides numerous security features including capability management, no_new_privs, TSC disabling, personality flags, file descriptor control, and signal control.

## Capability Management

### Configuration Fields

| Field | Default | CLI | Description |
|-------|---------|-----|-------------|
| `keep_caps` | `false` | `--keep_caps` | Retain all capabilities |
| `cap` | — | `--cap CAP_NAME` | Retain specific capabilities (can be specified multiple times) |

### Default Behavior (`keep_caps: false`, no `cap` specified)

All capabilities are dropped:

1. Clear the inheritable set
2. Clear the ambient set with `PR_CAP_AMBIENT_CLEAR_ALL`
3. If `CAP_SETPCAP` is in the effective set: drop all capabilities from the bounding set

### When Retaining Specific Capabilities (`cap` specified)

1. Clear the inheritable set
2. Clear the ambient set
3. Set each capability specified in `cap` in the inheritable set
4. `setCaps()`: set the effective and permitted sets
5. If `CAP_SETPCAP` is effective: drop capabilities not in the retain list from the bounding set
6. Raise each capability specified in `cap` in the ambient set

### When Retaining All Capabilities (`keep_caps: true`)

1. Copy all permitted capabilities into the inheritable set
2. Commit the inheritable set to the kernel via `setCaps()`
3. Raise all permitted capabilities in the ambient set (retained across `execve`)

### Supported Capabilities (41 total)

| Capability | Description |
|------------|-------------|
| `CAP_CHOWN` | Change file UID/GID |
| `CAP_DAC_OVERRIDE` | Bypass file read/write/execute access control |
| `CAP_DAC_READ_SEARCH` | Bypass file read and directory search permissions |
| `CAP_FOWNER` | Bypass file owner checks |
| `CAP_FSETID` | Allow modification of set-user-ID/set-group-ID bits |
| `CAP_KILL` | Bypass permission checks for sending signals |
| `CAP_SETGID` | Allow manipulation of GIDs |
| `CAP_SETUID` | Allow manipulation of UIDs |
| `CAP_SETPCAP` | Allow capability manipulation |
| `CAP_LINUX_IMMUTABLE` | Allow setting the immutable flag |
| `CAP_NET_BIND_SERVICE` | Allow binding to ports below 1024 |
| `CAP_NET_BROADCAST` | Allow network broadcasting |
| `CAP_NET_ADMIN` | Allow network configuration |
| `CAP_NET_RAW` | Allow use of raw sockets |
| `CAP_IPC_LOCK` | Allow memory locking |
| `CAP_IPC_OWNER` | Bypass System V IPC permission checks |
| `CAP_SYS_MODULE` | Allow loading/unloading kernel modules |
| `CAP_SYS_RAWIO` | Allow raw I/O access |
| `CAP_SYS_CHROOT` | Allow chroot |
| `CAP_SYS_PTRACE` | Allow ptrace |
| `CAP_SYS_PACCT` | Allow process accounting |
| `CAP_SYS_ADMIN` | Allow numerous system administration operations |
| `CAP_SYS_BOOT` | Allow reboot |
| `CAP_SYS_NICE` | Allow changing nice values and scheduling priorities |
| `CAP_SYS_RESOURCE` | Allow overriding resource limits |
| `CAP_SYS_TIME` | Allow changing the system clock |
| `CAP_SYS_TTY_CONFIG` | Allow configuring TTY devices |
| `CAP_MKNOD` | Allow mknod |
| `CAP_LEASE` | Allow setting file leases |
| `CAP_AUDIT_WRITE` | Allow writing to the audit log |
| `CAP_AUDIT_CONTROL` | Allow configuring the audit system |
| `CAP_SETFCAP` | Allow setting file capabilities |
| `CAP_MAC_OVERRIDE` | Override MAC (Mandatory Access Control) |
| `CAP_MAC_ADMIN` | Allow MAC administration |
| `CAP_SYSLOG` | Allow syslog operations |
| `CAP_WAKE_ALARM` | Allow wake alarms |
| `CAP_BLOCK_SUSPEND` | Allow blocking system suspend |
| `CAP_AUDIT_READ` | Allow reading the audit log |
| `CAP_PERFMON` | Allow performance monitoring |
| `CAP_BPF` | Allow BPF operations |
| `CAP_CHECKPOINT_RESTORE` | Allow checkpoint/restore |

`CAP_PERFMON`, `CAP_BPF`, and `CAP_CHECKPOINT_RESTORE` have fallback definitions for older kernel headers.

## no_new_privs

| Field | Default | CLI | Description |
|-------|---------|-----|-------------|
| `disable_no_new_privs` | `false` | `--disable_no_new_privs` | Do not set `PR_SET_NO_NEW_PRIVS` |

### Default Behavior

`prctl(PR_SET_NO_NEW_PRIVS, 1)` is set in two places:

1. `contain::containDropPrivs()`: set unconditionally during the privilege-dropping step of the containment sequence (unless `disable_no_new_privs` is `true`)
2. `sandbox::applyPolicy()`: set again immediately before applying the policy, but only when a seccomp policy is configured

Effects:

- Prevents privilege escalation via setuid binaries
- Allows installation of unprivileged seccomp filters
- Inherited by all child processes across `execve()`

### Effects When Disabled

When `disable_no_new_privs: true`:

- The setting in `containDropPrivs()` is skipped
- However, if a seccomp policy is configured, it is still set inside `sandbox::applyPolicy()` (required for applying seccomp filters)
- Without seccomp: use of setuid binaries is permitted

## TSC Disabling

| Field | Default | CLI | Description |
|-------|---------|-----|-------------|
| `disable_tsc` | `false` | `--disable_tsc` | Disable `rdtsc`/`rdtscp` instructions |

### Behavior

Sets `prctl(PR_SET_TSC, PR_TSC_SIGSEGV)`, causing `SIGSEGV` to be raised when `rdtsc`/`rdtscp` instructions are executed.

- Effective **only on x86/x86-64 architectures**
- Source code warning: "WARNING: To make it effective, you also need to forbid `prctl(PR_SET_TSC, PR_TSC_ENABLE, ...)` in seccomp rules! Dynamic binaries produced by GCC seem to rely on RDTSC, but static ones should work."
- For full effect, `prctl(PR_SET_TSC, PR_TSC_ENABLE)` must also be forbidden in seccomp rules

## Linux personality Flags

| Field | CLI | Kernel Constant | Description |
|-------|-----|-----------------|-------------|
| `persona_addr_compat_layout` | `--persona_addr_compat_layout` | `ADDR_COMPAT_LAYOUT` | Legacy virtual address layout |
| `persona_mmap_page_zero` | `--persona_mmap_page_zero` | `MMAP_PAGE_ZERO` | Map page zero |
| `persona_read_implies_exec` | `--persona_read_implies_exec` | `READ_IMPLIES_EXEC` | Implicitly grant EXEC along with READ for mmap |
| `persona_addr_limit_3gb` | `--persona_addr_limit_3gb` | `ADDR_LIMIT_3GB` | Limit address space to 3 GB |
| `persona_addr_no_randomize` | `--persona_addr_no_randomize` | `ADDR_NO_RANDOMIZE` | Disable ASLR |

All default to `false`. Set via the `personality()` system call.

## File Descriptor Control

| Field | Default | CLI | Description |
|-------|---------|-----|-------------|
| `silent` | `false` | `--silent` | Redirect fd 0/1/2 to `/dev/null` |
| `stderr_to_null` | `false` | `--stderr_to_null` | Redirect only fd 2 to `/dev/null` |
| `pass_fd` | — | `--pass_fd N` | Additional fds to pass into the jail (default: only 0, 1, 2) |

### File Descriptor close-on-exec Handling

`containMakeFdsCOE()`: sets the close-on-exec flag on all file descriptors not included in `openfds`. The following three methods are tried in order:

1. `close_range(CLOSE_RANGE_CLOEXEC)` (recent kernels)
2. Read `/proc/self/fd` and process each fd
3. Naive loop from fd 0 to 1024

## Signal Control

| Field | Default | CLI | Description |
|-------|---------|-----|-------------|
| `forward_signals` | `false` | `--forward_signals` | Forward fatal signals to child processes (instead of SIGKILL) |
| `skip_setsid` | `false` | `--skip_setsid` | Do not call `setsid()` |

### forward_signals

Controls the behavior of `killAndReapAll()` when nsjail itself receives a fatal signal (e.g., SIGTERM):

- `false` (default): send `SIGKILL` to child processes
- `true`: forward the received signal as-is to child processes

Note: When a process is terminated due to exceeding the time limit (`time_limit`), `SIGKILL` is always sent regardless of the `forward_signals` value. The effect of `forward_signals` applies only to `killAndReapAll()` when nsjail itself receives a signal.

### skip_setsid

`setsid()` is called by default to place the jail process in a new session. Setting `skip_setsid: true` skips this, but is considered dangerous as it allows terminal injection (injecting characters into the controlling terminal). It is useful when job control or signal control via `/bin/sh` is needed, but carries security risks.

## Death Signal via prctl

`prctl(PR_SET_PDEATHSIG, SIGKILL)` is set inside the child process. This causes the child process to be automatically killed if the parent process dies.
