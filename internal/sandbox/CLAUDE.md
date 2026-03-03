# internal/sandbox

Core sandbox execution engine, split across three files:

- **sandbox.go** — `Runner` orchestrates the full execution lifecycle: applies default files, runs compilation (if needed) and execution steps, and collects results.
- **runtime.go** — `Runtime` interface and concrete implementations for each supported language. `CompiledRuntime` extends it for languages requiring a build step. Defines `Rlimits` (per-process resource limits via nsjail `--rlimit_*`), `Cgroups` (cgroup-based limits via `--cgroup_pids_max`, `--cgroup_mem_max`, `--cgroup_mem_swap_max`, `--cgroup_cpu_ms_per_sec`), and `Limits` (combines both).
- **execution.go** — Handles a single nsjail invocation: loads the static nsjail config file, assembles per-invocation CLI arguments, manages pipes (stdout, stderr, nsjail log), drains output via `poll(2)`, and enforces limits.
- **configs/nsjail.cfg** — Static protobuf-format nsjail configuration loaded via `-C /etc/nsjail/nsjail.cfg`. Defines execution mode (`ONCE`), logging (`log_fd: 3`), working directory, static rlimits, and filesystem mounts (system libraries, device nodes, tmpfs, procfs, symlinks). Per-invocation settings (runtime bind mounts, resource limits, timeout, env vars) are passed as CLI flags.
- **defaults/go/** — Embedded `go.mod.tmpl` and `go.sum.tmpl` templates applied as default files for Go runtime execution.

Go runtime rejects user-submitted `go.mod` and `go.sum` files (HTTP 400) to enforce use of these defaults.
