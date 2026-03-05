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
    arch: [amd64, arm64]
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

### Architecture Filter

The optional `arch` field restricts a test case to specific CPU architectures. When omitted, the test runs on all architectures. When specified, the test runs only on the listed architectures (matched against Go's `runtime.GOARCH`).

```yaml
tests:
  - name: "amd64-only syscall test"
    arch: [amd64]
    requests:
      - input:
          # ...
```

### File Types

- `raw` (default): File content is provided inline via the `content` field.
- `fill`: Generates a file of the specified `size` bytes, filled with the character `A`.

### Regex Matching

String fields (`stdout`, `stderr`, `output`) support regex matching via `/pattern/` syntax. When a value starts and ends with `/`, the inner content is treated as a Go regular expression and matched against the actual value using partial match (`regexp.MatchString`).

Use regex only when exact match is technically impossible (e.g., non-deterministic timing output, variable iteration numbers):

```yaml
run:
  stdout: ""
  stderr: "/No space left on device/"
  output: "/No space left on device/"
  exit_code: 1
  status: "OK"
  signal: null
```

Exact match is the default and preferred mode. All fields are required — never omit a field to skip its assertion. Use regex only when exact match is technically impossible (e.g., non-deterministic output that varies across kernel versions or runs).

### Multiple Requests

A single test case can send multiple sequential requests by adding entries to the `requests` array.
