# Protobuf Configuration File Reference

nsjail configuration files are written in Protocol Buffers text format (TextProto) or JSON format. The schema is defined in `config.proto`.

## Loading a Configuration File

```bash
nsjail -C config.cfg [additional CLI flags]
```

- Specify a configuration file with `-C FILE` / `--config FILE`
- Both JSON and TextProto formats are auto-detected (`google::protobuf::util::JsonStringToMessage` is tried first, then `google::protobuf::TextFormat::ParseFromString`; exactly one must succeed. If both succeed, the input is considered ambiguous and an error is returned)
- Configuration files and CLI flags can be combined
- Priority follows "order of appearance in arguments". In general, the value processed later takes effect (processed sequentially by `getopt_long`)
- Example: with `--mode o -C cfg`, the `mode` in `cfg` may become the final value. With `-C cfg --mode o`, the CLI value becomes the final value

## Enumerations

### Mode

```proto
enum Mode {
    LISTEN = 0;   // TCP listen mode
    ONCE = 1;     // Execute once (default)
    RERUN = 2;    // Execute repeatedly
    EXECVE = 3;   // unshare + execve
}
```

### LogLevel

```proto
enum LogLevel {
    DEBUG = 0;
    INFO = 1;      // default
    WARNING = 2;
    ERROR = 3;
    FATAL = 4;
}
```

### RLimit

```proto
enum RLimit {
    VALUE = 0;   // Use the specified numeric value
    SOFT = 1;    // Use the system soft limit
    HARD = 2;    // Use the system hard limit
    INF = 3;     // Unlimited (RLIM64_INFINITY)
}
```

## Message Types

### IdMap

```proto
message IdMap {
    optional string inside_id = 1 [default = ""];
    optional string outside_id = 2 [default = ""];
    optional uint32 count = 3 [default = 1];
    optional bool use_newidmap = 4 [default = false];
}
```

### MountPt

```proto
message MountPt {
    optional string src = 1 [default = ""];
    optional string prefix_src_env = 2 [default = ""];
    optional bytes src_content = 3 [default = ""];
    required string dst = 4 [default = ""];
    optional string prefix_dst_env = 5 [default = ""];
    optional string fstype = 6 [default = ""];
    optional string options = 7 [default = ""];
    optional bool is_bind = 8 [default = false];
    optional bool rw = 9 [default = false];
    optional bool is_dir = 10;
    optional bool mandatory = 11 [default = true];
    optional bool is_symlink = 12 [default = false];
    optional bool nosuid = 13 [default = false];
    optional bool nodev = 14 [default = false];
    optional bool noexec = 15 [default = false];
}
```

### Exe

```proto
message Exe {
    required string path = 1;
    repeated string arg = 2;
    optional string arg0 = 3;
    optional bool exec_fd = 4 [default = false];
}
```

### TrafficRule (nested message within NsJailConfig)

```proto
message TrafficRule {
    enum TrafficAction {
        UNKNOWN_ACTION = 0;
        DROP = 1;
        REJECT = 2;
        ALLOW = 3;
    }
    enum IpFamily {
        IPV4 = 0;
        IPV6 = 1;
    }
    enum Protocol {
        UNKNOWN_PROTO = 0;
        TCP = 1;
        UDP = 2;
        ICMP = 3;
        ICMPV6 = 4;
    }

    optional string src_ip = 1;
    optional string dst_ip = 2;
    optional string iif = 3;
    optional string oif = 4;
    optional Protocol proto = 5 [default = UNKNOWN_PROTO];
    optional uint32 sport = 6;
    optional uint32 dport = 7;
    optional uint32 sport_end = 8;
    optional uint32 dport_end = 9;
    optional TrafficAction action = 10 [default = DROP];
    optional IpFamily ip_family = 15 [default = IPV4];
}
```

### UserNet (nested message within NsJailConfig)

