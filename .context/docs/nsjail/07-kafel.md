# Kafel Policy Language Reference

[Kafel](https://github.com/google/kafel) (Kernel Application Function Execution Limiter) is a small, embeddable C library developed by Google that compiles a human-readable policy language into BPF code for seccomp-filter. It is included as a git submodule in nsjail.

Official documentation: [google.github.io/kafel](https://google.github.io/kafel/)

## Policy File Structure

A Kafel policy file consists of the following elements:

1. **Constant definitions** (`#define`)
2. **Policy definitions** (`POLICY name { ... }`)
3. **Default action** (`DEFAULT action`)
4. **Include directives** (`#include "file.policy"`)
5. **File-scope statements** (appended to an implicit top-level policy)

## Comments

Kafel supports two comment syntaxes:

- Line comment: `// comment`
- Block comment: `/* comment */`

Note: `#` is not a comment character. It is used as a preprocessor prefix for `#define` and `#include`.

## Numeric Literals

| Format | Example |
|--------|---------|
| Decimal | `42`, `-1` |
| Hexadecimal | `0xfa1` |
| Octal | `0777` |
| Binary | `0b10101` |

Negative decimal numbers are supported (e.g., `-1`). Note: negative values are lexically accepted but stored as unsigned 64-bit values via `strtoull` (two's complement representation). For example, `-1` becomes `0xFFFFFFFFFFFFFFFF` internally.

## Actions

| Kafel Keyword | seccomp-filter Return Value | Description |
|---------------|-----------------------------|-------------|
| `ALLOW` | `SECCOMP_RET_ALLOW` | Allow the system call |
| `LOG` | `SECCOMP_RET_LOG` | Log and allow |
| `KILL` / `KILL_THREAD` / `DENY` | `SECCOMP_RET_KILL` | Kill the thread |
| `KILL_PROCESS` | `SECCOMP_RET_KILL_PROCESS` | Kill the entire process |
| `ERRNO(n)` | `SECCOMP_RET_ERRNO+n` | Return error n |
| `TRAP(n)` | `SECCOMP_RET_TRAP+n` | Send SIGSYS |
| `TRACE(n)` | `SECCOMP_RET_TRACE+n` | Notify the ptrace tracer |
| `USER_NOTIF` | `SECCOMP_RET_USER_NOTIF` | Notify a userspace supervisor |

The argument `n` for `ERRNO(n)`, `TRAP(n)`, and `TRACE(n)` is restricted to the range 0–65535.

## Default Action

```
DEFAULT KILL
```

If `DEFAULT` is not specified, the default action is `KILL` (kills the thread on any unmatched system call). `DEFAULT` can only be specified once; specifying it multiple times is a compilation error.

## Basic Rule Syntax

```kafel
ACTION {
    syscall_name1,
    syscall_name2,
    syscall_name3
}
```

Rules separated by commas are semantically equivalent to `||` (logical OR) and have the lowest precedence.

### Example

```kafel
// Allow read, write, exit_group
ALLOW {
    read,
    write,
    exit_group
}

// Block open with EPERM (errno 1)
// Note: Only numeric literals are valid as ERRNO() arguments. Constant names (e.g. EPERM) will cause a parse error
ERRNO(1) { open }

// Kill everything else
DEFAULT KILL
```

## Argument Filtering

Filtering can be done based on system call arguments.

### Named Arguments

Define argument names for non-standard system calls:

```kafel
some_syscall(first_arg, my_arg_name) {
    first_arg == 42 && my_arg_name != 42
}
```

### Direct Argument Access for Standard System Calls

```kafel
write { fd == 1 }          // Allow stdout only
read { fd == 0 }           // Allow stdin only
```

### Flag Testing with Bitwise Operations

```kafel
#define PROT_EXEC 0x4
#define O_RDONLY  0
#define O_CLOEXEC 0x80000

mmap { (prot & PROT_EXEC) == 0 }     // Allow only if PROT_EXEC flag is not set
open { flags == O_RDONLY|O_CLOEXEC }  // Allow read-only + CLOEXEC only
```

### Comparison Operators

| Operator | Description |
|----------|-------------|
| `==` | Equal to |
| `!=` | Not equal to |
| `<` | Less than |
| `>` | Greater than |
| `<=` | Less than or equal to |
| `>=` | Greater than or equal to |

### Logical Operators

| Operator | Description |
|----------|-------------|
| `!` | Logical NOT (unary) |
| `&&` | Logical AND |
| `\|\|` | Logical OR |

### Bitwise Operators

| Operator | Description |
|----------|-------------|
| `&` | Bitwise AND |
| `\|` | Bitwise OR |

The maximum expression depth is 200. A maximum of 6 arguments per syscall can be specified, matching the Linux kernel convention. Duplicate argument names within a single syscall definition are not allowed.

## Policy Definitions and USE

Named policies can be defined and reused:

```kafel
POLICY my_policy {
    ALLOW { read, write, exit_group }
    ERRNO(1) { open }
}

USE my_policy DEFAULT KILL
```

`USE somePolicy` inserts the body of `somePolicy` inline. Only previously defined policies can be referenced. Defining two policies with the same name is a compilation error.

## Architecture-Specific Filters

Rules that apply only on specific architectures can be defined. The `ON` guard accepts a single architecture name or a comma-separated list enclosed in braces:

```kafel
ALLOW {
    io_uring_setup ON x86_64,
    arm_fadvise64_64 ON arm,
    some_syscall ON { x86_64, aarch64 }
}
```

Supported architecture names (matching is case-insensitive): `x86_64` (alias: `amd64`), `x86` (alias: `i386`), `arm`, `aarch64` (alias: `arm64`), `mips` (alias: `mipso32`), `mips64`, `riscv64` (alias: `rv64`), `m68k`

Rules for architectures not being compiled for are ignored.

## Custom System Call Numbers

```kafel
#define mysyscall -1

POLICY my_const {
    ALLOW { mysyscall }
}

POLICY my_literal {
    ALLOW { SYSCALL[-1] }
}
```

The `SYSCALL[N]` syntax allows specifying an arbitrary system call number.

## Constant Definitions

```kafel
#define MY_CONSTANT 42
#define MY_FLAG 0x1000

// Note: Constants defined with #define cannot be used as ERRNO() arguments either (only NUMBER tokens are accepted)
ERRNO(42) { some_syscall }
```

Kafel does not automatically import Linux kernel header constants. To use constants such as `O_RDONLY`, `PROT_EXEC`, or `EPERM` in argument filtering expressions, they must be explicitly defined in the policy file using `#define`. Only numeric literals are accepted as action arguments for `ERRNO()`, `TRAP()`, and `TRACE()`.

Redefining a constant with a different value is a compilation error. Redefining with the same value is silently accepted. Constant definitions cannot be placed inside policy blocks.

## File Includes

```kafel
#include "some_other_file.policy"
#include "first.policy" "second.policy"
```

Multiple files, separated by whitespace, can be specified in one `#include` directive. Include directives are terminated by a newline or semicolon. Search paths must be explicitly registered via `kafel_add_include_search_path()`. The maximum include depth is 16.

## Complete Examples

### Minimal Allow Policy (Whitelist Approach)

```kafel
POLICY basic {
    ALLOW {
        read,
        write,
        close,
        exit,
        exit_group,
        brk,
        mmap,
        munmap,
        mprotect,
        fstat,
        arch_prctl,
        set_tid_address,
        set_robust_list
    }
}

USE basic DEFAULT KILL_PROCESS
```

### bash Configuration Example Bundled with nsjail

```kafel
ERRNO(1337) { geteuid }
ERRNO(1) { ptrace, sched_setaffinity }
KILL_PROCESS { syslog }
DEFAULT ALLOW
```

In this example:
- `geteuid` fails with errno 1337 (raw kernel return value: -1337, displayed by `/usr/bin/id` as `euid=4294965959`)
- `ptrace` and `sched_setaffinity` are blocked with EPERM (1)
- `syslog` kills the process
- Everything else is allowed
