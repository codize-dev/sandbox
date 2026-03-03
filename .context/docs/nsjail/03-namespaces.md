# Linux Namespaces

nsjail supports 8 types of Linux namespaces. All namespaces except `clone_newtime` are enabled by default.

## Namespace List

| Namespace | Proto Field | Default | Disable CLI | Kernel Flag | Required Kernel |
|--------------|----------------|---------|------------|-------------|------------|
| User | `clone_newuser` | `true` | `--disable_clone_newuser` | `CLONE_NEWUSER` | — |
| Mount | `clone_newns` | `true` | `--disable_clone_newns` | `CLONE_NEWNS` | — |
| PID | `clone_newpid` | `true` | `--disable_clone_newpid` | `CLONE_NEWPID` | — |
| Network | `clone_newnet` | `true` | `-N` / `--disable_clone_newnet` | `CLONE_NEWNET` | — |
| UTS | `clone_newuts` | `true` | `--disable_clone_newuts` | `CLONE_NEWUTS` | — |
| IPC | `clone_newipc` | `true` | `--disable_clone_newipc` | `CLONE_NEWIPC` | — |
| Cgroup | `clone_newcgroup` | `true` | `--disable_clone_newcgroup` | `CLONE_NEWCGROUP` | 4.6 or above |
| Time | `clone_newtime` | `false` | `--enable_clone_newtime` | `CLONE_NEWTIME` | 5.3 or above |

## User Namespace (`CLONE_NEWUSER`)

Provides UID/GID mapping and enables nsjail to run as an unprivileged user.

### Behavior

- When enabled: nsjail can operate as an unprivileged user
- When disabled (`--disable_clone_newuser`): root privileges are required
- Using unprivileged user namespaces requires `sysctl kernel.unprivileged_userns_clone = 1` on some distributions

### UID/GID Mapping

Defined by the `IdMap` message:

```proto
message IdMap {
    optional string inside_id = 1 [default = ""];   // empty = current uid/gid
    optional string outside_id = 2 [default = ""];  // empty = current uid/gid
    optional uint32 count = 3 [default = 1];
    optional bool use_newidmap = 4 [default = false]; // use newuidmap/newgidmap binaries
}
```

#### CLI Options

| CLI | Description | Protobuf |
|-----|------|---------|
| `-u VALUE` / `--user VALUE` | UID mapping without `newuidmap` (format: `inside:outside:count` or simply `uid`) | `uidmap` |
| `-g VALUE` / `--group VALUE` | GID mapping without `newgidmap` | `gidmap` |
| `-U VALUE` / `--uid_mapping VALUE` | UID mapping using `/usr/bin/newuidmap` (setuid binary) | `uidmap` (use_newidmap: true) |
| `-G VALUE` / `--gid_mapping VALUE` | GID mapping using `/usr/bin/newgidmap` | `gidmap` (use_newidmap: true) |

#### How Mapping Works

- Multiple mappings can be specified
- The first UID/GID map entry is used as the primary ID for `setresuid`/`setresgid`
- Subsequent entries are set via `setgroups` (only when `CLONE_NEWUSER` is not used)
- The paths for `newuidmap`/`newgidmap` can be configured at compile time via the `NEWUIDMAP_PATH`/`NEWGIDMAP_PATH` macros
- UID/GID mappings are written to `/proc/PID/uid_map` and `/proc/PID/gid_map` from the parent process
- `"deny"` is written to `/proc/PID/setgroups` first (only when all three of the following conditions are met: `clone_newuser == true`, effective UID is not root, and a GID mapping not using `newidmap` exists)

#### Processing Inside the Child Process

1. `prctl(PR_SET_SECUREBITS, SECBIT_KEEP_CAPS | SECBIT_NO_SETUID_FIXUP)` — retain capabilities when uid/gid changes
2. `setresgid()` — set primary GID
3. `setgroups()` — set supplementary GIDs
4. `setresuid()` — set primary UID
5. `prctl(PR_SET_SECUREBITS, 0)` — reset securebits

## Mount Namespace (`CLONE_NEWNS`)