```proto
message UserNet {
    enum Backend {
        NSTUN = 0;
        PASTA = 1;
    }

    optional Backend backend = 1 [default = NSTUN];
    optional Pasta pasta = 2;
    repeated NstunRule rule4 = 3;
    repeated NstunRule rule6 = 12;
    optional string ip4 = 4 [default = "10.255.255.2"];
    optional string gw4 = 5 [default = "10.255.255.1"];
    optional string ip6 = 6 [default = "fc00::2"];
    optional string gw6 = 7 [default = "fc00::1"];
    optional string ns_iface = 8 [default = "eth0"];
}
```

### Pasta (nested message within UserNet)

```proto
message Pasta {
    optional bool nat = 1 [default = true];
    optional bool enable_tcp = 2 [default = true];
    optional bool enable_udp = 3 [default = true];
    optional bool enable_icmp = 4 [default = true];
    optional bool ip4_enabled = 5 [default = true];
    optional string mask4 = 6 [default = "255.255.255.0"];
    optional bool enable_ip4_dhcp = 7 [default = false];
    optional bool ip6_enabled = 8 [default = true];
    optional string mask6 = 9 [default = "64"];
    optional bool enable_ip6_dhcp = 10 [default = false];
    optional bool enable_ip6_ra = 11 [default = false];
    optional bool enable_dns = 12 [default = false];
    optional string dns_forward = 13 [default = ""];
    optional bool map_gw = 14 [default = true];
    optional string tcp_map_in = 15 [default = "none"];
    optional string udp_map_in = 16 [default = "none"];
    optional string tcp_map_out = 17 [default = "none"];
    optional string udp_map_out = 18 [default = "none"];
}
```

### NstunRule (nested message within UserNet)

```proto
message NstunRule {
    enum Direction {
        GUEST_TO_HOST = 0;
        HOST_TO_GUEST = 1;
    }
    enum Action {
        DROP = 0;
        REJECT = 1;
        ALLOW = 2;
        REDIRECT = 3;
        ENCAP_SOCKS5 = 4;
        ENCAP_CONNECT = 5;
    }
    enum Protocol {
        ANY = 0;
        TCP = 1;
        UDP = 2;
        ICMP = 3;
    }

    optional Direction direction = 11 [default = GUEST_TO_HOST];
    optional Action action = 1 [default = ALLOW];
    optional Protocol proto = 2 [default = ANY];
    optional string src_ip = 3;
    optional uint32 sport = 4;
    optional uint32 sport_end = 5;
    optional string dst_ip = 6;
    optional uint32 dport = 7;
    optional uint32 dport_end = 8;
    optional string redirect_ip = 9;
    optional uint32 redirect_port = 10;
}
```

## NsJailConfig Full Field List

### Basic Settings (Fields 1-6)

| # | Field | Type | Default | Description |
|---|-------|------|---------|-------------|
| 1 | `name` | string | `""` | Configuration name (human-readable) |
| 2 | `description` | repeated string | — | Description text (multi-line) |
| 3 | `mode` | Mode | `ONCE` | Execution mode |
| 4 | `hostname` | string | `"NSJAIL"` | UTS hostname |
| 5 | `cwd` | string | `"/"` | Working directory inside the jail |
| 6 | `no_pivotroot` | bool | `false` | Use MS_MOVE + chroot instead of pivot_root |

### TCP Listen Mode (Fields 7-10)

| # | Field | Type | Default | Description |
|---|-------|------|---------|-------------|
| 7 | `port` | uint32 | `0` | TCP listen port |
| 8 | `bindhost` | string | `"::"` | Bind address |
| 9 | `max_conns` | uint32 | `0` | Maximum concurrent connections (0 = unlimited) |
| 10 | `max_conns_per_ip` | uint32 | `0` | Maximum connections per IP (0 = unlimited) |

### Time / CPU / Priority (Fields 11-14)

| # | Field | Type | Default | Description |
|---|-------|------|---------|-------------|
| 11 | `time_limit` | uint32 | `600` | Wall clock time limit (seconds, 0 = unlimited) |
| 12 | `daemon` | bool | `false` | Daemonize |
| 13 | `max_cpus` | uint32 | `0` | CPU count limit (0 = no limit) |
| 14 | `nice_level` | int32 | `19` | nice value (-20 to 19) |

### Logging (Fields 15-17)

