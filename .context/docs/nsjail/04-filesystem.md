# Filesystem Isolation

nsjail uses mount namespaces to isolate the filesystem. It supports both the mount(2) legacy API and the newer fsopen/fsmount API.

## Mount Point Configuration

### MountPt Message (Protobuf)

```proto
message MountPt {
    optional string src = 1;            // Source path (empty for pseudo filesystems such as tmpfs/proc)
    optional string prefix_src_env = 2; // Use the value of an environment variable as a prefix for src
    optional bytes src_content = 3;     // Write byte content to a temporary file and bind mount it
    required string dst = 4;            // Destination path inside the jail
    optional string prefix_dst_env = 5; // Use the value of an environment variable as a prefix for dst
    optional string fstype = 6;         // Filesystem type (e.g., "proc", "tmpfs")
    optional string options = 7;        // Mount options (e.g., "size=5000000")
    optional bool is_bind = 8;          // Whether this is a bind mount
    optional bool rw = 9;              // Read-write (default: read-only)
    optional bool is_dir = 10;          // Whether the target is a directory (auto-detected if not set)
    optional bool mandatory = 11;       // Whether a mount failure should be treated as an error (default: true)
    optional bool is_symlink = 12;      // Create a symbolic link instead of mounting
    optional bool nosuid = 13;          // MS_NOSUID
    optional bool nodev = 14;           // MS_NODEV
    optional bool noexec = 15;          // MS_NOEXEC
}
```

### CLI Shortcuts

| CLI Flag | Description | Format |
|-----------|------|------|
| `-R SRC[:DST]` / `--bindmount_ro` | Read-only bind mount | `src` or `src:dst` |
| `-B SRC[:DST[:OPTIONS]]` / `--bindmount` | Read-write bind mount | `src` or `src:dst` or `src:dst:options` |
| `-T DST` / `--tmpfsmount` | tmpfs mount (default size 4 MiB) | `dst` |
| `-m SRC:DST:FSTYPE:OPTIONS` / `--mount` | Arbitrary mount | `src:dst:fstype:options` |
| `-s SRC:DST` / `--symlink` | Create symbolic link | `src:dst` |

### Mount Flags

For bind mounts, `MS_BIND | MS_REC | MS_PRIVATE` is used. Read-only is applied via a remount after the initial mount.

The `nosuid`, `nodev`, and `noexec` flags can be configured individually per mount point.

## /proc Configuration

| CLI | Protobuf Field | Description |
|-----|-------------------|------|
| `--proc_path PATH` | `mount_proc: true` (set implicitly) | Specify the mount point for procfs and mount it (default: `/proc`). Specifying this flag automatically sets `mount_proc` to `true` |
| `--proc_rw` | CLI-only (no protobuf equivalent) | Mount procfs as read-write (default: read-only). In config files, use `rw: true` on the proc mount entry |
| `--disable_proc` | `mount_proc: false` (set implicitly) | Do not mount procfs |

Note: There is no CLI flag called `--mount_proc`. In Protobuf configuration files, `mount_proc: true` can be specified directly. In CLI usage, it is implicitly enabled when `--proc_path` is used.

Difference in default behavior between CLI and configuration file: In CLI mode, `proc_path` is pre-set to `/proc` by default, so `/proc` is mounted unless `--disable_proc` is explicitly specified. In configuration files, the default for `mount_proc` is `false`, so `mount_proc: true` must be explicitly specified.

## src_content Field

The `src_content` field writes the specified byte content to a temporary file and bind mounts it into the jail. This is useful for injecting content directly from a configuration file (e.g., `/etc/resolv.conf`).

## Environment Variable Prefixes

The `prefix_src_env` and `prefix_dst_env` fields allow the value of an environment variable to be used as a prefix for a path.

Example: Combining `prefix_src_env: "HOME"` with `src: "/Documents"` results in `$HOME/Documents` as the source path.

## chroot Configuration

| CLI | Description |
|-----|------|
| `--chroot PATH` / `-c PATH` | Root directory for the jail (default: none) (CLI-only, no protobuf equivalent) |
| `--rw` | Mount chroot as read-write (default: read-only) (CLI-only, no protobuf equivalent) |
| `--no_pivotroot` | Use `mount(MS_MOVE)` + `chroot()` instead of `pivot_root` (protobuf field: `no_pivotroot`) |

## Mount Process Details

The mount process executed in `mnt::initCloneNs()`. Note that the step order differs between the new API path and the legacy API path:

**New API path:**

