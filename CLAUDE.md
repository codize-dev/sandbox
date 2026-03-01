# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A code execution sandbox service that runs arbitrary code inside Linux namespace jails (nsjail). Supports Node.js and Ruby runtimes. Exposes an HTTP API to receive code, execute it in an isolated environment, and return the output.

## Development Setup

Tool versions are managed by [mise](https://mise.jdx.dev/). Run `mise install` to install Go and golangci-lint.

- **Go**: 1.26.0 (mise.toml), module requires 1.25.0 (go.mod)
- **golangci-lint**: 2.10.1 (installed via mise aqua backend)

## Build & Run Commands

```bash
# Build the server binary
go build -o sandbox .

# Run locally (requires nsjail + Node.js + Ruby at hardcoded paths; use Docker instead)
docker compose up --build

# Restart with rebuild (for E2E testing after code changes)
docker compose down && docker compose up --build -d

# Lint
golangci-lint run

# Run unit tests
go test ./...

# Run E2E tests (requires running server via docker compose)
go test -tags e2e ./e2e/...
```

The container must run in **privileged mode** (required for nsjail to create Linux namespaces).

## Architecture

### Request Flow

```
POST /v1/run → main.go → cmd/serve.go (Cobra CLI, Echo v5 router)
             → internal/handler/handler.go (validate runtime, decode base64 files, write to tmpdir)
             → internal/sandbox/sandbox.go (invoke nsjail with the selected runtime)
             → Response: {stdout, stderr, output, exit_code, status, signal} (stdout/stderr/output are base64-encoded)
```

### Key Packages

- **cmd/** — CLI entrypoint using Cobra. `root.go` defines the root command; `serve.go` registers the `serve` subcommand that starts the Echo v5 HTTP server with request logging middleware. Single route: `POST /v1/run`. Accepts `--addr` (default `:8080`), `--timeout` (default `30`), and `--output-limit` (default 1 MiB) flags.
- **internal/handler/** — Request parsing and response formatting. Defines the `Handler` struct holding a `*sandbox.Runner`. Validates the `runtime` field and file names (rejects path traversal, slashes, `.`, `..`, empty names, null bytes), decodes base64 file contents from the request, writes them to a temp directory, and calls `Runner.Run()`. The first file in the `files` array is the entrypoint. Returns HTTP 400 on invalid input, HTTP 504 on execution timeout.
- **internal/sandbox/** — Core execution logic split across two files. `sandbox.go` defines the public API: `Runner` struct (created via `NewRunner(cfg Config)`), `Config`, `Runtime`, `Status`, and `Result` types, and the runtime configuration registry (`runtimes` map). `Runner.Run()` orchestrates pipe creation, process execution, and result collection. `execution.go` defines the private `execution` struct that handles nsjail CLI argument assembly, pipe management (stdout, stderr, log fd 3), output draining via `unix.Poll` for deterministic pipe ordering, output limit enforcement, and timeout/signal detection from the nsjail log pipe. Returns base64-encoded output.

### nsjail Isolation

The sandbox uses nsjail (`/bin/nsjail`) with these key properties:
- `-Mo` (once mode): runs the process once and exits
- Network isolation via new network namespace
- `--log_fd 3`: nsjail logs piped to fd 3 for timeout detection
- `--time_limit`: configurable via `--timeout` CLI flag (default 30s); Go-level exec timeout is nsjail limit + 10s
- Read-only bind mounts for system libraries, the selected runtime, `/dev/null`, `/dev/urandom`, and `/proc` (via `-m`)
- Read-write bind mount for the user code directory (`/code`) and a separate temp directory mounted as `/tmp`
- Address space limited to system hard limit (`--rlimit_as hard`)
- Environment: `PATH` set to runtime bin dir, `HOME=/tmp`
- Symlink mount for `/dev/fd` via `/proc/self/fd` (`-s /proc/self/fd:/dev/fd`)
- Combined output limit enforced by Go: configurable via `--output-limit` CLI flag (default 1 MiB). When exceeded, the jailed process is killed and status is set to `OUTPUT_LIMIT_EXCEEDED`.

### Hardcoded Paths (in sandbox.go and Dockerfile)

- nsjail binary: `/bin/nsjail`
- Node.js runtime: `/mise/installs/node/24.14.0/bin/node`
- Ruby runtime: `/mise/installs/ruby/3.4.8/bin/ruby`

### Docker Build

Four-stage Dockerfile:
1. **mise** stage: downloads mise binary for the target architecture
2. **base** stage: based on `ghcr.io/codize-dev/nsjail` (pinned by commit SHA), pre-installs Node.js 24.14.0 and Ruby 3.4.8 via mise
3. **builder** stage: compiles Go binary (`sandbox`) with `CGO_ENABLED=0`, `-trimpath`, `-ldflags="-w -s"`
4. **runtime** stage: extends `base`, adds the `sandbox` binary. Entrypoint: `sandbox serve`

### Sister Repository

The nsjail Docker base image is built from the `codize-dev/nsjail` repository and published to `ghcr.io/codize-dev/nsjail`.

## API

### POST /v1/run

Request (`runtime` is required, must be `"node"` or `"ruby"`):
```json
{"runtime": "node", "files": [{"name": "index.js", "content": "<base64-encoded source>"}]}
```

Response:
```json
{"run": {"stdout": "<base64>", "stderr": "<base64>", "output": "<base64>", "exit_code": 0, "status": "OK", "signal": null}}
```

Possible `status` values: `"OK"`, `"TIMEOUT"`, `"OUTPUT_LIMIT_EXCEEDED"`.