| # | Field | Type | Default | Description |
|---|-------|------|---------|-------------|
| 15 | `log_fd` | int32 | — | Log output fd |
| 16 | `log_file` | string | — | Log file path |
| 17 | `log_level` | LogLevel | `INFO` | Log level (Note: the proto2 definition default is `DEBUG` (value 0), but when unset `has_log_level()` returns `false` and `setLogLevel()` is not called, so the runtime initial value of `INFO` is maintained) |

### Environment Variables (Fields 18-19)

| # | Field | Type | Default | Description |
|---|-------|------|---------|-------------|
| 18 | `keep_env` | bool | `false` | Pass all host environment variables |
| 19 | `envar` | repeated string | — | Set environment variables (value-less form uses the current value) |

### Capabilities (Fields 20-21)

| # | Field | Type | Default | Description |
|---|-------|------|---------|-------------|
| 20 | `keep_caps` | bool | `false` | Retain all capabilities |
| 21 | `cap` | repeated string | — | Individual capabilities to retain |

### I/O and Process Control (Fields 22-29)

| # | Field | Type | Default | Description |
|---|-------|------|---------|-------------|
| 22 | `silent` | bool | `false` | Redirect fd 0/1/2 to /dev/null |
| 23 | `skip_setsid` | bool | `false` | Do not call setsid() |
| 24 | `stderr_to_null` | bool | `false` | Redirect only fd 2 to /dev/null |
| 25 | `pass_fd` | repeated int32 | — | Additional fds to pass (default: only 0, 1, 2) |
| 26 | `disable_no_new_privs` | bool | `false` | Do not set PR_SET_NO_NEW_PRIVS |
| 27 | `forward_signals` | bool | `false` | Forward fatal signals to child |
| 28 | `disable_tsc` | bool | `false` | Disable rdtsc/rdtscp (x86 only) |
| 29 | `oom_score_adj` | int32 | — | OOM score adjustment for the sandbox (-1000 to 1000) |

### rlimit (Fields 30-50)

| # | Field | Type | Default | Description |
|---|-------|------|---------|-------------|
| 30 | `rlimit_as` | uint64 | `4096` | Virtual address space (MiB) |
| 31 | `rlimit_as_type` | RLimit | `VALUE` | |
| 32 | `rlimit_core` | uint64 | `0` | Core dump (MiB) |
| 33 | `rlimit_core_type` | RLimit | `VALUE` | |
| 34 | `rlimit_cpu` | uint64 | `600` | CPU time (seconds) |
| 35 | `rlimit_cpu_type` | RLimit | `VALUE` | |
| 36 | `rlimit_fsize` | uint64 | `1` | File size (MiB) |
| 37 | `rlimit_fsize_type` | RLimit | `VALUE` | |
| 38 | `rlimit_nofile` | uint64 | `32` | Number of open fds |
| 39 | `rlimit_nofile_type` | RLimit | `VALUE` | |
| 40 | `rlimit_nproc` | uint64 | `1024` | Number of processes |
| 41 | `rlimit_nproc_type` | RLimit | `SOFT` | |
| 42 | `rlimit_stack` | uint64 | `8` | Stack (MiB) |
| 43 | `rlimit_stack_type` | RLimit | `SOFT` | |
| 44 | `rlimit_memlock` | uint64 | `64` | Locked memory (KiB) |
| 45 | `rlimit_memlock_type` | RLimit | `SOFT` | |
| 46 | `rlimit_rtprio` | uint64 | `0` | Real-time priority |
| 47 | `rlimit_rtprio_type` | RLimit | `SOFT` | |
| 48 | `rlimit_msgqueue` | uint64 | `1024` | Message queue (bytes) |
| 49 | `rlimit_msgqueue_type` | RLimit | `SOFT` | |
| 50 | `disable_rl` | bool | `false` | Disable all rlimits |

### personality Flags (Fields 51-55)

