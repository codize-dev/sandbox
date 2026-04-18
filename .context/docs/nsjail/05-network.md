# Network Isolation

nsjail uses Network Namespace (`CLONE_NEWNET`) to isolate the network. Enabled by default.

## Default Behavior

When `clone_newnet: true` (default), the jail process gets an empty network namespace with no interfaces. The loopback (`lo`) interface is brought up by default.

- `iface_no_lo: true` / `--iface_no_lo`: Prevents the loopback interface from being brought up
- `-N` / `--disable_clone_newnet`: Disables the network namespace (shares the host network stack)

## Methods for Providing Network Access

### 1. MACVLAN

Clones a physical interface as a MACVLAN virtual interface and places it inside the jail. Requires root privileges. Requires nsjail to be built with libnl3 support.

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

Moves an existing network interface into the jail's network namespace. Requires nsjail to be built with libnl3 support.

- `--iface_own IFACE` / `repeated string iface_own`
- Moves the interface using `rtnl_link_change()` by setting `ns_pid` to the child process's PID
- Multiple interfaces can be specified

### 3. User-Mode Networking (UserNet)

User-mode networking provides network access inside the jail without root privileges. Two backends are available: **NSTUN** (built-in TUN-based networking) and **pasta** (external userspace networking binary).

#### Enabling

- CLI: `--user_net` (enables the NSTUN backend by default)
- Protobuf: `user_net { }` (the presence of the `user_net` block enables user networking, detected via `has_user_net()`)
- `clone_newnet: true` must be set

To use the pasta backend instead of NSTUN, set `backend: PASTA` in the config:

```protobuf
user_net {
    backend: PASTA
    pasta {
        # pasta-specific options here
    }
}
```

#### UserNet Message (Protobuf)

```proto
message UserNet {
    enum Backend {
        NSTUN = 0;   // Built-in TUN-based networking (default)
        PASTA = 1;   // External pasta binary
    }
    optional Backend backend = 1 [default = NSTUN];

    message Pasta {
        optional bool nat = 1 [default = true];              // Master NAT switch (see below)
        optional bool enable_tcp = 2 [default = true];       // TCP forwarding (false -> --no-tcp)
        optional bool enable_udp = 3 [default = true];       // UDP forwarding (false -> --no-udp)
        optional bool enable_icmp = 4 [default = true];      // ICMP forwarding (false -> --no-icmp)

        optional bool ip4_enabled = 5 [default = true];      // IPv4 enabled (false -> -6, IPv6-only)
        optional string mask4 = 6 [default = "255.255.255.0"]; // IPv4 netmask
        optional bool enable_ip4_dhcp = 7 [default = false]; // IPv4 DHCP (false -> --no-dhcp)

        optional bool ip6_enabled = 8 [default = true];      // IPv6 enabled (false -> -4, IPv4-only)
        optional string mask6 = 9 [default = "64"];           // IPv6 prefix length
        optional bool enable_ip6_dhcp = 10 [default = false]; // DHCPv6 (false -> --no-dhcpv6)
        optional bool enable_ip6_ra = 11 [default = false];  // IPv6 Router Advertisement (false -> --no-ra)

        optional bool enable_dns = 12 [default = false];     // Announce DNS via DHCP/NDP (--dhcp-dns)
        optional string dns_forward = 13 [default = ""];     // DNS forwarding (--dns-forward)
        optional bool map_gw = 14 [default = true];          // Map gateway to host (false -> --no-map-gw)

        optional string tcp_map_in = 15 [default = "none"];  // TCP port forwarding host->ns (-t)
        optional string udp_map_in = 16 [default = "none"];  // UDP port forwarding host->ns (-u)
        optional string tcp_map_out = 17 [default = "none"]; // TCP port forwarding ns->host (-T)
        optional string udp_map_out = 18 [default = "none"]; // UDP port forwarding ns->host (-U)
    }
    optional Pasta pasta = 2;

    message NstunRule {
        enum Direction {
            GUEST_TO_HOST = 0;
            HOST_TO_GUEST = 1;
        }
        enum Action {
            DROP = 0;          // Block silently
            REJECT = 1;        // Block and send ICMP unreachable / TCP RST
            ALLOW = 2;         // Explicitly allow
            REDIRECT = 3;      // Redirect to a different IP/port
            ENCAP_SOCKS5 = 4;  // Tunnel through a SOCKS5 proxy
            ENCAP_CONNECT = 5; // Tunnel through an HTTP CONNECT proxy
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

        /* Match fields */
        optional string src_ip = 3;       // Source IP or CIDR (e.g. "10.0.0.0/8")
        optional uint32 sport = 4;        // Source port (start of range)
        optional uint32 sport_end = 5;    // Source port range end
        optional string dst_ip = 6;       // Destination IP or CIDR
        optional uint32 dport = 7;        // Dest port (start of range; also listen port for HOST_TO_GUEST)
        optional uint32 dport_end = 8;    // Dest port range end

        /* Redirect/encap fields */
        optional string redirect_ip = 9;  // Target IP for REDIRECT/ENCAP
        optional uint32 redirect_port = 10; // Target port for REDIRECT/ENCAP
    }

    repeated NstunRule rule4 = 3;  // IPv4 firewall/NAT rules
    repeated NstunRule rule6 = 12; // IPv6 firewall/NAT rules

    optional string ip4 = 4 [default = "10.255.255.2"];  // IPv4 address for the container
    optional string gw4 = 5 [default = "10.255.255.1"];  // IPv4 gateway
    optional string ip6 = 6 [default = "fc00::2"];       // IPv6 address for the container
    optional string gw6 = 7 [default = "fc00::1"];       // IPv6 gateway
    optional string ns_iface = 8 [default = "eth0"];     // Interface name inside the namespace
}
```