Provides an isolated filesystem view to processes inside the jail.

- Uses `pivot_root()` by default to change the root filesystem
- Specifying `--no_pivotroot` falls back to `mount(MS_MOVE)` + `chroot()` (useful when `pivot_root` is unavailable in initramfs, but escapable)
- Specifying `--chroot` makes that directory the new root
- By default, chroot is mounted read-only; `--rw` mounts it read-write

See [04-filesystem.md](04-filesystem.md) for details.

## PID Namespace (`CLONE_NEWPID`)

Provides an independent PID tree to processes inside the jail, starting from PID 1.

- Prevents process visibility across jails
- When enabled in EXECVE mode, a dummy "ns-init" process acting as PID 1 is spawned
- This init process is configured with `SA_NOCLDWAIT | SA_NOCLDSTOP` to reap zombie processes
- Without PID 1, subsequent `fork()` calls fail with `ENOMEM`

## Network Namespace (`CLONE_NEWNET`)

Creates an independent network stack (interfaces, routing tables, iptables rules, sockets).

- No network access by default (no interfaces other than loopback)
- The loopback interface is brought up by default
- `iface_no_lo: true` prevents the loopback from being brought up
- Three methods to provide network access:
  1. MACVLAN clone
  2. `iface_own` (moving an existing interface)
  3. pasta userland networking

See [05-network.md](05-network.md) for details.

## UTS Namespace (`CLONE_NEWUTS`)

Isolates the hostname and NIS domain name.

- `--hostname VALUE` / `hostname` field: sets the hostname inside the jail (default: `"NSJAIL"`)
- Internally calls `sethostname()`

## IPC Namespace (`CLONE_NEWIPC`)

Isolates System V IPC objects (message queues, semaphores, shared memory segments) and POSIX message queues.

## Cgroup Namespace (`CLONE_NEWCGROUP`)

Provides an isolated view of the cgroup hierarchy.

- Requires kernel 4.6 or above
- Must be disabled with `--disable_clone_newcgroup` on kernels below 4.6

## Time Namespace (`CLONE_NEWTIME`)

Allows offsets for `CLOCK_MONOTONIC` and `CLOCK_BOOTTIME` clocks within the namespace.

- Requires kernel 5.3 or above (`CONFIG_TIME_NS` kernel configuration option)
- **Disabled by default** (`clone_newtime: false`)
- Enable with `--enable_clone_newtime`
- On Linux in general, offsets can be configured via `/proc/pid/timens_offsets`, but nsjail itself does not implement writing to this file
- The `CLONE_NEWTIME` flag is also applied in ONCE/RERUN/LISTEN modes. It may work in environments where `clone3()` is available, but a warning log is emitted in the implementation
- If `clone_newtime` is requested in an environment that does not support `clone3()`, it will fail in all modes except EXECVE (`-Me`)

## Namespace Creation Methods

### clone Method (ONCE, RERUN, LISTEN modes)

Three-stage fallback (`subproc.cc: cloneProc()`):

1. Attempt `clone3()` + `CLONE_CLEAR_SIGHAND` (Linux 5.5 or above)
2. If step 1 fails (regardless of error type): retry `clone3()` without `CLONE_CLEAR_SIGHAND`
3. If step 2 fails with `ENOSYS` (clone3 not supported): fall back to legacy `clone()` with a 128 KiB static stack and a `setjmp`/`longjmp` trick

All configured namespace flags are included in the clone flags.

### unshare Method (EXECVE Mode)

- Calls `unshare()` to isolate namespaces within the current process
- Does not spawn a new process with `clone()`

### Parent-Child Process Synchronization

- Uses a communication channel created by `socketpair(AF_UNIX, SOCK_STREAM)`
- After the parent process completes setup of network, cgroup, and UID/GID mappings, it sends the sync character `'D'` to unblock the child process

### Special Handling in EXECVE Mode

In EXECVE mode, no socketpair between parent and child is used. The child process (= the current process) directly calls `user::initNsFromParent()` and `cgroup::initNsFromParent()` / `cgroup2::initNsFromParent()` inside `newProc()`.
