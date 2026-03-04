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

## YAML Schema

Each YAML file has a top-level `tests` key containing a list of test cases:

```yaml
tests:
  - name: "test case name"
    requests:
      - input:
          runtime: node
          files:
            - name: index.js
              type: raw
              content: |
                console.log("hello");
            - name: large.txt
              type: fill
              size: 1048576
        output:
          status: 200
          body:
            compile:
              stdout: ""
              stderr: ""
              output: ""
              exit_code: 0
              status: "OK"
              signal: null
            run:
              stdout: "hello\n"
              stderr: ""
              output: "hello\n"
              exit_code: 0
              status: "OK"
              signal: null
            error: ""
```

### File Types

- `raw` (default): File content is provided inline via the `content` field.
- `fill`: Generates a file of the specified `size` bytes, filled with the character `A`.

### Multiple Requests

A single test case can send multiple sequential requests by adding entries to the `requests` array.