| # | Field | Type | Default | Description |
|---|-------|------|---------|-------------|
| 51 | `persona_addr_compat_layout` | bool | `false` | ADDR_COMPAT_LAYOUT |
| 52 | `persona_mmap_page_zero` | bool | `false` | MMAP_PAGE_ZERO |
| 53 | `persona_read_implies_exec` | bool | `false` | READ_IMPLIES_EXEC |
| 54 | `persona_addr_limit_3gb` | bool | `false` | ADDR_LIMIT_3GB |
| 55 | `persona_addr_no_randomize` | bool | `false` | ADDR_NO_RANDOMIZE |

### Namespaces (Fields 56-63)

| # | Field | Type | Default | Description |
|---|-------|------|---------|-------------|
| 56 | `clone_newnet` | bool | `true` | Network namespace |
| 57 | `clone_newuser` | bool | `true` | User namespace |
| 58 | `clone_newns` | bool | `true` | Mount namespace |
| 59 | `clone_newpid` | bool | `true` | PID namespace |
| 60 | `clone_newipc` | bool | `true` | IPC namespace |
| 61 | `clone_newuts` | bool | `true` | UTS namespace |
| 62 | `clone_newcgroup` | bool | `true` | Cgroup namespace (kernel 4.6 or later) |
| 63 | `clone_newtime` | bool | `false` | Time namespace (kernel 5.3 or later) |

### ID Mapping (Fields 64-65)

| # | Field | Type | Default | Description |
|---|-------|------|---------|-------------|
| 64 | `uidmap` | repeated IdMap | — | UID mapping |
| 65 | `gidmap` | repeated IdMap | — | GID mapping |

### Mounts (Fields 66-67)

| # | Field | Type | Default | Description |
|---|-------|------|---------|-------------|
| 66 | `mount_proc` | bool | `false` | Mount /proc (Note: in the CLI, `proc_path` is pre-set to `/proc` as its default value, so /proc is mounted by default unless `--disable_proc` is explicitly specified) |
| 67 | `mount` | repeated MountPt | — | Mount points |

### seccomp (Fields 68-70, 99-100)

| # | Field | Type | Default | Description |
|---|-------|------|---------|-------------|
| 68 | `seccomp_policy_file` | string | — | Kafel policy file |
| 69 | `seccomp_string` | repeated string | — | Inline Kafel policy |
| 70 | `seccomp_log` | bool | `false` | SECCOMP_FILTER_FLAG_LOG |
| 99 | `seccomp_unotify` | bool | `false` | Enable seccomp user notification |
| 100 | `seccomp_unotify_report` | string | — | Seccomp unotify report file path |

### cgroup v1 (Fields 71-84)

| # | Field | Type | Default | Description |
|---|-------|------|---------|-------------|
| 71 | `cgroup_mem_max` | uint64 | `0` | Memory limit (bytes) |
| 72 | `cgroup_mem_memsw_max` | uint64 | `0` | Memory+swap limit (bytes) |
| 73 | `cgroup_mem_swap_max` | int64 | `-1` | Swap limit (bytes) |
| 74 | `cgroup_mem_mount` | string | `/sys/fs/cgroup/memory` | memory mount |
| 75 | `cgroup_mem_parent` | string | `NSJAIL` | memory parent cgroup |
| 76 | `cgroup_pids_max` | uint64 | `0` | PID limit |
| 77 | `cgroup_pids_mount` | string | `/sys/fs/cgroup/pids` | pids mount |
| 78 | `cgroup_pids_parent` | string | `NSJAIL` | pids parent cgroup |
| 79 | `cgroup_net_cls_classid` | uint32 | `0` | net_cls class ID |
| 80 | `cgroup_net_cls_mount` | string | `/sys/fs/cgroup/net_cls` | net_cls mount |
| 81 | `cgroup_net_cls_parent` | string | `NSJAIL` | net_cls parent cgroup |
| 82 | `cgroup_cpu_ms_per_sec` | uint32 | `0` | CPU ms/second |
| 83 | `cgroup_cpu_mount` | string | `/sys/fs/cgroup/cpu` | cpu mount |
| 84 | `cgroup_cpu_parent` | string | `NSJAIL` | cpu parent cgroup |

### cgroup v2 (Fields 85-87)

