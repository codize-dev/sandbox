# nsjail: Project Overview

## Overview

nsjail is a Linux process isolation tool. It leverages Linux Namespaces, cgroups, rlimits, and seccomp-bpf system call filters to run processes inside a lightweight sandbox. It integrates the Kafel BPF language for writing security policies.

- **Developer**: Google (explicitly noted as "not an official Google product")
- **Lead Developer**: Robert Swiecki (Google Staff Information Security Engineer)
- **License**: Apache 2.0
- **Language**: C++20
- **Official Repository**: [github.com/google/nsjail](https://github.com/google/nsjail)
- **Official Website**: [nsjail.dev](https://nsjail.dev/)
- **Initial Release**: May 14, 2015

## Primary Use Cases

Use cases documented in the official documentation:

- Isolating network services (web, DNS, time synchronization, etc.) from the OS
- Hosting CTF (Capture The Flag) computer security challenges
- Fuzzing workflows
- Sandboxing desktop applications (Firefox, Chromium, etc.)
- Running programs with least privilege

Google's [kCTF](https://google.github.io/kctf/) (Kubernetes-based CTF infrastructure) uses nsjail as its core isolation mechanism.

It is also mirrored in the Android platform repository ([android.googlesource.com/platform/external/nsjail](https://android.googlesource.com/platform/external/nsjail/)) and is used in Android build infrastructure.

## Release History

| Version | Reference Commit | Reference Date (git) | Notable Changes |
|-----------|-------------|-------------|-----------|
| 3.4 | `079d70d` | 2023-10-04 | Enhanced cgroup v2 support, improved Docker interoperability, improved clone3 support, improved signal handling |
| 3.3 | `c7c0adf` | 2022-11-22 | Build improvements, cgroup v2 controller setup fixes |
| 3.2 | `2e62649` | 2022-10-14 | C++14 upgrade, atomic operations in signal handlers, CPU affinity improvements, fatal signal forwarding |
| 3.1 | `6483728` | 2022-03-15 | New capabilities (CAP_BPF, CAP_PERFMON, CAP_CHECKPOINT_RESTORE), global connection limit, rlimit extensions, TSC disabling |
| 3.0 | `7de87ae` | 2020-07-23 | Convert TCP proxy to socketpair-based architecture, CLONE_NEWPID awareness |
| 2.9 | `3612c2a` | 2019-09-02 | Basic cgroup v2 support, EINTR handling, changed default resource limits |
| 2.8 | `ddd515e` | 2018-11-08 | cgroup code refactoring, mount options (noexec/nodev/nosuid), MACVLAN support |
| 2.7 | `72ed4b5` | 2018-06-12 | SECCOMP_FILTER_FLAG_LOG support, interface isolation feature |
| 2.6 | `cfa3a64` | 2018-04-18 | Bug fixes and documentation |
| 2.5 | `9cbe1c5` | 2018-02-16 | Converted to C++, arbitrary mount options |

Note: Dates in the table above are reference commit dates in the `google/nsjail` repository, not GitHub Releases publication dates.

## Architecture

### Source Code Structure

| Module | File | Role |
|-----------|---------|------|
| Main entry point | `nsjail.cc` | Signal handling, event loop, mode dispatch |
| Command-line parsing | `cmdline.cc` | Definition and parsing of all CLI flags |
| Configuration file parsing | `config.cc` | Reading configuration files in Protobuf TextProto and JSON formats |
| Subprocess management | `subproc.cc` | Process creation via `clone()`/`clone3()`, reaping, and time limits |
| Containment setup | `contain.cc` | Orchestration of all namespace and resource setup inside the child process |
| Mount namespace | `mnt.cc`, `mnt_legacy.cc`, `mnt_newapi.cc` | Filesystem isolation via legacy `mount(2)` or new `fsopen`/`fsmount` APIs |
| Network | `net.cc` | MACVLAN cloning, interface moving, pasta launch, TCP listener |
| User namespace | `user.cc` | UID/GID mapping, `setresuid`/`setresgid` |
| Capabilities | `caps.cc` | Dropping capabilities, bounding set manipulation, ambient set |
| Cgroup v1 | `cgroup.cc` | memory, pids, net_cls, cpu controllers |
| Cgroup v2 | `cgroup2.cc` | memory, pids, cpu controllers in the unified hierarchy |
| Seccomp/BPF | `sandbox.cc` | Compiling Kafel policies and applying seccomp-bpf |
| CPU affinity | `cpu.cc` | CPU restriction via `sched_setaffinity` |
| PID namespace | `pid.cc` | Dummy init process for EXECVE mode |
| UTS namespace | `uts.cc` | `sethostname` |
| Logging | `logs.cc` | Color-coded level-based log output |
| Utilities | `util.cc` | File I/O, string manipulation, kernel version checks, random number generation, rlimit wrappers |

### Core Data Structures

```cpp
struct nsj_t {
    nsjail::NsJailConfig njc;   // protobuf config object (all settings)
    int exec_fd;                // fd for execveat()
    std::vector<std::string> argv; // command and arguments
    uid_t orig_uid, orig_euid;
    std::map<pid_t, pids_t> pids; // active sandbox processes
    std::vector<idmap_t> uids;    // UID mappings
    std::vector<idmap_t> gids;    // GID mappings
    std::vector<int> openfds;     // fds to keep open (default: 0,1,2)
    std::vector<pipemap_t> pipes; // pipe pairs for TCP listen mode
    std::string chroot;
    std::string proc_path;
    bool is_root_rw;
    bool mnt_newapi;             // whether to use the new fsopen/fsmount API
    bool is_proc_rw;
    struct sock_fprog seccomp_fprog; // compiled BPF filter
};

struct pids_t {
    time_t start;
    std::string remote_txt;
    struct sockaddr_in6 remote_addr;
    int pid_syscall_fd;    // /proc/PID/syscall for seccomp violation reporting
    pid_t pasta_pid;       // pasta userland network process
};
```

### Build Dependencies

- `protobuf` (via pkg-config)
- `libnl-route-3` (required for MACVLAN/interface features, optional)
- `kafel` (git submodule, built with `make -C kafel`)

Build flags (COMMON_FLAGS + CXXFLAGS): `-O2 -c -D_GNU_SOURCE -D_FILE_OFFSET_BITS=64 -fPIE -Wformat -Wformat-security -Wno-format-nonliteral -Wall -Wextra -Werror -Ikafel/include -std=c++20 -fno-exceptions -Wno-unused -Wno-unused-parameter`

Link flags: `-pie -Wl,-z,noexecstack -lpthread`

`NEWUIDMAP_PATH` and `NEWGIDMAP_PATH` can be set at build time via `USER_DEFINES`.

### Kernel Version Requirements

| Feature | Required Kernel Version |
|------|----------------------|
| Basic functionality | Linux 3.x or later |
| `CLONE_NEWCGROUP` | 4.6 or later |
| `SECCOMP_FILTER_FLAG_LOG` | 4.14 or later |
| `clone3()` system call | 5.3 or later |
| `CLONE_NEWTIME` | 5.3 or later (`CONFIG_TIME_NS` kernel config required) |
| `CLONE_CLEAR_SIGHAND` | 5.5 or later |
| New mount API (`fsopen`/`fsmount`) | 6.3 or later |

To use unprivileged user namespaces, some distributions (such as Ubuntu) require `sysctl kernel.unprivileged_userns_clone = 1`.
