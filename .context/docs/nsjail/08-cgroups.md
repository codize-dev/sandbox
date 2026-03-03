# Resource Control with Cgroups

nsjail supports both cgroup v1 and cgroup v2, providing resource control for memory, process count, CPU, and network classification.

## Selecting the Cgroup Version

| CLI | Protobuf field | Description |
|-----|----------------|-------------|
| `--use_cgroupv2` | `use_cgroupv2: true` | Explicitly use cgroup v2 |
| `--detect_cgroupv2` | `detect_cgroupv2: true` | Auto-detect: use v2 if cgroup v2 is mounted at `cgroupv2_mount` |
| (none) | (default) | Use cgroup v1 |

Auto-detection checks `f_type == CGROUP2_SUPER_MAGIC` via `statfs(cgroupv2_mount)`.

## Common Configuration Fields

The following resource limit fields are shared between cgroup v1 and v2:

| Field | Default | CLI | Unit | Description |
|-------|---------|-----|------|-------------|
| `cgroup_mem_max` | `0` (disabled) | `--cgroup_mem_max` | bytes | Memory limit |
| `cgroup_mem_memsw_max` | `0` (disabled) | `--cgroup_mem_memsw_max` | bytes | Memory + swap limit |
| `cgroup_mem_swap_max` | `-1` (disabled) | `--cgroup_mem_swap_max` | bytes | Swap limit |
| `cgroup_pids_max` | `0` (disabled) | `--cgroup_pids_max` | count | Process count limit |
| `cgroup_cpu_ms_per_sec` | `0` (unlimited) | `--cgroup_cpu_ms_per_sec` | milliseconds | CPU time per second |

`cgroup_mem_memsw_max` and `cgroup_mem_swap_max` cannot be specified simultaneously.

## Cgroup v1

### Controller List

| Controller | Config prefix | Default mount | Default parent |
|------------|---------------|---------------|----------------|
| memory | `cgroup_mem_*` | `/sys/fs/cgroup/memory` | `NSJAIL` |
| pids | `cgroup_pids_*` | `/sys/fs/cgroup/pids` | `NSJAIL` |
| net_cls | `cgroup_net_cls_*` | `/sys/fs/cgroup/net_cls` | `NSJAIL` |
| cpu | `cgroup_cpu_*` | `/sys/fs/cgroup/cpu` | `NSJAIL` |

### Path Structure

The cgroup path for each jail is created as `{mount}/{parent}/NSJAIL.{pid}`.

Example: `/sys/fs/cgroup/memory/NSJAIL/NSJAIL.12345`

### v1-Specific Configuration Fields

| Field | Default | CLI | Description |
|-------|---------|-----|-------------|
| `cgroup_mem_mount` | `/sys/fs/cgroup/memory` | `--cgroup_mem_mount` | Mount point for the memory controller |
| `cgroup_mem_parent` | `NSJAIL` | `--cgroup_mem_parent` | Parent cgroup directory for the memory controller (must exist beforehand) |
| `cgroup_pids_mount` | `/sys/fs/cgroup/pids` | `--cgroup_pids_mount` | Mount point for the pids controller |
| `cgroup_pids_parent` | `NSJAIL` | `--cgroup_pids_parent` | Parent cgroup directory for the pids controller |
| `cgroup_net_cls_classid` | `0` (disabled) | `--cgroup_net_cls_classid` | Class ID for network classification (hexadecimal value) |
| `cgroup_net_cls_mount` | `/sys/fs/cgroup/net_cls` | `--cgroup_net_cls_mount` | Mount point for the net_cls controller |
| `cgroup_net_cls_parent` | `NSJAIL` | `--cgroup_net_cls_parent` | Parent cgroup directory for the net_cls controller |
| `cgroup_cpu_mount` | `/sys/fs/cgroup/cpu` | `--cgroup_cpu_mount` | Mount point for the cpu controller |
| `cgroup_cpu_parent` | `NSJAIL` | `--cgroup_cpu_parent` | Parent cgroup directory for the cpu controller |

### memory Controller

- Writes `cgroup_mem_max` to `memory.limit_in_bytes`
- Writes `cgroup_mem_memsw_max` to `memory.memsw.limit_in_bytes` (when configured)
  - If `cgroup_mem_swap_max` is set: calculated as `memsw_max = mem_max + swap_max`
- Writes `0` to `memory.oom_control` to enable the OOM killer (so processes are killed rather than hanging)

### pids Controller

- Writes `cgroup_pids_max` to `pids.max`

### net_cls Controller

- Writes `cgroup_net_cls_classid` to `net_cls.classid`
- Can be used in combination with tc (traffic control) for network bandwidth control

### cpu Controller

