# E2E Test Guidelines

## Testing Philosophy

E2E tests must verify that **attack vectors are actually blocked**, not that security configurations are enabled.

What matters is whether the sandbox prevents the attack — not whether a particular setting is in place. Configuration-level assertions break easily on refactors and implementation changes, while attack-vector tests remain valid as long as the defense holds.

### Good

- **Network isolation**: Attempt an outbound connection from inside the sandbox and assert it fails.
- **Filesystem restriction**: Try to read a sensitive host path from inside the sandbox and assert it fails.
- **Resource limits**: Run a fork bomb or allocate excessive memory and assert the sandbox terminates appropriately.

### Bad

- **Network isolation**: Read `/proc/net/if_inet6` and assert no network interfaces exist.
- **Filesystem restriction**: Parse the mount table and assert a read-only flag is present.
- **Resource limits**: Read `/proc/self/limits` and assert rlimit values match expectations.
