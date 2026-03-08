# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A code execution sandbox service that runs arbitrary code inside Linux namespace jails (nsjail). Supports Node.js, Ruby, Go, Python, Rust, and Bash runtimes. Exposes an HTTP API to receive code, execute it in an isolated environment, and return the output.

## Development Setup

Tool versions are managed by [mise](https://mise.jdx.dev/). Run `mise install` to install Go, golangci-lint, and lefthook.

- **Go**: 1.26.0 (mise.toml), module requires 1.25.0 (go.mod)
- **golangci-lint**: 2.10.1 (installed via mise aqua backend)
- **lefthook**: 2.1.2 (installed via mise aqua backend) — pre-commit hook runs `go fmt ./...`, `golangci-lint run`, and `gitleaks git --pre-commit --staged`; pre-push hook runs `gitleaks git`

## Build & Run Commands

```bash
# Build the server binary
go build -o sandbox .

# Run locally (requires nsjail + Node.js + Ruby + Go at hardcoded paths; use Docker instead)
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

### CLI Flags (`serve` subcommand)

- `--port` (default `8080`, overridden by `PORT` env var) — port to listen on
- `--run-timeout` (default `30`) — sandbox run timeout in seconds
- `--compile-timeout` (default `30`) — sandbox compile timeout in seconds
- `--output-limit` (default `1048576` / 1 MiB) — maximum combined output bytes
- `--max-files` (default `10`) — maximum number of files per request
- `--max-file-size` (default `262144` / 256 KiB) — maximum file size in bytes per file
- `--max-body-size` (default `5242880` / 5 MiB) — maximum request body size in bytes
- `--max-concurrency` (default `10`) — maximum number of concurrent sandbox executions
- `--max-queue-size` (default `50`) — maximum number of requests waiting in the execution queue
- `--queue-timeout` (default `30`) — maximum time in seconds a request waits in the execution queue

## Architecture

### Request Flow

```
POST /v1/run → main.go → cmd/serve.go (Cobra CLI, Echo v5 router)
             → internal/handler/handler.go (validate runtime, reject restricted files, decode base64 files, validate filenames, write to tmpdir)
             → internal/sandbox/sandbox.go (invoke nsjail with the selected runtime)
             → Response: {compile, run} where each contains {stdout, stderr, output, exit_code, status, signal} (stdout/stderr/output are base64-encoded)
```

### Key Packages

- **cmd/** — CLI entrypoint using Cobra.
- **cmd/gocacheprog/** — Read-only Go module cache helper used during compilation.
- **internal/handler/** — Request parsing and response formatting.
- **internal/sandbox/** — Core sandbox execution logic; `configs/nsjail.cfg` holds the static nsjail protobuf config; `configs/seccomp.kafel` holds the Seccomp-BPF syscall filtering policy.
- **internal/middleware/** — Custom Echo middleware. Concurrency limiter with queue management.
- **e2e/** — YAML-driven E2E test suite. Test cases live under `e2e/tests/runtime/`, `e2e/tests/security/`, and `e2e/tests/api/`. See `e2e/CLAUDE.md` for testing guidelines.

### Docker Build

Four-stage Dockerfile (`mise` → `base` → `builder` → final). The `base` stage uses `ghcr.io/codize-dev/nsjail` (based on `debian:bookworm-slim`) and installs language runtimes via mise. The `builder` stage compiles both the main `sandbox` binary and the `gocacheprog` helper. See Dockerfile for details.

### Sister Repository

The nsjail Docker base image is built from the `codize-dev/nsjail` repository and published to `ghcr.io/codize-dev/nsjail`.

### nsjail Reference Docs

Comprehensive nsjail reference documentation lives in `.context/docs/nsjail/`. Covers execution modes, namespaces, filesystem isolation, networking, Seccomp-BPF, Kafel policy language, cgroups, resource limits, security features, process lifecycle, configuration reference, and CLI reference. Consult these docs when modifying nsjail settings or hardening the sandbox.

## API

### POST /v1/run

Request (`runtime` is required, must be `"node"`, `"ruby"`, `"go"`, `"python"`, `"rust"`, or `"bash"`):
```json
{"runtime": "node", "files": [{"name": "index.js", "content": "<base64-encoded source>"}]}
```

Response:
```json
{"compile": null, "run": {"stdout": "<base64>", "stderr": "<base64>", "output": "<base64>", "exit_code": 0, "status": "OK", "signal": null}}
```

Possible `status` values: `"OK"`, `"SIGNAL"`, `"TIMEOUT"`, `"OUTPUT_LIMIT_EXCEEDED"`.

`compile`: Compilation step result (same schema as `run`). `null` for non-compiled runtimes (node, ruby, python, bash). When compilation fails, `run` is `null`.
