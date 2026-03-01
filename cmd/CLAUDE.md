# cmd

CLI entrypoint using Cobra.

- `root.go` defines the root command
- `serve.go` registers the `serve` subcommand that starts the Echo v5 HTTP server with request logging middleware
  - Single route: `POST /v1/run`
  - Accepts `--addr` (default `:8080`), `--timeout` (default `30`), and `--output-limit` (default 1 MiB) flags
