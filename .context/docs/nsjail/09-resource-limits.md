# Resource Limits

nsjail restricts resources via rlimits, time limits, CPU affinity, and process priority.

## rlimit (Resource Limits)

### RLimit Enum

Each rlimit field has a corresponding `_type` field that specifies how the value is interpreted:

| Value | Description |
|-------|-------------|
| `VALUE` (0) | Use the specified numeric value as-is |
| `SOFT` (1) | Use the system soft limit |
| `HARD` (2) | Use the system hard limit |
| `INF` (3) | Unlimited (`RLIM64_INFINITY`) |

The following special strings can be used from the CLI:

| String | Corresponding RLimit |
|--------|----------------------|
| `inf` | `INF` |
| `soft` / `def` | `SOFT` |
| `hard` / `max` | `HARD` |
| (numeric) | `VALUE` |

Note: In the current implementation, when an rlimit is specified via CLI, `*_type` is internally set to `VALUE`. For `--rlimit_as`, `--rlimit_core`, `--rlimit_fsize`, `--rlimit_stack`, and `--rlimit_memlock`, a unit conversion (MiB/KiB) is applied in a later step, so specifying `soft`/`hard` from the CLI will reliably produce incorrect values (because `adjustRLimit` applies an additional multiplication of MiB (×1,048,576) or KiB (×1,024) to the byte value returned by `parseRLimit`). To strictly use `SOFT`/`HARD`/`INF`, set `*_type` explicitly in the configuration file.

### rlimit List

| Field | Default value | Default type | Unit | CLI | POSIX constant | Description |
|-------|---------------|--------------|------|-----|----------------|-------------|
| `rlimit_as` | 4096 | `VALUE` | MiB | `--rlimit_as` | `RLIMIT_AS` | Virtual address space |
| `rlimit_core` | 0 | `VALUE` | MiB | `--rlimit_core` | `RLIMIT_CORE` | Core dump size |
| `rlimit_cpu` | 600 | `VALUE` | seconds | `--rlimit_cpu` | `RLIMIT_CPU` | CPU time |
| `rlimit_fsize` | 1 | `VALUE` | MiB | `--rlimit_fsize` | `RLIMIT_FSIZE` | Maximum file size |
| `rlimit_nofile` | 32 | `VALUE` | count | `--rlimit_nofile` | `RLIMIT_NOFILE` | Number of open file descriptors |
| `rlimit_nproc` | 1024 | `SOFT` | count | `--rlimit_nproc` | `RLIMIT_NPROC` | Number of processes |
| `rlimit_stack` | 8 | `SOFT` | MiB | `--rlimit_stack` | `RLIMIT_STACK` | Stack size |
| `rlimit_memlock` | 64 | `SOFT` | KiB | `--rlimit_memlock` | `RLIMIT_MEMLOCK` | Locked memory |
| `rlimit_rtprio` | 0 | `SOFT` | — | `--rlimit_rtprio` | `RLIMIT_RTPRIO` | Real-time priority |
| `rlimit_msgqueue` | 1024 | `SOFT` | bytes | `--rlimit_msgqueue` | `RLIMIT_MSGQUEUE` | POSIX message queue size |

Note: For fields whose default type is `SOFT` (`rlimit_nproc`, `rlimit_stack`, `rlimit_memlock`, `rlimit_rtprio`, `rlimit_msgqueue`), the default values shown in the table are only used if the type is changed to `VALUE`. When left as `SOFT`, the system soft limit is applied instead.

### Why rlimit_nproc Defaults to SOFT

Comment in the source code: "RLIMIT_NPROC is system-wide - tricky to use; use the soft limit value by default here"

Because `RLIMIT_NPROC` is applied system-wide on a per-user basis, setting a fixed value can cause interference between sandboxes. Therefore the system soft limit is used by default.

### Internal Implementation

All rlimits are set using the `prlimit64` (`__NR_prlimit64`) system call, setting both the soft and hard limits to the same value.

### Disabling rlimits

- `--disable_rlimits` / `disable_rl: true`: Disables all rlimits (inherited from the parent process)

## Time Limits

| Field | Default | CLI | Description |
|-------|---------|-----|-------------|
| `time_limit` | `600` | `-t VALUE` / `--time_limit VALUE` | Wall-clock time limit (seconds). 0 = unlimited. |

### Difference Between time_limit and rlimit_cpu

- `time_limit`: **Wall-clock time** limit. Monitored and enforced by the supervisor process.
- `rlimit_cpu`: **CPU time** limit. Enforced by the kernel via SIGXCPU.

### Time Limit Enforcement Mechanism

1. `setTimer()` sets an `ITIMER_REAL` timer at 1-second intervals, periodically firing `SIGALRM`
2. The parent process's main loop (`pause()` in `standaloneMode()`, `poll()` in `listenMode()`) wakes on signals including `SIGALRM` and calls `reapProc()`
3. Inside `reapProc()`, the elapsed time of running processes is checked
4. If `time_limit` is exceeded:
   - First sends `SIGCONT`
   - Then sends `SIGKILL`
5. The reason `SIGCONT` is sent first: stopped or namespaced processes may not respond to `SIGKILL` (noted in the source code as a possible kernel bug)

### Behavior When time_limit = 0

When `time_limit` is `0`, no time limit is applied. This is explicitly checked inside the time limit enforcement loop.

### time_limit Limitation in EXECVE Mode

In EXECVE mode (`-Me`), `setTimer()` returns early, so the `ITIMER_REAL` timer is not set and wall-clock time limiting via `time_limit` does not function. Since there is no supervisor process in EXECVE mode, the entire time limit enforcement mechanism does not operate. Use `rlimit_cpu` (CPU time limit, enforced by the kernel via SIGXCPU) to limit execution time.

## CPU Affinity

| Field | Default | CLI | Description |
|-------|---------|-----|-------------|
| `max_cpus` | `0` (no limit) | `--max_cpus VALUE` | Maximum number of CPUs that may be used |

### Behavior

When `max_cpus > 0`:

1. Obtain the current permitted CPU set with `sched_getaffinity`
2. Randomly select `max_cpus` CPUs from the available CPU set
3. Set the affinity with `sched_setaffinity`

Random selection uses the MMIX LCG pseudo-random number generator, seeded from `getrandom()` or `/dev/urandom`.

This processing is performed inside the child process (after capability dropping, but while sufficient privileges remain).

## Process Priority

| Field | Default | CLI | Description |
|-------|---------|-----|-------------|
| `nice_level` | `19` | `--nice_level VALUE` | Process nice value (-20: highest priority, 19: lowest priority) |

Set via `setpriority(PRIO_PROCESS, 0, nice_level)`. The default value of `19` is the lowest priority, preventing sandbox processes from affecting the responsiveness of the host system.
