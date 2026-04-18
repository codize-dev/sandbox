# Command-Line Options Reference

```
nsjail [options] -- command-path [arguments...]
```

## General

| Option | Description |
|--------|-------------|
| `--help` / `-h` | Show help |
| `--mode VALUE` / `-M VALUE` | Execution mode: `l` (LISTEN), `o` (ONCE, default), `e` (EXECVE), `r` (RERUN) |
| `--config VALUE` / `-C VALUE` | Protobuf configuration file |
| `--exec_file VALUE` / `-x VALUE` | File to execute (default: argv[0]) |
| `--execute_fd` | Use fd-based execution via execveat() |

## Jail and Filesystem

| Option | Description |
|--------|-------------|
| `--chroot VALUE` / `-c VALUE` | Root directory for the jail (default: none) |
| `--rw` | Mount chroot as read-write (default: read-only) |
| `--no_pivotroot` | Use mount(MS_MOVE) + chroot instead of pivot_root |
| `--bindmount_ro VALUE` / `-R VALUE` | Read-only bind mount (`src` or `src:dst`) |
| `--bindmount VALUE` / `-B VALUE` | Read-write bind mount (`src` or `src:dst` or `src:dst:options`) |
| `--tmpfsmount VALUE` / `-T VALUE` | tmpfs mount (default size 4 MiB) |
| `--mount VALUE` / `-m VALUE` | Arbitrary mount (`src:dst:fstype:options`) |
| `--symlink VALUE` / `-s VALUE` | Create a symbolic link (`src:dst`) |
| `--proc_path VALUE` | Specify procfs mount path to enable mounting (default: `/proc`). Implicitly sets `mount_proc: true` |
| `--proc_rw` | Mount procfs as read-write (default: read-only) |
| `--disable_proc` | Do not mount procfs (implicitly sets `mount_proc: false`) |

## User / Group

| Option | Description |
|--------|-------------|
| `--user VALUE` / `-u VALUE` | Username/UID inside the jail (default: current UID). The `inside:outside:count` format is also supported. Can be specified multiple times |
| `--group VALUE` / `-g VALUE` | Group name/GID inside the jail (default: current GID). The `inside:outside:count` format is also supported. Can be specified multiple times |
| `--uid_mapping VALUE` / `-U VALUE` | Custom UID mapping (`inside:outside:count`), uses newuidmap |
| `--gid_mapping VALUE` / `-G VALUE` | Custom GID mapping (`inside:outside:count`), uses newgidmap |

## Hostname / Environment Variables

| Option | Description |
|--------|-------------|
| `--hostname VALUE` / `-H VALUE` | UTS hostname (default: `NSJAIL`) |
| `--cwd VALUE` / `-D VALUE` | Working directory (default: `/`) |
| `--keep_env` / `-e` | Pass all environment variables |
| `--env VALUE` / `-E VALUE` | Add/set environment variables |

## Namespaces

| Option | Description |
|--------|-------------|
| `--disable_clone_newnet` / `-N` | Disable Network namespace |
| `--disable_clone_newuser` | Disable User namespace (requires root) |
| `--disable_clone_newns` | Disable Mount namespace |
| `--disable_clone_newpid` | Disable PID namespace |
| `--disable_clone_newipc` | Disable IPC namespace |
| `--disable_clone_newuts` | Disable UTS namespace |
| `--disable_clone_newcgroup` | Disable Cgroup namespace (for kernel < 4.6) |
| `--enable_clone_newtime` | Enable Time namespace (requires kernel >= 5.3) |

## Network

| Option | Description |
|--------|-------------|
| `--port VALUE` / `-p VALUE` | TCP port (enables LISTEN mode, default: 0) |
| `--bindhost VALUE` | Bind address (default: `::`) |
| `--max_conns VALUE` | Maximum concurrent connections (default: 0 = unlimited) |
| `--max_conns_per_ip VALUE` / `-i VALUE` | Maximum connections per IP (default: 0 = unlimited) |
| `--iface_no_lo` | Do not bring up the loopback interface |
| `--iface_own VALUE` | Move an interface into the jail |
| `--macvlan_iface VALUE` / `-I VALUE` | MACVLAN clone source interface |
| `--macvlan_vs_ip VALUE` | MACVLAN IP address |
| `--macvlan_vs_nm VALUE` | MACVLAN netmask |
| `--macvlan_vs_gw VALUE` | MACVLAN gateway |
| `--macvlan_vs_ma VALUE` | MACVLAN MAC address |
| `--macvlan_vs_mo VALUE` | MACVLAN mode (private/vepa/bridge/passthru, default: private) |
| `--user_net` | Enable user-mode networking (nstun backend) |

## Resource Limits

| Option | Description |
|--------|-------------|
| `--time_limit VALUE` / `-t VALUE` | Wall clock time limit (seconds, default: 600) |
| `--max_cpus VALUE` | CPU count limit (default: 0 = unlimited) |
| `--rlimit_as VALUE` | Virtual memory limit (MiB, default: 4096) |
| `--rlimit_core VALUE` | Core dump limit (MiB, default: 0) |
| `--rlimit_cpu VALUE` | CPU time limit (seconds, default: 600) |
| `--rlimit_fsize VALUE` | File size limit (MiB, default: 1) |
| `--rlimit_nofile VALUE` | Open file count limit (default: 32) |
| `--rlimit_nproc VALUE` | Process count limit (default: `soft`) |
| `--rlimit_stack VALUE` | Stack size limit (MiB, default: `soft`) |
| `--rlimit_memlock VALUE` | Locked memory limit (KB, default: `soft`) |
| `--rlimit_rtprio VALUE` | Real-time priority limit (default: `soft`) |
| `--rlimit_msgqueue VALUE` | Message queue limit (bytes, default: `soft`) |
| `--disable_rlimits` | Disable all resource limits |

