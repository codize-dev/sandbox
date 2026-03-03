# nsjail Documentation

Comprehensive reference documentation on nsjail specifications, configuration, and internals.

## Table of Contents

| File | Contents |
|---------|------|
| [01-overview.md](01-overview.md) | Project overview, history, and architecture |
| [02-execution-modes.md](02-execution-modes.md) | Execution modes (ONCE / LISTEN / RERUN / EXECVE) |
| [03-namespaces.md](03-namespaces.md) | Linux Namespace usage (8 types) |
| [04-filesystem.md](04-filesystem.md) | Filesystem isolation and mount configuration |
| [05-network.md](05-network.md) | Network isolation (MACVLAN / pasta / iface_own) |
| [06-seccomp.md](06-seccomp.md) | System call filtering via Seccomp-BPF |
| [07-kafel.md](07-kafel.md) | Kafel policy language reference |
| [08-cgroups.md](08-cgroups.md) | Resource control via Cgroup v1 / v2 |
| [09-resource-limits.md](09-resource-limits.md) | rlimit, time limits, and CPU affinity |
| [10-security-features.md](10-security-features.md) | Capabilities, no_new_privs, TSC, and personality |
| [11-process-lifecycle.md](11-process-lifecycle.md) | Process lifecycle and containment sequence |
| [12-configuration-reference.md](12-configuration-reference.md) | Full reference for all Protobuf configuration file fields |
| [13-cli-reference.md](13-cli-reference.md) | Full reference for all command-line options |

## Sources

- [github.com/google/nsjail](https://github.com/google/nsjail) — Official repository
- [nsjail.dev](https://nsjail.dev/) — Official website
- [config.proto](https://github.com/google/nsjail/blob/master/config.proto) — Protobuf schema definition
- [nsjail.1](https://github.com/google/nsjail/blob/master/nsjail.1) — man page
- [google/kafel](https://github.com/google/kafel) — Kafel policy language
- nsjail source code (C++20)
