---
name: add-runtime
description: >
  Add a new programming language runtime to the sandbox. Use this skill when the user
  asks to add a new language, new runtime, or support for a new programming language
  (e.g., "Add Rust support", "add Python runtime", "support a new language").
  Also trigger when the user mentions adding a runtime, language support, or interpreter/compiler
  to the sandbox execution engine. This skill covers the full end-to-end process:
  runtime implementation, Docker setup, resource limit design, testing, and documentation.
---

# Add Runtime Skill

This skill guides the complete process of adding a new programming language runtime to the sandbox. It covers every touchpoint in the codebase and includes resource limit design rationale.

Before starting, gather the following information from the user:

1. **Language name and version** (e.g., "Rust 1.82.0")
2. **Interpreted or compiled?** — Interpreted runtimes (like Ruby, Python) run source directly. Compiled runtimes (like Go) need a compilation step before execution.
3. **mise package name** — Check with `mise ls-remote <tool>` or the [mise registry](https://mise.jdx.dev/registry.html) to confirm the tool name and available versions.

---

## Step 1: Determine Resource Limits

Resource limits are a security boundary. Choose values based on the runtime's characteristics, not arbitrary defaults. Below are the existing limits for reference, followed by the decision framework.

### Existing Limits Reference

| Limit | Node.js | Ruby | Python | Go (run) | Go (compile) | Rust (run) | Rust (compile) | Bash |
|-------|---------|------|--------|----------|--------------|------------|----------------|------|
| AS (MiB) | 4096 | 1024 | 1024 | 1024 | 4096 | 1024 | 4096 | 512 |
| Fsize (MiB) | 64 | 64 | 64 | 64 | 64 | 64 | 64 | 64 |
| Nofile | 64 | 64 | 64 | 64 | 256 | 64 | 256 | 64 |
| Nproc | soft | soft | soft | soft | soft | soft | soft | soft |
| PidsMax | 64 | 32 | 32 | 64 | 128 | 64 | 128 | 32 |
| MemMax (bytes) | 268435456 | 268435456 | 268435456 | 268435456 | 268435456 | 268435456 | 268435456 | 268435456 |
| MemSwapMax | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| CpuMsPerSec | 900 | 900 | 900 | 900 | 900 | 900 | 900 | 900 |

### Decision Framework

These values are consistent across all runtimes and should be kept as-is unless there is a strong, documented reason to deviate:

- **Fsize**: 64 MiB (sufficient for typical output files)
- **Nproc**: "soft" (inherits system soft limit; per-sandbox limiting uses cgroup_pids_max)
- **MemMax**: 268435456 (256 MiB physical memory; prevents host OOM)
- **MemSwapMax**: 0 (swap disabled for strict memory enforcement)
- **CpuMsPerSec**: 900 (90% of one core)

These values require runtime-specific analysis:

#### AS (Virtual Address Space, in MiB)

The AS limit controls the maximum virtual address space. It does NOT directly limit physical memory (that's MemMax). Unmapped VAS pages consume no RAM, so a higher AS is safe when MemMax constrains physical usage.

| Category | Value | When to use |
|----------|-------|-------------|
| 4096 | High VAS | Runtimes with JIT/mmap-heavy memory management (V8/Node.js, JVM, .NET CLR). Also needed for compiler toolchains (Go compiler + linker). |
| 1024 | Standard | Traditional interpreters (CPython, CRuby, Perl) and compiled binaries. |
| 512 | Minimal | Lightweight runtimes (Bash, shell utilities). Bash needs ~2.8× output size for command substitution. |

**How to decide**: Run the runtime with a simple program and check its VAS usage (`/proc/<pid>/status` → VmSize). Then add 2-4× headroom. If the runtime uses mmap-based garbage collection (like V8 or JVM), use 4096.

#### Nofile (Open File Descriptors)

| Category | Value | When to use |
|----------|-------|-------------|
| 64 | Standard | Most runtimes. Covers stdin/stdout/stderr (3) + nsjail internal fds (~5) + runtime engine fds. |
| 256 | High | Compilation steps that open many source/object files concurrently (e.g., `go build`). |

#### PidsMax (Per-cgroup Process + Thread Limit)

| Category | Value | When to use |
|----------|-------|-------------|
| 32 | Low concurrency | Single-threaded interpreters (Ruby, Python, Bash). Limits fork bombs. |
| 64 | Moderate concurrency | Runtimes with built-in concurrency (Node.js worker_threads, Go goroutines). |
| 128 | High concurrency | Compilation steps with heavy parallelism (Go compiler). |

**How to decide**: Run a "hello world" program and check the peak thread/process count. Then add headroom for user-created threads. Interpreters that rarely spawn threads → 32. Runtimes with native concurrency support → 64.

### For Compiled Runtimes

Compiled runtimes need TWO sets of limits: one for compilation (CompileLimits) and one for execution (Limits). Compilation typically needs:
- Higher AS (compiler toolchains are memory-hungry)
- Higher Nofile (many concurrent source file reads)
- Higher PidsMax (compiler parallelism)

---

## Step 2: Verify mise Installation

Before writing code, verify that the runtime installs correctly via mise on the target platform (Debian bookworm / glibc).

```bash
# Check available versions
mise ls-remote <tool> | tail -20

# Check if special settings are needed (like ruby.compile=false)
# Search mise docs for the tool
```

Key considerations:
- The Dockerfile uses a **glibc-linked mise binary** (not musl). This is because mise's libc detection affects which precompiled binaries it downloads. A musl-linked mise would download musl Python/Ruby/etc., which won't run on Debian (glibc).
- Some runtimes need special mise settings (e.g., `ruby.compile=false` to use prebuilt binaries instead of compiling from source).
- Check if the runtime binary path follows the standard pattern: `/mise/installs/<tool>/<version>/bin/<executable>`.

---

## Step 3: Implementation Checklist

The following files need changes. Items marked with ★ apply only to compiled runtimes.

### 3.1 `internal/sandbox/runtime.go`

#### 3.1a Add Runtime Constant

Add the constant to the `const` block. Insert before `RuntimeBash` (Bash is always last by convention):

```go
const (
    RuntimeNode   RuntimeName = "node"
    RuntimeRuby   RuntimeName = "ruby"
    RuntimeGo     RuntimeName = "go"
    RuntimePython RuntimeName = "python"
    // ← Insert new runtime here (before RuntimeBash)
    RuntimeBash   RuntimeName = "bash"
)
```

#### 3.1b Register in Runtimes Map

Add the entry to the `runtimes` map variable, matching the constant order:

```go
var runtimes = map[RuntimeName]Runtime{
    RuntimeNode:   nodeRuntime{},
    RuntimeRuby:   rubyRuntime{},
    RuntimeGo:     goRuntime{},
    RuntimePython: pythonRuntime{},
    // ← Insert new runtime here (before RuntimeBash)
    RuntimeBash:   bashRuntime{},
}
```

#### 3.1c Implement Runtime Struct

Insert the implementation between the preceding runtime's section and the next one. Follow the `// --- Name ---` section header convention.

**Interpreted runtime template** (use Ruby/Python as reference):

```go
// --- LanguageName ---

type langRuntime struct{}

func (langRuntime) Name() RuntimeName { return RuntimeLang }

func (langRuntime) Command(entryFile string) []string {
    return []string{"/mise/installs/<tool>/<version>/bin/<executable>", entryFile}
}

func (langRuntime) BindMounts() []BindMount {
    return []BindMount{{Src: "/mise/installs/<tool>/<version>", Dst: "/mise/installs/<tool>/<version>"}}
}

func (langRuntime) Env() []string {
    return []string{"PATH=/mise/installs/<tool>/<version>/bin:/usr/bin:/bin"}
}

// Limits returns resource limits for <Language> execution.
// Rlimits:
//   - AS <value> MiB: <rationale>.
//   - Fsize 64 MiB: sufficient for typical output files.
//   - Nofile <value>: <rationale>.
//   - Nproc soft: inherits the system soft limit; per-sandbox process limiting is handled by cgroup_pids_max.
//
// Cgroups:
//   - PidsMax <value>: per-cgroup task limit (processes + threads); limits fork bombs and runaway thread creation.
//   - MemMax 268435456 (256 MiB): physical memory limit; prevents sandbox OOM from affecting the host.
//   - MemSwapMax 0: swap disabled to enforce strict memory limits.
//   - CpuMsPerSec 900: throttle CPU to 900 ms per second (90% of one core).
func (langRuntime) Limits() Limits {
    return Limits{
        Rlimits: Rlimits{
            AS:     "<value>",
            Fsize:  "64",
            Nofile: "<value>",
            Nproc:  "soft",
        },
        Cgroups: Cgroups{
            PidsMax:     "<value>",
            MemMax:      "268435456",
            MemSwapMax:  "0",
            CpuMsPerSec: "900",
        },
    }
}

func (langRuntime) RestrictedFiles() []string { return nil }
```

**★ Compiled runtime**: additionally implement the `CompiledRuntime` interface methods (`CompileCommand`, `CompileBindMounts`, `CompileEnv`, `CompileLimits`) following the Go runtime as reference. Add `var _ CompiledRuntime = langRuntime{}` type assertion after the existing one for Go.

#### 3.1d Default Files (if needed)

If the runtime requires default files (like Go's `go.mod` and `go.sum`), create them under `internal/sandbox/defaults/<runtime>/`. Files with `.tmpl` suffix have the suffix stripped at runtime. Most interpreted runtimes need no default files.

#### 3.1e Restricted Files (if needed)

If certain filenames must be rejected (like Go's `go.mod`, `go.sum`, `main`), return them from `RestrictedFiles()`. Most interpreted runtimes return `nil`.

### 3.2 `Dockerfile`

Add the runtime installation in the `base` stage, after the existing runtime installations:

```dockerfile
# <Tool>
ENV PATH="/mise/installs/<tool>/<version>/bin:$PATH"
RUN mise use -g <tool>@<version>
```

If the runtime needs special mise settings (like Ruby's `ruby.compile=false`), add them in the same `RUN` command.

**★ Compiled runtime**: may need additional setup like pre-building standard libraries or pre-downloading dependencies (see Go's pattern with `go build std` and `go mod download`).

### 3.3 `internal/sandbox/sandbox_test.go`

Add 4 test entries:

#### 3.3a `Test_LookupRuntime` — add valid runtime entry:
```go
{name: "<lang> is valid", runtime: Runtime<Lang>, wantErr: false},
```

#### 3.3b `Test<Lang>Runtime_Limits` — add new test function:
```go
func Test<Lang>Runtime_Limits(t *testing.T) {
    t.Parallel()
    rt := <lang>Runtime{}
    got := rt.Limits()
    assert.Equal(t, "<AS>", got.Rlimits.AS)
    assert.Equal(t, "64", got.Rlimits.Fsize)
    assert.Equal(t, "<Nofile>", got.Rlimits.Nofile)
    assert.Equal(t, "soft", got.Rlimits.Nproc)
    assert.Equal(t, "<PidsMax>", got.Cgroups.PidsMax)
    assert.Equal(t, "268435456", got.Cgroups.MemMax)
    assert.Equal(t, "0", got.Cgroups.MemSwapMax)
    assert.Equal(t, "900", got.Cgroups.CpuMsPerSec)
}
```

**★ Compiled runtime**: also test `CompileLimits()` in the same function (see `TestGoRuntime_Limits`).

#### 3.3c `Test_readDefaultFiles` — add sub-test:
```go
t.Run("<lang> has no defaults", func(t *testing.T) {
    t.Parallel()
    files, err := readDefaultFiles(Runtime<Lang>)
    assert.NoError(t, err)
    assert.Empty(t, files)
})
```

If the runtime HAS default files, assert their names and content instead.

#### 3.3d `TestRuntime_RestrictedFiles` — add sub-test:
```go
t.Run("<lang> has no restricted files", func(t *testing.T) {
    t.Parallel()
    rt, err := LookupRuntime(Runtime<Lang>)
    require.NoError(t, err)
    assert.Empty(t, rt.RestrictedFiles())
})
```

If the runtime HAS restricted files, use `assert.ElementsMatch` instead.

### 3.4 `e2e/tests/api/validation.yml`

Update the "unknown runtime" test case's expected error message to include the new runtime in alphabetical order:

```yaml
message: 'must be one of "bash", "go", "<new>", "node", "python", "ruby"'
```

Also verify the test case's `runtime` field is set to a truly unknown runtime (not one that was just added). Currently uses `"java"`.

### 3.5 `e2e/tests/runtime/<lang>.yml`

Create a new E2E test file. Include at minimum these test categories:

| Category | Purpose | Example |
|----------|---------|---------|
| hello world | Basic execution | `print("Hello, World!")` |
| stderr output | Stderr works | Write to stderr |
| stdout and stderr | Interleaved output | Both streams, verify `output` field |
| non-zero exit code | Exit code propagation | `sys.exit(1)` equivalent |
| stderr with non-zero exit | Error + exit code | Combined |
| multiple files | Multi-file support | Import from second file |
| syntax error | Parse failure | Broken syntax → stderr with error, exit 1 |
| unhandled exception | Runtime error | Uncaught exception → stderr with error, exit 1 |
| standard library usage | stdlib available | JSON, regex, math, etc. |
| language features | Core features | Classes, closures, data structures, etc. |

Use regex matching (`/pattern/`) for stderr/output when the exact output is non-deterministic (e.g., stack traces).

**★ Compiled runtime**: test cases should also verify the `compile` field in the response (stdout, stderr, exit_code, status for the compile step). Test compilation errors as well.

### 3.6 Documentation Updates

#### `CLAUDE.md` — 3 locations:
1. **Line 7** (Project Overview): Add language name to the supported runtimes list
2. **Line 90** (API docs): Add `"<lang>"` to the runtime enum list
3. **Line 102** (compile field docs): If interpreted, add to "non-compiled runtimes" list

#### `README.md` — 3 locations:
1. **Supported Runtimes table** (~line 35-41): Add row `| <Language> | \`<lang>\` |`
2. **API `runtime` parameter** (~line 118): Add `"<lang>"` to the enum list
3. **`compile` field description** (~line 137): If interpreted, add to interpreted runtimes list

### 3.7 `.claude/skills/add-runtime/SKILL.md`

Update the "Existing Limits Reference" table in Step 1 of this skill to include the new runtime's limits. This keeps the table accurate for the next runtime addition.

---

## Step 4: Verification

Run these checks in order. Fix any failures before proceeding to the next step.

### 4.1 Unit Tests
```bash
go test ./...
```

### 4.2 Lint
```bash
golangci-lint run
```

### 4.3 Docker Build + E2E Tests
```bash
docker compose down && docker compose up --build -d
go test -tags e2e ./e2e/...
```

If the Docker build fails:
- **mise install failure**: Check if special settings are needed (e.g., `ruby.compile=false`). Check if the mise binary's libc matches the base image (must be glibc for Debian).
- **Binary not found**: Verify the install path with `mise where <tool>@<version>` inside a running container.

---

## Quick Reference: Files to Modify

| # | File | Operation | Required for |
|---|------|-----------|-------------|
| 1 | `internal/sandbox/runtime.go` | Edit (constant, map, struct) | All |
| 2 | `internal/sandbox/defaults/<lang>/` | Create (if needed) | Compiled |
| 3 | `Dockerfile` | Edit (mise install) | All (except Bash) |
| 4 | `internal/sandbox/sandbox_test.go` | Edit (4 test additions) | All |
| 5 | `e2e/tests/runtime/<lang>.yml` | Create | All |
| 6 | `e2e/tests/api/validation.yml` | Edit (error message) | All |
| 7 | `CLAUDE.md` | Edit (3 locations) | All |
| 8 | `README.md` | Edit (3 locations) | All |
| 9 | `.claude/skills/add-runtime/SKILL.md` | Edit (limits table in Step 1) | All |

Files that do NOT need changes (they resolve runtimes dynamically):
- `internal/handler/handler.go` — uses `LookupRuntime()`
- `internal/sandbox/sandbox.go` — uses `CompiledRuntime` type assertion
- `internal/sandbox/execution.go` — generic execution engine
- `internal/sandbox/configs/nsjail.cfg` — static config, per-invocation overrides via CLI flags
- `internal/sandbox/configs/seccomp.kafel` — syscall policy, runtime-agnostic