#### NSTUN Backend

NSTUN is the built-in TUN-based networking backend. It creates a TUN device inside the jail's network namespace and runs a userspace network stack in a thread on the parent process.

##### How It Works

1. The child process creates a TUN device (`/dev/net/tun`) with `IFF_TUN | IFF_NO_PI` flags
2. The TUN interface is configured inside the namespace with the specified IPv4 address (point-to-point link, /32 netmask) and a default route. When `ip6` is configured, the interface also gets an IPv6 address with a /128 prefix and an IPv6 default route.
3. The TUN file descriptor is passed to the parent process via a Unix domain socket (`sendmsg`/`recvmsg` with `SCM_RIGHTS`)
4. The parent spawns a dedicated thread running an `epoll`-based network loop that reads L3 packets from the TUN device and forwards them to/from the host network stack

##### Policy Rules (NstunRule)

NSTUN supports fine-grained policy rules via `rule4` (IPv4) and `rule6` (IPv6) repeated fields. Rules are evaluated in order; the first matching rule wins. If no rule matches, traffic is allowed by default.

Each rule specifies:
- **Direction**: `GUEST_TO_HOST` (outbound from jail) or `HOST_TO_GUEST` (inbound to jail)
- **Protocol**: `ANY`, `TCP`, `UDP`, or `ICMP`
- **Match criteria**: Source/destination IP (with CIDR mask), source/destination port (or port range)
- **Action**: What to do with matching traffic

Available actions:
| Action | Description |
|--------|------|
| `DROP` | Block silently |
| `REJECT` | Block and send ICMP unreachable / TCP RST |
| `ALLOW` | Explicitly allow |
| `REDIRECT` | Redirect to a different IP/port |
| `ENCAP_SOCKS5` | Tunnel TCP/UDP through a SOCKS5 proxy |
| `ENCAP_CONNECT` | Tunnel TCP through an HTTP CONNECT proxy |

`ENCAP_SOCKS5` and `ENCAP_CONNECT` are not supported for ICMP protocol.

For `HOST_TO_GUEST` rules with `REDIRECT` action, nstun creates listener sockets on the host for the specified `dport` (or port range) and redirects accepted connections into the jail. The `redirect_ip` and `redirect_port` fields specify the target address inside the jail.

```protobuf
user_net {
    # Allow outbound HTTP/HTTPS, block everything else
    rule4 {
        direction: GUEST_TO_HOST
        proto: TCP
        dport: 80
        action: ALLOW
    }
    rule4 {
        direction: GUEST_TO_HOST
        proto: TCP
        dport: 443
        action: ALLOW
    }
    rule4 {
        direction: GUEST_TO_HOST
        action: DROP
    }

    # Tunnel all outbound TCP through a SOCKS5 proxy
    rule4 {
        direction: GUEST_TO_HOST
        proto: TCP
        action: ENCAP_SOCKS5
        redirect_ip: "127.0.0.1"
        redirect_port: 1080
    }

    # Expose host port 8080 -> guest port 80
    rule4 {
        direction: HOST_TO_GUEST
        proto: TCP
        dport: 8080
        action: REDIRECT
        redirect_ip: "10.255.255.2"
        redirect_port: 80
    }
}
```

#### pasta Backend

