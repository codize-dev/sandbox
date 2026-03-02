# internal/sandbox

Core sandbox execution engine, split across three files:

- **sandbox.go** — `Runner` orchestrates the full execution lifecycle: applies default files, runs compilation (if needed) and execution steps, and collects results.
- **runtime.go** — `Runtime` interface and concrete implementations for each supported language. `CompiledRuntime` extends it for languages requiring a build step.
- **execution.go** — Handles a single nsjail invocation: assembles CLI arguments, manages pipes (stdout, stderr, nsjail log), drains output via `poll(2)`, and enforces limits.
- **defaults/go/** — Embedded `go.mod.tmpl` and `go.sum.tmpl` templates applied as default files for Go runtime execution.

Go runtime rejects user-submitted `go.mod` and `go.sum` files (HTTP 400) to enforce use of these defaults.
