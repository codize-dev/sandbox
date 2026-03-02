# internal/sandbox

Core execution logic split across three files.

## Public API (sandbox.go)

- `Runner` struct (created via `NewRunner(cfg Config)`)
- `Config`, `Status`, `Result`, and `RunOutput` types
- `Runner.Run()` returns `RunOutput` (compile + run results); orchestrates pipe creation, process execution, and result collection

## Runtime (runtime.go)

- `Runtime` interface (`Command`, `BindMounts`, `Env`, `PrepareDir`, `Rlimits` methods)
- `CompiledRuntime` optional interface (`CompileCommand`, `CompileBindMounts`, `CompileEnv`, `CompileRlimits` methods; checked via type assertion)
- `BindMount` struct and `LookupRuntime` function
- `Rlimits` struct and `Rlimits()` / `CompileRlimits()` methods for per-runtime nsjail resource limits
- Concrete implementations: `nodeRuntime`, `rubyRuntime`, `goRuntime` (unexported)

## Execution (execution.go)

- Private `execution` struct handles:
  - nsjail CLI argument assembly
  - Pipe management (stdout, stderr, log fd 3)
  - Output draining via `unix.Poll` for deterministic pipe ordering
  - Output limit enforcement
  - Timeout/signal detection from the nsjail log pipe
- Returns base64-encoded output

## nsjail Isolation

The sandbox uses nsjail (`/bin/nsjail`) with these key properties:
- `-Mo` (once mode): runs the process once and exits
- `-D /code`: sets the initial working directory inside the jail
- Network isolation via new network namespace
- `--log_fd 3`: nsjail logs piped to fd 3 for timeout detection
- `--time_limit`: configurable via `--timeout` CLI flag (default 30s); Go-level exec timeout is nsjail limit + 10s for interpreted runtimes, or 2 × nsjail limit + 10s for compiled runtimes (compile + run)
- Read-only bind mounts for system libraries (`/lib`, `/usr`, and `/lib64` if it exists), the selected runtime, `/dev/null`, `/dev/urandom`, and `/proc` (via `-m`)
- Read-write bind mount for the user code directory (`/code`) and a separate temp directory mounted as `/tmp`
- Resource limits (`--rlimit_as`, `--rlimit_fsize`, `--rlimit_nofile`) are configured per runtime via `Rlimits()` (run step) and `CompileRlimits()` (compile step). Tuned per runtime: Node.js run (`--rlimit_as 4096`, `--rlimit_fsize 64`, `--rlimit_nofile 64`), Ruby run (`--rlimit_as 1024`, `--rlimit_fsize 64`, `--rlimit_nofile 64`), Go compile (`--rlimit_as 4096`, `--rlimit_fsize 64`, `--rlimit_nofile 256`), Go run (`--rlimit_as 1024`, `--rlimit_fsize 64`, `--rlimit_nofile 64`).
- Environment: runtime-specific variables (typically `PATH` set to runtime bin dir), plus `HOME=/tmp` always appended
- Symlink mount for `/dev/fd` via `/proc/self/fd` (`-s /proc/self/fd:/dev/fd`)
- Combined output limit enforced by Go: configurable via `--output-limit` CLI flag (default 1 MiB). When exceeded, the jailed process is killed and status is set to `OUTPUT_LIMIT_EXCEEDED`.

## Hardcoded Paths

- nsjail binary: `/bin/nsjail`
- Node.js runtime: `/mise/installs/node/24.14.0/bin/node`
- Ruby runtime: `/mise/installs/ruby/3.4.8/bin/ruby`
- Go runtime: `/mise/installs/go/1.26.0/bin/go`
- Go build cache: `/mise/go-cache` (pre-built in Docker image, mounted read-only in nsjail)
