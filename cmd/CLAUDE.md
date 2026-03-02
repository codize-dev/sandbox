# cmd

CLI entrypoint using Cobra. `root.go` defines the root command; `serve.go` starts the HTTP server.

`gocacheprog/` is a separate binary that serves as a read-only Go module cache helper (`GOCACHEPROG`) used during Go sandbox compilation.