| # | Field | Type | Default | Description |
|---|-------|------|---------|-------------|
| 85 | `cgroupv2_mount` | string | `/sys/fs/cgroup` | cgroup v2 mount |
| 86 | `use_cgroupv2` | bool | `false` | Explicitly use cgroup v2 |
| 87 | `detect_cgroupv2` | bool | `false` | Auto-detect cgroup v2 |

### Network (Fields 88-97)

| # | Field | Type | Default | Description |
|---|-------|------|---------|-------------|
| 88 | `iface_no_lo` | bool | `false` | Do not bring up loopback |
| 89 | `iface_own` | repeated string | — | Interfaces to move |
| 90 | `macvlan_iface` | string | — | MACVLAN clone source |
| 91 | `macvlan_vs_ip` | string | `192.168.0.2` | MACVLAN IP |
| 92 | `macvlan_vs_nm` | string | `255.255.255.0` | MACVLAN netmask |
| 93 | `macvlan_vs_gw` | string | `192.168.0.1` | MACVLAN gateway |
| 94 | `macvlan_vs_ma` | string | `""` | MACVLAN MAC address |
| 95 | `macvlan_vs_mo` | string | `"private"` | MACVLAN mode |
| 96 | `traffic_rule` | repeated TrafficRule | — | Network traffic filtering rules |
| 97 | `user_net` | UserNet | — | User network configuration (nstun/pasta) |

### Executable Binary (Field 98)

| # | Field | Type | Default | Description |
|---|-------|------|---------|-------------|
| 98 | `exec_bin` | Exe | — | Binary to execute |

## Configuration File Example (TextProto format)

```proto
name: "example-jail"
description: "Example nsjail configuration"

mode: ONCE
hostname: "sandbox"
cwd: "/app"

time_limit: 30

clone_newnet: true
clone_newuser: true
clone_newns: true
clone_newpid: true

uidmap {
    inside_id: "0"
    outside_id: "1000"
    count: 1
}

gidmap {
    inside_id: "0"
    outside_id: "1000"
    count: 1
}

mount_proc: true

mount {
    src: "/lib"
    dst: "/lib"
    is_bind: true
    rw: false
}

mount {
    dst: "/tmp"
    fstype: "tmpfs"
    rw: true
    options: "size=16777216"
}

rlimit_as: 512
rlimit_as_type: VALUE
rlimit_fsize: 10
rlimit_fsize_type: VALUE

exec_bin {
    path: "/bin/bash"
}
```

## Configuration File Example (JSON format)

```json
{
    "name": "example-jail",
    "mode": "ONCE",
    "hostname": "sandbox",
    "cwd": "/app",
    "time_limit": 30,
    "clone_newnet": true,
    "uidmap": [
        {"inside_id": "0", "outside_id": "1000", "count": 1}
    ],
    "mount": [
        {"src": "/lib", "dst": "/lib", "is_bind": true, "rw": false}
    ],
    "exec_bin": {
        "path": "/bin/bash"
    }
}
```

## Bundled Configuration File Examples

The `configs/` directory in the nsjail repository contains 15 example configuration files:

| File | Description |
|------|-------------|
| `apache.cfg` | Apache Web server |
| `bash-with-fake-geteuid.cfg` | bash with geteuid faked via seccomp |
| `bash-with-fake-geteuid.json` | JSON format version of the above |
| `chromium-with-net-wayland.cfg` | Chromium (Wayland + network) |
| `firefox-with-net-X11.cfg` | Firefox (X11 + network) |
| `firefox-with-net-wayland.cfg` | Firefox (Wayland + pasta network) |
| `hexchat-with-net.cfg` | HexChat |
| `home-documents-with-xorg-no-net.cfg` | Document viewer (X11, no network) |
| `imagemagick-convert.cfg` | ImageMagick convert (strict seccomp whitelist) |
| `static-busybox-with-execveat.cfg` | Statically linked BusyBox (using exec_fd) |
| `telegram.cfg` | Telegram |
| `tomcat8.cfg` | Tomcat 8 |
| `weechat-with-net.cfg` | WeeChat |
| `xchat-with-net.cfg` | XChat |
| `znc-with-net.cfg` | ZNC |
