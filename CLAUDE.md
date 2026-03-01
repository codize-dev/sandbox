# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A code execution sandbox service that runs arbitrary code inside Linux namespace jails (nsjail). Supports Node.js and Ruby runtimes. Exposes an HTTP API to receive code, execute it in an isolated environment, and return the output.

## Development Setup

Tool versions are managed by [mise](https://mise.jdx.dev/). Run `mise install` to install Go, golangci-lint, and lefthook.

- **Go**: 1.26.0 (mise.toml), module requires 1.25.0 (go.mod)
- **golangci-lint**: 2.10.1 (installed via mise aqua backend)
- **lefthook**: 2.1.2 (installed via mise aqua backend) — runs `golangci-lint run` as a pre-commit hook

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

- **cmd/** — CLI entrypoint using Cobra. See `cmd/CLAUDE.md`.
- **internal/handler/** — Request parsing and response formatting. See `internal/handler/CLAUDE.md`.
- **internal/sandbox/** — Core sandbox execution logic. See `internal/sandbox/CLAUDE.md`.

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
