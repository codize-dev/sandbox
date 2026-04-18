# Execution Modes

nsjail supports 4 execution modes. They are defined by the `Mode` enum in `config.proto`.

## Mode List

| CLI Flag | Proto enum | Log label | Behavior |
|-----------|------------|-----------|------|
| `-Mo` | `ONCE = 1` | `MODE_STANDALONE_ONCE` | Performs `clone`/`execve` exactly once, then exits with the child process's exit status. **Default.** |
| `-Ml` / `--port PORT` | `LISTEN = 0` | `MODE_LISTEN_TCP` | inetd-style TCP server. Forks a new jail for each connection. |
| `-Mr` | `RERUN = 2` | `MODE_STANDALONE_RERUN` | Repeatedly re-executes the command. Useful for fuzzing. |
| `-Me` | `EXECVE = 3` | `MODE_STANDALONE_EXECVE` | Directly performs `unshare()` → `execve()` without a supervisor fork. |

The "Log label" column shows the display strings used in help text and log messages; the actual mode is represented by the Proto enum value (`nsjail::Mode`).

## ONCE Mode (`-Mo`)

The default mode. nsjail spawns a child process using `clone()` (or `clone3()`) and waits until that process exits. The child process's exit status becomes nsjail's exit status.

```bash
nsjail -Mo --chroot /jail -- /bin/bash
```

## LISTEN Mode (`-Ml`)

An inetd-style server that listens on a TCP port and forks a new jail process for each connection.

### Configuration

| Field | Type | Default | CLI | Description |
|-----------|---|---------|-----|------|
| `port` | uint32 | `0` | `-p` | TCP port to listen on |
| `bindhost` | string | `"::"` | `--bindhost` | Bind address (IPv6; IPv4 is automatically converted to `::ffff:IP`) |
| `max_conns` | uint32 | `0` (unlimited) | `--max_conns` | Global limit on simultaneous connections |
| `max_conns_per_ip` | uint32 | `0` (unlimited) | `-i` | Connection limit per IP address |
| `daemon` | bool | `false` | `-d` | Daemonize after startup |

### Behavioral Details

- Creates an `AF_INET6` socket with `SO_REUSEADDR`, `O_NONBLOCK`, and `SOMAXCONN` backlog
- IPv4 addresses are automatically converted to the `::ffff:IP` format
- `max_conns_per_ip` tracks and limits connections per IP
- For each connection, two pipe pairs are created: one for stdin and one shared for stdout and stderr. The child process receives the pipe endpoints (not the TCP socket directly). The parent bridges data between the TCP socket and the pipe file descriptors using `poll()` + `splice()`
- Teardown is handled on `POLLERR`/`POLLHUP`

Note: Specifying `--port` on the CLI implicitly sets the mode to LISTEN, even without `-Ml`.

```bash
nsjail -Ml --port 8080 --chroot /jail -- /bin/handler
```

## RERUN Mode (`-Mr`)

Executes the command repeatedly. Primarily used in fuzzing workflows. Each child is waited on before the next iteration is launched (serial execution). Internally, RERUN mode uses the same standalone code path as ONCE mode, looping the spawn-and-wait cycle.

```bash
nsjail -Mr --chroot /jail -- /bin/fuzz_target
```

## EXECVE Mode (`-Me`)

Unlike other modes, rather than spawning a new process with `clone()`, this mode calls `unshare()` to isolate namespaces within the current process, then performs `execve()`. There is no supervisor process.

Note: The periodic interval timer used for time limit enforcement is not set in EXECVE mode, since there is no supervisor process to receive the timer signals.

### init Process for PID Namespace

When `CLONE_NEWPID` is enabled, a dummy "ns-init" process that acts as PID 1 inside the PID namespace is spawned via `subproc::cloneProc(CLONE_FS, 0)` (not `fork()`). This is because without a PID 1 process, subsequent `fork()` calls within the namespace will fail with `ENOMEM`. This dummy process is configured with `SA_NOCLDWAIT | SA_NOCLDSTOP` to reap zombie processes, and `PR_SET_PDEATHSIG(SIGKILL)` (so it is killed when the parent dies) and `PR_SET_DUMPABLE(0)` (to prevent core dumps) are also set via `prctl()`.

```bash
nsjail -Me --chroot /jail -- /bin/bash
```

## Execution via execveat

When the `exec_fd` field of the `Exe` message or the `--execute_fd` CLI flag is used, `execveat()` is used instead of `execve()`. This allows executing a binary using a file descriptor rather than a path.

### Use Cases

Useful for executing binaries that do not exist inside the chroot of the mount namespace. The file descriptor is opened before the mount namespace is established, and the binary is executed from that fd via `execveat()`. This enables running statically linked binaries on an empty filesystem (e.g., one containing only `/proc`).

### Protobuf Definition

```proto
message Exe {
    required string path = 1;       // execv path and argv[0]
    repeated string arg = 2;        // argv[1], argv[2], ...
    optional string arg0 = 3;       // override argv[0]
    optional bool exec_fd = 4 [default = false];      // use execveat()
}
```

- `path`: Path to the binary to execute. Used as the path argument to `execv()` when `exec_fd` is false; opened with `O_RDONLY | O_PATH | O_CLOEXEC` when true
- `arg`: Command-line arguments (argv[1] and beyond)
- `arg0`: Overrides argv[0]. If not specified, `path` is used as argv[0]
- `exec_fd`: When true, the binary is executed via a raw syscall invocation: `syscall(__NR_execveat, fd, "", argv, environ, AT_EMPTY_PATH)`

On the CLI, specify as `-- /path/to/cmd args`. CLI specification overrides `exec_bin` in the configuration file.
