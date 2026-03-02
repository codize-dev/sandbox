# internal/handler

Request parsing and response formatting.

- Defines `Handler` struct holding a `*sandbox.Runner`
- Validates the `runtime` field and file names (rejects path traversal, slashes, `.`, `..`, empty names, null bytes)
- Decodes base64 file contents from the request and writes them to a temp directory
- Calls `Runner.Run()` — the first file in the `files` array is the entrypoint
- Returns HTTP 400 on invalid input, HTTP 504 on execution timeout
- `RunResponse` contains `Compile *sandbox.Result` and `Run *sandbox.Result`
