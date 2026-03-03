# Network Isolation

nsjail uses Network Namespace (`CLONE_NEWNET`) to isolate the network. Enabled by default.

## Default Behavior

When `clone_newnet: true` (default), the jail process gets an empty network namespace with no interfaces. The loopback (`lo`) interface is brought up by default.

- `iface_no_lo: true` / `--iface_no_lo`: Prevents the loopback interface from being brought up
- `-N` / `--disable_clone_newnet`: Disables the network namespace (shares the host network stack)

## Methods for Providing Network Access

### 1. MACVLAN

Clones a physical interface as a MACVLAN virtual interface and places it inside the jail. Requires root privileges. Uses `libnl-route-3` (netlink).

#### Configuration

| Field | Default | CLI | Description |
|-----------|---------|-----|------|
| `macvlan_iface` | — | `-I IFACE` | Name of the interface to clone |
| `macvlan_vs_ip` | `"192.168.0.2"` | `--macvlan_vs_ip` | IP address of the MACVLAN interface |
| `macvlan_vs_nm` | `"255.255.255.0"` | `--macvlan_vs_nm` | Netmask |
| `macvlan_vs_gw` | `"192.168.0.1"` | `--macvlan_vs_gw` | Default gateway |
| `macvlan_vs_ma` | `""` (auto) | `--macvlan_vs_ma` | MAC address |
| `macvlan_vs_mo` | `"private"` | `--macvlan_vs_mo` | MACVLAN mode |

#### MACVLAN Modes

| Mode | Description |
|-------|------|
| `private` | Prohibits communication with other MACVLANs on the same physical interface |
| `vepa` | Routes traffic through an external switch |
| `bridge` | Bridges traffic between MACVLANs on the same physical interface |
| `passthru` | Uses the physical interface directly (only one MACVLAN allowed) |

#### Internal Implementation

- Allocates a MACVLAN link object with `rtnl_link_macvlan_alloc()`
- Sets the master interface, name ("vs"), and target PID namespace
- Sets the optional MAC address with `rtnl_link_set_addr`
- Sets the mode with `rtnl_link_macvlan_set_mode`
- The "vs" interface inside the jail has its IP/mask/gateway configured via `ioctl SIOCSIFADDR`, `SIOCSIFNETMASK`, `SIOCADDRT`

```bash
sudo nsjail --macvlan_iface eth0 \
  --macvlan_vs_ip 192.168.1.100 \
  --macvlan_vs_nm 255.255.255.0 \
  --macvlan_vs_gw 192.168.1.1 \
  -- /bin/bash
```

### 2. Interface Ownership Transfer (iface_own)

Moves an existing network interface into the jail's network namespace.

- `--iface_own IFACE` / `repeated string iface_own`
- Moves the interface using `rtnl_link_change()` by setting `ns_pid` to the child process's PID
- Multiple interfaces can be specified

### 3. pasta Userland Networking

[pasta](https://passt.top/) is an external binary that provides userspace networking. It can provide network access without root privileges.

#### Enabling

- CLI: `--use_pasta`
- Protobuf: `user_net { enable: true }`
- `clone_newnet: true` must be set

#### UserNet Message (Protobuf)

```proto
message UserNet {
    optional bool enable = 1 [default = false];       // Enable pasta
    optional string ip = 2 [default = "10.255.255.2"];   // IPv4 address
    optional string mask = 3 [default = "255.255.255.0"]; // IPv4 netmask
    optional string gw = 4 [default = "10.255.255.1"];   // IPv4 gateway
    optional string ip6 = 5 [default = "fc00::2"];       // IPv6 address
    optional string mask6 = 6 [default = "64"];          // IPv6 prefix length (defined only, not passed to pasta)
    optional string gw6 = 7 [default = "fc00::1"];       // IPv6 gateway
    optional string ns_iface = 8 [default = "eth0"];     // Interface name inside the namespace
    optional string tcp_ports = 9 [default = "none"];    // TCP port forwarding (pasta -t flag)
    optional string udp_ports = 10 [default = "none"];   // UDP port forwarding (pasta -u flag)
    optional bool enable_ip4_dhcp = 11 [default = false]; // Enable IPv4 DHCP
    optional bool enable_dns = 12 [default = false];     // Enable DNS (--dhcp-dns flag)
    optional string dns_forward = 13 [default = ""];     // DNS forwarding (--dns-forward)
    optional bool enable_tcp = 14 [default = true];      // TCP forwarding (false → --no-tcp)
    optional bool enable_udp = 15 [default = true];      // UDP forwarding (false → --no-udp)
    optional bool enable_icmp = 16 [default = true];     // ICMP forwarding (false → --no-icmp)
    optional bool no_map_gw = 17 [default = false];      // --no-map-gw pasta flag
    optional bool enable_ip6_dhcp = 18 [default = false]; // DHCPv6
    optional bool enable_ip6_ra = 19 [default = false];  // IPv6 Router Advertisement
}
```

#### Behavior Details

- pasta is an external binary launched per jail from the parent process via `fork()` + `execvp("pasta", ...)`
- If the pasta process exits unexpectedly, `SIGKILL` is sent to the corresponding jail process
- Flags are added in the following order (implementation order of `spawnPasta()` in `net.cc`):
  1. `--no-dhcp` (if `enable_ip4_dhcp: false`)
  2. `--no-dhcpv6` (if `enable_ip6_dhcp: false`)
  3. `--no-ra` (if `enable_ip6_ra: false`)
  4. `--config-net` (if both IPv4 DHCP and IPv6 DHCP/RA are disabled)
  5. `-f` (foreground) — always passed
  6. `-q` (quiet) — always passed
  7. `-t` / `-u` (TCP/UDP port forwarding)
  8. `--dhcp-dns` (if `enable_dns: true`)
  9. `--dns-forward` (if `dns_forward` is set)
  10. `--no-tcp` / `--no-udp` / `--no-icmp` (if each respective forwarding is disabled)
  11. `--no-map-gw` (if `no_map_gw: true`)
  12. `-a` / `-n` / `-g` (IP address, netmask, gateway — IPv4)
  13. `-a` / `-g` (IPv6 address and gateway; `mask6` is not passed)
  14. `-4` (passed if `ip` is empty and `enable_ip4_dhcp` is `false`, disabling IPv4)
  15. `-6` (passed if `ip6` is empty, `enable_ip6_dhcp` is `false`, and `enable_ip6_ra` is `false`, disabling IPv6)
  16. `-I` (interface name inside the namespace)

```bash
nsjail --user 1000 --group 1000 --use_pasta --chroot / -- /usr/bin/curl https://example.com
```

## Network in TCP Listener Mode

In LISTEN mode:

- An `AF_INET6` socket is created with `SO_REUSEADDR`, `O_NONBLOCK`, and `SOMAXCONN` backlog
- IPv4 addresses are automatically converted to `::ffff:IP` format
- `max_conns` limits the total simultaneous connections across all IPs (`0` = unlimited)
- `max_conns_per_ip` tracks and limits connections per IP
- stdin/stdout are connected to the TCP socket via a `poll()` + `splice()`-based pipe relay
