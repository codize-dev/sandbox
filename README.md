<h1 align="center">
Codize Sandbox
</h1>

<p align="center">
<i>
A sandboxed code execution engine.
</i>
</p>

<p align="center">

<a href="https://github.com/codize-dev/sandbox/releases/latest">
<img alt="GitHub Release" src="https://img.shields.io/github/v/release/codize-dev/sandbox">
</a>

<a href="https://github.com/codize-dev/sandbox/actions/workflows/ci.yml">
<img alt="GitHub Actions Workflow Status" src="https://img.shields.io/github/actions/workflow/status/codize-dev/sandbox/ci.yml">
</a>

<a href="https://goreportcard.com/report/github.com/codize-dev/sandbox">
<img alt="Go Report Card" src="https://goreportcard.com/badge/github.com/codize-dev/sandbox">
</a>

<a href="./LICENSE">
<img alt="GitHub License" src="https://img.shields.io/github/license/codize-dev/sandbox">
</a>

</p>

Codize Sandbox is a code execution engine that runs arbitrary code safely inside Linux namespace jails ([google/nsjail](https://github.com/google/nsjail)). It exposes an HTTP API to receive code, execute it in an isolated environment, and return the output.

## Supported Runtimes

| Runtime | Identifier |
| --- | --- |
| Node.js | `node` |
| TypeScript | `node-typescript` |
| Ruby | `ruby` |
| Go | `go` |
| Python | `python` |
| Rust | `rust` |
| Bash | `bash` |

## Usage

The container must run in **privileged mode** (required for nsjail to create Linux namespaces) with **`--cgroupns=host`** (required for nsjail to manage cgroups for resource limiting).

### Docker

```console
$ docker run \
    --privileged \
    --cgroupns=host \
    -p 8080:8080 \
    ghcr.io/codize-dev/sandbox:latest serve
```

Behavior can be customized via CLI flags (see [CLI Flags](#cli-flags) for the full list):

```console
$ docker run \
    --privileged \
    --cgroupns=host \
    -p 8080:8080 \
    ghcr.io/codize-dev/sandbox:latest serve --run-timeout 10 --compile-timeout 10
```

### Docker Compose

Create a `compose.yml`:

```yaml
services:
  sandbox:
    image: ghcr.io/codize-dev/sandbox:latest
    privileged: true
    cgroup: host
    command: ["serve", "--run-timeout", "10", "--compile-timeout", "10"]
    ports:
      - "8080:8080"
```

```console
$ docker compose up
```

### CLI Flags

| Flag | Default | Description |
| --- | --- | --- |
| `--port` | `8080` (overridden by `PORT` env var) | Listen port |
| `--run-timeout` | `30` | Run timeout in seconds |
| `--compile-timeout` | `30` | Compile timeout in seconds |
| `--output-limit` | `1048576` (1 MiB) | Maximum combined output bytes |
| `--max-files` | `10` | Maximum number of files per request |
| `--max-file-size` | `262144` (256 KiB) | Maximum file size in bytes |
| `--max-body-size` | `5242880` (5 MiB) | Maximum request body size in bytes |
| `--max-concurrency` | `10` | Maximum number of concurrent sandbox executions |
| `--max-queue-size` | `50` | Maximum number of requests waiting in the execution queue |
| `--queue-timeout` | `30` | Maximum time in seconds a request waits in the execution queue |

### API

#### `GET /healthz`

Returns the service health status. Intended for load balancer health checks, Docker health checks, and Kubernetes liveness probes.

Response:

```json
{"status":"ok"}
```

#### `POST /v1/run`

Request:

```json
{
  "runtime": "node",
  "files": [
    {
      "name": "index.js",
      "content": "Y29uc29sZS5sb2coIkhlbGxvLCBXb3JsZCEiKQ=="
    }
  ]
}
```

- `runtime` (required): one of `"node"`, `"node-typescript"`, `"ruby"`, `"go"`, `"python"`, `"rust"`, `"bash"`
- `files` (required): array of source files. `content` is Base64-encoded. The first file in the array is used as the entrypoint

Response:

```json
{
  "compile": null,
  "run": {
    "stdout": "SGVsbG8sIFdvcmxkIQo=",
    "stderr": "",
    "output": "SGVsbG8sIFdvcmxkIQo=",
    "exit_code": 0,
    "status": "OK",
    "signal": null
  }
}
```

- `compile`: compilation result (same schema as `run`). `null` for interpreted runtimes (node, ruby, python, bash). When compilation fails, `run` is `null`
- `run`: execution result. `null` when compilation fails
  - `stdout` / `stderr` / `output`: Base64-encoded output. `output` is the interleaved combination of stdout and stderr
  - `exit_code`: process exit code
  - `status`: one of `"OK"`, `"SIGNAL"`, `"TIMEOUT"`, `"OUTPUT_LIMIT_EXCEEDED"`
  - `signal`: signal name if the process was killed by a signal (e.g. `"SIGKILL"`), `null` otherwise

## How It Works

### Architecture

```
POST /v1/run
  → Echo HTTP server (request validation, Base64 decoding, write files to tmpdir)
    → nsjail (execute code in a namespace-isolated environment)
      → Return response
```

### Sandbox Isolation

Code is isolated by [google/nsjail](https://github.com/google/nsjail) with multiple layers of defense:

- **Linux namespaces**: PID, network, mount, UTS, IPC, and cgroup namespaces are all isolated. External network access is completely blocked, and loopback communication is also disabled.
- **UID/GID mapping**: Sandboxed processes run as nobody (65534). Only a single UID is mapped, making setuid impossible.
- **Filesystem restrictions**: Only the minimum required paths are mounted (shared libraries, device files, user code directory). Everything except the user code directory is read-only. `/tmp` is a 64 MiB tmpfs mounted with noexec.
- **Resource limits**: Execution time is enforced by nsjail's `--time_limit` and `--rlimit_cpu`. Cgroups limit PID count, memory, and CPU usage. Rlimits constrain stack size and other per-process resources.
- **Seccomp-BPF**: Dangerous syscalls (io_uring, bpf, mount, ptrace, unshare, etc.) are blocked at the kernel level. Clone calls with namespace creation flags are also blocked.
- **Output limits**: The process is killed if the combined stdout and stderr exceeds the configured limit.

## License

[MIT](./LICENSE)
