# Version-Agnostic Runtime Paths

## Problem

Runtime versions are defined as Dockerfile ARGs (e.g., `GO_VERSION=1.26.1`) and hardcoded in `internal/sandbox/runtime.go` as literal paths (e.g., `/mise/installs/go/1.26.0/bin/go`). When Renovate updates Dockerfile ARGs, runtime.go is not updated, causing a mismatch.

This affects: Node.js, Ruby, Go, Python (4 runtimes). Rust is unaffected because its paths (`/mise/cargo`, `/mise/rustup`) don't contain version numbers.

## Solution

Use version-agnostic symlinks created during Docker build.

### Dockerfile Changes

After each `mise use` command, create a `current` symlink:

```
/mise/installs/node/current -> /mise/installs/node/${NODE_VERSION}
/mise/installs/ruby/current -> /mise/installs/ruby/${RUBY_VERSION}
/mise/installs/go/current   -> /mise/installs/go/${GO_VERSION}
/mise/installs/python/current -> /mise/installs/python/${PYTHON_VERSION}
```

### runtime.go Changes

Replace all version-specific paths with `current`:

- `/mise/installs/node/24.14.0` -> `/mise/installs/node/current`
- `/mise/installs/ruby/3.4.8` -> `/mise/installs/ruby/current`
- `/mise/installs/go/1.26.0` -> `/mise/installs/go/current`
- `/mise/installs/python/3.13.12` -> `/mise/installs/python/current`

### Out of Scope

- Rust `RUSTUP_TOOLCHAIN=1.94.0` env var (no version in filesystem paths)
- Builder stage `golang:1.26.1-bookworm` (build toolchain, not sandbox runtime)
- `go.mod` Go version
