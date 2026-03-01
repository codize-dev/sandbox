# internal/sandbox

Core execution logic split across two files.

## Public API (sandbox.go)

- `Runner` struct (created via `NewRunner(cfg Config)`)
- `Config`, `Runtime`, `Status`, and `Result` types
- Runtime configuration registry (`runtimes` map)
- `Runner.Run()` orchestrates pipe creation, process execution, and result collection

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
- `--time_limit`: configurable via `--timeout` CLI flag (default 30s); Go-level exec timeout is nsjail limit + 10s
- Read-only bind mounts for system libraries (`/lib`, `/usr`, and `/lib64` if it exists), the selected runtime, `/dev/null`, `/dev/urandom`, and `/proc` (via `-m`)
- Read-write bind mount for the user code directory (`/code`) and a separate temp directory mounted as `/tmp`
- Address space limited to system hard limit (`--rlimit_as hard`)
- Environment: `PATH` set to runtime bin dir, `HOME=/tmp`
- Symlink mount for `/dev/fd` via `/proc/self/fd` (`-s /proc/self/fd:/dev/fd`)
- Combined output limit enforced by Go: configurable via `--output-limit` CLI flag (default 1 MiB). When exceeded, the jailed process is killed and status is set to `OUTPUT_LIMIT_EXCEEDED`.

## Hardcoded Paths

- nsjail binary: `/bin/nsjail`
- Node.js runtime: `/mise/installs/node/24.14.0/bin/node`
- Ruby runtime: `/mise/installs/ruby/3.4.8/bin/ruby`
