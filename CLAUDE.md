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
go build -o server ./cmd/server

# Run locally (requires nsjail + Node.js + Ruby at hardcoded paths; use Docker instead)
docker compose up --build

# Lint
golangci-lint run

# Run unit tests
go test ./...

# Run E2E tests (requires running server: docker compose up --build)
go test -tags e2e ./e2e/...
```

The container must run in **privileged mode** (required for nsjail to create Linux namespaces).

## Architecture

### Request Flow

```
POST /v1/run → cmd/server/main.go (Echo v5 router)
             → internal/handler/handler.go (validate runtime, decode base64 files, write to tmpdir)
             → internal/sandbox/sandbox.go (invoke nsjail with the selected runtime)
             → Response: {stdout, stderr, output, exit_code, status} (all base64-encoded)
```

### Key Packages

- **cmd/server/** — HTTP server entrypoint. Echo v5 with request logging middleware. Single route: `POST /v1/run`.
- **internal/handler/** — Request parsing and response formatting. Validates the `runtime` field, decodes base64 file contents from the request, writes them to a temp directory, and calls `sandbox.Run()`. The first file in the `files` array is the entrypoint. Returns HTTP 504 on execution timeout.
- **internal/sandbox/** — Core execution logic. Defines the `Runtime` type and a runtime configuration registry (`runtimes` map). Assembles nsjail CLI arguments for the selected runtime and runs the jailed process. Captures stdout, stderr, and combined output using `unix.Poll` for deterministic pipe ordering. Detects nsjail timeout via log pipe. Returns base64-encoded output.

### nsjail Isolation

The sandbox uses nsjail (`/bin/nsjail`) with these key properties:
- `-Mo` (once mode): runs the process once and exits
- Network isolation via new network namespace
- `--log_fd 3`: nsjail logs piped to fd 3 for timeout detection
- `--time_limit`: configurable via `SANDBOX_RUN_TIMEOUT` env var (default 30s); Go-level exec timeout is nsjail limit + 10s
- Read-only bind mounts for system libraries, the selected runtime, `/dev/null`, `/dev/urandom`, and `/proc` (via `-m`)
- Read-write bind mount for the user code directory (`/code`) and a separate temp directory mounted as `/tmp`
- Address space limited to system hard limit (`--rlimit_as hard`)
- Environment: `PATH` set to runtime bin dir, `HOME=/tmp`

### Hardcoded Paths (in sandbox.go and Dockerfile)

- nsjail binary: `/bin/nsjail`
- Node.js runtime: `/mise/installs/node/24.14.0/bin/node`
- Ruby runtime: `/mise/installs/ruby/3.4.8/bin/ruby`

### Docker Build

Four-stage Dockerfile:
1. **mise** stage: downloads mise binary for the target architecture
2. **base** stage: based on `ghcr.io/codize-dev/nsjail` (pinned by commit SHA), pre-installs Node.js 24.14.0 and Ruby 3.4.8 via mise
3. **builder** stage: compiles Go binary with `CGO_ENABLED=0`
4. **runtime** stage: extends `base`, adds the server binary

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
{"run": {"stdout": "<base64>", "stderr": "<base64>", "output": "<base64>", "exit_code": 0, "status": "OK"}}
```