Note: The help text in `cmdline.cc` uses "MB", but the actual unit conversion happens in `contain.cc`. The multiplication factor is `1024 * 1024` (= 1 MiB) for `--rlimit_as/core/fsize/stack`, and `1024` (= 1 KiB) for `--rlimit_memlock`.

In addition to numeric values, the special strings `inf`, `soft`/`def`, and `hard`/`max` can be specified for rlimit values.
However, in the current implementation, using special strings for `--rlimit_as/core/fsize/stack/memlock` may produce unexpected results due to interaction with unit conversion. For strict `SOFT/HARD/INF` specification, using the `*_type` fields in the configuration file is recommended.

## Capabilities / Security

| Option | Description |
|--------|-------------|
| `--keep_caps` | Retain all capabilities |
| `--cap VALUE` | Retain a specific capability (e.g., `CAP_NET_ADMIN`) |
| `--disable_no_new_privs` | Do not set PR_SET_NO_NEW_PRIVS |
| `--seccomp_policy VALUE` / `-P VALUE` | Kafel seccomp policy file |
| `--seccomp_string VALUE` | Inline Kafel policy |
| `--seccomp_log` | Use SECCOMP_FILTER_FLAG_LOG |
| `--seccomp_unotify` | Use SECCOMP_RET_USER_NOTIF and trace accessed files and network sockets |
| `--seccomp_unotify_report VALUE` | File to write the seccomp_unotify report to |
| `--disable_tsc` | Disable rdtsc/rdtscp (x86/x86-64 only) |
| `--forward_signals` | Forward fatal signals to child processes |

## Cgroup v1

| Option | Description |
|--------|-------------|
| `--cgroup_mem_max VALUE` | Memory limit (bytes, default: 0 = disabled) |
| `--cgroup_mem_memsw_max VALUE` | Memory+swap limit (bytes, default: 0 = disabled) |
| `--cgroup_mem_swap_max VALUE` | Swap limit (bytes, default: -1 = disabled) |
| `--cgroup_mem_mount VALUE` | memory cgroup mount (default: `/sys/fs/cgroup/memory`) |
| `--cgroup_mem_parent VALUE` | memory parent cgroup (default: `NSJAIL`) |
| `--cgroup_pids_max VALUE` | PID limit (default: 0 = disabled) |
| `--cgroup_pids_mount VALUE` | pids cgroup mount (default: `/sys/fs/cgroup/pids`) |
| `--cgroup_pids_parent VALUE` | pids parent cgroup (default: `NSJAIL`) |
| `--cgroup_net_cls_classid VALUE` | Network classification class ID (default: 0 = disabled) |
| `--cgroup_net_cls_mount VALUE` | net_cls cgroup mount (default: `/sys/fs/cgroup/net_cls`) |
| `--cgroup_net_cls_parent VALUE` | net_cls parent cgroup (default: `NSJAIL`) |
| `--cgroup_cpu_ms_per_sec VALUE` | CPU ms/second (default: 0 = unlimited) |
| `--cgroup_cpu_mount VALUE` | cpu cgroup mount (default: `/sys/fs/cgroup/cpu`) |
| `--cgroup_cpu_parent VALUE` | cpu parent cgroup (default: `NSJAIL`) |

## Cgroup v2

| Option | Description |
|--------|-------------|
| `--cgroupv2_mount VALUE` | cgroup v2 mount point (default: `/sys/fs/cgroup`) |
| `--use_cgroupv2` | Explicitly use cgroup v2 |
| `--detect_cgroupv2` | Auto-detect cgroup v2 |

## Logging

| Option | Description |
|--------|-------------|
| `--log VALUE` / `-l VALUE` | Log file path |
| `--log_fd VALUE` / `-L VALUE` | Log fd (default: 2 = stderr) |
| `--verbose` / `-v` | DEBUG level output |
| `--quiet` / `-q` | Output WARNING and above only |
| `--really_quiet` / `-Q` | Output FATAL only |
| `--daemon` / `-d` | Daemonize |

## Miscellaneous

| Option | Description |
|--------|-------------|
| `--nice_level VALUE` | nice value (default: 19) |
| `--skip_setsid` | Do not call setsid() |
| `--silent` | Redirect stdio to /dev/null |
| `--stderr_to_null` | Redirect stderr to /dev/null |
| `--pass_fd VALUE` | Pass an fd into the jail |
| `--persona_addr_no_randomize` | Disable ASLR |
| `--persona_addr_compat_layout` | ADDR_COMPAT_LAYOUT personality |
| `--persona_mmap_page_zero` | MMAP_PAGE_ZERO personality |
| `--persona_read_implies_exec` | READ_IMPLIES_EXEC personality |
| `--persona_addr_limit_3gb` | ADDR_LIMIT_3GB personality |
| `--oom_score_adj VALUE` | OOM score adjustment for the sandbox (-1000 to 1000, default: not set) |
| `--experimental_mnt VALUE` | Mount API selection (`new`/`old`/`auto`, default: `old`) |