[pasta](https://passt.top/) is an external binary that provides userspace networking.

##### Enabling

Set `backend: PASTA` in the `user_net` block and configure the `pasta` sub-message:

```protobuf
user_net {
    backend: PASTA
    pasta {
        nat: true
        enable_tcp: true
        enable_udp: true
    }
}
```

##### pasta Execution Strategy

pasta is launched per jail from the parent process via `fork()`. The child process uses a two-branch execution strategy:

1. **Embedded binary**: If pasta was compiled into the nsjail binary (via `PASTA_BIN_PATH`), `getPastaFd()` returns a valid file descriptor (created via `memfd_create()`), and the binary is executed via `execveat()` with `AT_EMPTY_PATH`. There is no fallback if this fails.
2. **External binary**: If no embedded binary exists (`getPastaFd()` returns an invalid fd), the `NSJAIL_PASTA_PATH` environment variable is checked. If set, the binary at that path is executed via `execvpe()`. Otherwise, `execvpe("pasta", ...)` is called to find pasta in `$PATH`.

If the pasta process exits unexpectedly, `SIGKILL` is sent to the corresponding jail process.

##### The `nat` Field

The `nat` field (default: `true`) is the master switch for pasta's NAT behavior:

- **When `nat: true`** (default): Full NAT mode. DHCP/addressing/forwarding flags are passed, IP addresses are configured, and port mapping is active. Most pasta flags (DHCP, RA, DNS, protocol enables, IP addresses, interface name) are only emitted within this mode.
- **When `nat: false`**: Minimal mode. The `--splice-only` flag is passed to pasta, and only port mapping flags (`-t`/`-u`/`-T`/`-U`) apply. No addressing, DHCP, or protocol forwarding flags are emitted.

##### pasta Flag Order

Flags are added in the following order (implementation order of `pastaProcess()` in `net.cc`):

1. `-f` (foreground) -- always passed
2. `-q` (quiet) -- always passed
3. **If `nat: true`:**
   1. `--no-dhcp` (if `enable_ip4_dhcp: false`)
   2. `--no-dhcpv6` (if `enable_ip6_dhcp: false`)
   3. `--no-ra` (if `enable_ip6_ra: false`)
   4. `--config-net` (if both IPv4 DHCP and IPv6 DHCP are disabled)
   5. `--dhcp-dns` (if `enable_dns: true`)
   6. `--dns-forward VALUE` (if `dns_forward` is set)
   7. `--no-tcp` (if `enable_tcp: false`)
   8. `--no-udp` (if `enable_udp: false`)
   9. `--no-icmp` (if `enable_icmp: false`)
   10. `--no-map-gw` (if `map_gw: false`)
   11. `-a` / `-n` / `-g` (IPv4 address, netmask, gateway -- from `ip4`, `mask4`, `gw4`)
   12. `-a` / `-g` (IPv6 address and gateway -- from `ip6`, `gw6`; `mask6` is not passed to pasta)
   13. `-6` (passed if `ip4_enabled: false`, disabling IPv4 -- IPv6-only mode)
   14. `-4` (passed if `ip6_enabled: false`, disabling IPv6 -- IPv4-only mode)
   15. `-I` (interface name inside the namespace -- from `ns_iface`)
4. `-t` / `-u` (TCP/UDP inbound port mapping -- from `tcp_map_in`, `udp_map_in`; outside `nat` block)
5. `-T` / `-U` (TCP/UDP outbound port mapping -- from `tcp_map_out`, `udp_map_out`; outside `nat` block)
6. **If `nat: false`:** `--splice-only`
7. PID of the target process (final argument)

```bash
nsjail --user 1000 --group 1000 --user_net -C config.proto --chroot / -- /usr/bin/curl https://example.com
```

### 4. TrafficRule (IP-Level Traffic Policies)

`TrafficRule` provides IP-level traffic filtering inside the jail's network namespace using netlink FIB rules. Rules are applied from the child process after the namespace is created. Requires nsjail to be built with libnl3 support.

Unlike NSTUN rules (which operate in the parent's userspace network stack), `TrafficRule` entries are kernel-level routing policy rules applied via `rtnl_rule_add()`.

#### TrafficRule Message

```proto
message TrafficRule {
    enum TrafficAction {
        UNKNOWN_ACTION = 0;
        DROP = 1;     // Block silently (FR_ACT_BLACKHOLE)
        REJECT = 2;   // Block and send ICMP unreachable (FR_ACT_UNREACHABLE)
        ALLOW = 3;    // Allow (FR_ACT_TO_TBL -> RT_TABLE_MAIN)
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

    optional string src_ip = 1;          // Source IP or CIDR
    optional string dst_ip = 2;          // Destination IP or CIDR
    optional string iif = 3;             // Input interface
    optional string oif = 4;             // Output interface
    optional Protocol proto = 5 [default = UNKNOWN_PROTO];
    optional uint32 sport = 6;           // Source port (or range start)
    optional uint32 dport = 7;           // Destination port (or range start)
    optional uint32 sport_end = 8;       // Source port range end
    optional uint32 dport_end = 9;       // Destination port range end
    optional TrafficAction action = 10 [default = DROP];
    optional IpFamily ip_family = 15 [default = IPV4];
}
repeated TrafficRule traffic_rule = 96;
```

#### Example

```protobuf
# Block all outbound traffic to 10.0.0.0/8
traffic_rule {
    dst_ip: "10.0.0.0/8"
    action: DROP
    ip_family: IPV4
}
```

## Conditional Compilation Note

MACVLAN, interface ownership transfer (`iface_own`), and `TrafficRule` features require nsjail to be built with libnl3 support (`HAVE_LIBNL3`). If these features are requested but libnl3 is not available, nsjail will log an error and fail.

## Network in TCP Listener Mode

In LISTEN mode:

- An `AF_INET6` socket is created with `SO_REUSEADDR`, `O_NONBLOCK`, and `SOMAXCONN` backlog
- IPv4 addresses are automatically converted to `::ffff:IP` format
- `max_conns` limits the total simultaneous connections across all IPs (`0` = unlimited)
- `max_conns_per_ip` tracks and limits connections per IP
- stdin/stdout are connected to the TCP socket via a `poll()` + `splice()`-based pipe relay