1. `chdir('/')` — Set current directory to root
2. `mount("/", "/", NULL, MS_REC | MS_PRIVATE)` — Make root private
3. Create a temporary working directory (tried in the following order):
   - `/run/user/UID/nsjail/root`
   - `/run/user/nsjail.UID.root`
   - `/tmp/nsjail.UID.root`
   - `$TMPDIR/nsjail.UID.root` (if the `TMPDIR` environment variable is set)
   - `/dev/shm/nsjail.UID.root`
   - Fallback: traverse directories under `/` to find a writable directory
   - Final fallback: `/tmp/nsjail.UID.root.<random>` (with a random suffix)
4. Mount tmpfs (16 MiB) on the working directory
5. Process all configured mount entries in order
6. If `is_root_rw` is false: remount the root directory as read-only (uses `mount_setattr` with `AT_RECURSIVE`, so the remount applies recursively to all submounts)
7. `pivot_root(destdir, destdir)` + `umount2("/", MNT_DETACH)`
   - Or if `no_pivotroot`: `chdir(destdir)` + `mount(".", "/", MS_MOVE)` + `chroot(".")`
8. Remount each mount point to apply read-only flags
9. `chdir(cwd)`

**Legacy API path:**

1. `chdir('/')` — Set current directory to root
2. Create a temporary working directory (same search order as above)
3. `mount("/", "/", NULL, MS_REC | MS_PRIVATE)` — Make root private
4. Mount tmpfs (16 MiB) on the working directory (a second tmpfs is always additionally mounted for `src_content`)
5. Process all configured mount entries in order
6. If `is_root_rw` is false: remount the root directory as read-only (only remounts the root itself, not submounts)
7. `pivot_root(destdir, destdir)` + `umount2("/", MNT_DETACH)`
   - Or if `no_pivotroot`: `chdir(destdir)` + `mount(".", "/", MS_MOVE)` + `chroot(".")`
8. Remount each mount point to apply read-only flags
9. `chdir(cwd)`

## Legacy Mount API (`mnt_legacy.cc`)

Uses the `mount(2)` system call. During remounts, existing flags are detected and preserved via `statvfs`:

- `MS_NOSUID`
- `MS_NODEV`
- `MS_NOEXEC`
- `MS_SYNCHRONOUS`
- `MS_MANDLOCK`
- `MS_NOATIME`
- `MS_NODIRATIME`
- `MS_RELATIME`
- `MS_NOSYMFOLLOW`

## New Mount API (`mnt_newapi.cc`)

Requires kernel 6.3 or later at runtime (detected via `uname()`). Additionally, the relevant definitions for the new mount API (`__NR_fsopen`, `mount_setattr`, etc.) must be available in headers at build time. Uses the following system calls:

| System Call | Purpose |
|--------------|------|
| `fsopen(fstype)` | Create a filesystem context fd |
| `fsconfig(fd, FSCONFIG_SET_STRING, "source", ...)` | Set the source path |
| `fsconfig(fd, FSCONFIG_SET_FLAG, ...)` | Set mount option flags |
| `fsconfig(fd, FSCONFIG_CMD_CREATE)` | Create the filesystem |
| `fsmount(fs_fd, ...)` | Create a detached mount fd |
| `open_tree(AT_FDCWD, src, OPEN_TREE_CLONE \| OPEN_TREE_CLOEXEC)` | Clone a bind mount (`AT_RECURSIVE` is added conditionally) |
| `mount_setattr(fd, "", AT_EMPTY_PATH, ...)` | Set RDONLY/NOSUID/NODEV/NOEXEC |
| `move_mount(src_fd, "", dst_fd, path, MOVE_MOUNT_F_EMPTY_PATH)` | Attach a mount |

Operations on the destination side are fd-relative (using the `root_fd` of destdir). However, source-side path resolution via `open_tree(AT_FDCWD, src, ...)` and `fsconfig(..., "source", ...)` remains, so operations are not entirely fd-relative.

The new API parses generic options from the mount options string (e.g., `ro`, `nosuid`, `noexec`) and applies them as mount attribute flags via `mount_setattr`. The legacy API does not perform this generic options parsing.

### Mount API Control

Controllable via the `--experimental_mnt` CLI option:

| Value | Behavior |
|----|------|
| `auto` | Automatically selected based on kernel version |
| `new` | Force use of the new API |
| `old` | Force use of the legacy API |

The default when `--experimental_mnt` is not specified is `old`.

## no_pivotroot Warning

The source code explicitly warns that `no_pivotroot` can be escaped if the process holds the relevant capabilities, because `mount(MS_MOVE)` + `chroot()` is reversible.