Sets CFS (Completely Fair Scheduler) quota:

- Writes `1000000` (1 second) to `cpu.cfs_period_us`
- Writes `cgroup_cpu_ms_per_sec * 1000` to `cpu.cfs_quota_us`

Example: `cgroup_cpu_ms_per_sec = 800` → `cpu.cfs_quota_us = 800000` (800ms of CPU time per second)

### Process Assignment

Processes are assigned to the cgroup by writing the PID to the `tasks` file.

### Cleanup

`finishFromParent()`: After process termination, removes the cgroup directory with `rmdir`.

Note: In both v1 and v2, the conditions for creating and deleting a cgroup directory are inconsistent. If only `cgroup_mem_swap_max` is set (`cgroup_mem_max = 0`), the directory leaks. Similarly, if only `cgroup_mem_memsw_max` is set (`cgroup_mem_max = 0`), it also leaks (because `initNsFromParentMem` creates the cgroup by computing `swap_max = memsw_max - mem_max >= 0`, but the deletion condition in `finishFromParent` — `cgroup_mem_max != 0` — does not apply). In v2, the directory is also excluded from deletion when `cgroup_pids_max = 0` and `cgroup_cpu_ms_per_sec = 0`.

## Cgroup v2

### Overview

In cgroup v2 (unified hierarchy), all controllers share a single cgroup hierarchy.

- Basic support added in v2.9 (September 2021)
- Enhanced in v3.4 (October 2024)

### v2-Specific Configuration Fields

| Field | Default | CLI | Description |
|-------|---------|-----|-------------|
| `cgroupv2_mount` | `/sys/fs/cgroup` | `--cgroupv2_mount` | Root mount path for cgroup v2 |
| `use_cgroupv2` | `false` | `--use_cgroupv2` | Explicitly use cgroup v2 |
| `detect_cgroupv2` | `false` | `--detect_cgroupv2` | Auto-detect cgroup v2 |

### Path Structure

The cgroup path for each jail is created as `{cgroupv2_mount}/NSJAIL.{pid}`.

Example: `/sys/fs/cgroup/NSJAIL.12345`

### Supported Controllers

| Controller | Control file | Value format |
|------------|--------------|--------------|
| memory | `memory.max` | bytes |
| memory (swap) | `memory.swap.max` | bytes |
| pids | `pids.max` | process count |
| cpu | `cpu.max` | `"<quota_us> 1000000"` |

Note: The cgroup v1 settings `cgroup_net_cls_classid` / `cgroup_net_cls_mount` / `cgroup_net_cls_parent` are not processed in cgroup v2 and are silently ignored. Since cgroup v2 does not have a net_cls controller, use cgroup v1 if network classification is required.

### Swap Calculation Differences Between v1 and v2

The handling of `cgroup_mem_swap_max` and `cgroup_mem_memsw_max` differs between cgroup v1 and v2:

- **cgroup v1**: `memory.memsw.limit_in_bytes = mem_max + swap_max` (writes the combined value)
- **cgroup v2**: `memory.swap.max = memsw_max - mem_max` (writes the difference)

When `cgroup_mem_memsw_max` is specified:
- v1: writes it directly to `memory.memsw.limit_in_bytes`
- v2: writes `cgroup_mem_memsw_max - cgroup_mem_max` to `memory.swap.max`

### CPU Throttling

Format of `cpu.max`: `"<quota_us> <period_us>"`

Example: `"800000 1000000"` → 800ms of CPU time per second

### Handling the Internal Process Rule

To handle the cgroup v2 "no internal processes" rule (when a cgroup has processes, controllers cannot be enabled in its subtree):

1. Read `cgroup.subtree_control` on the root cgroup to check whether the required controllers are already enabled
2. If all are enabled, do nothing (skip steps 3–5)
3. Enable the missing controllers with `+memory`, `+pids`, `+cpu`
4. If `EBUSY` is returned (because the nsjail process itself is in the root cgroup), move nsjail itself into the `NSJAIL_SELF.{nsjail-pid}` child cgroup (`moveSelfIntoChildCgroup`)
5. Enable subtree control again

Note: No code exists to delete the `NSJAIL_SELF.{pid}` child cgroup. This cgroup directory may leak after nsjail exits.

### Process Assignment

Processes are assigned to the cgroup by writing the PID to the `cgroup.procs` file.

### Directory Permissions

cgroup directories are created with `0700`.

### Usage Inside Docker

When using cgroup v2 inside a Docker container:

- nsjail must be the root process of the container
- Docker's `--cgroupns=host` flag is required

### Usage Example

```bash
nsjail -Mo --cgroup_mem_max 6000000000 --detect_cgroupv2 --chroot / -- /bin/bash
```
