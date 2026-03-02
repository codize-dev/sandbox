package sandbox

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Runtime defines the interface that all sandbox runtimes must implement.
type Runtime interface {
	// Command returns the full command and arguments to execute inside the sandbox.
	// entryFile is the absolute path inside the sandbox (e.g. "/code/index.js").
	// Compiled runtimes may ignore this parameter when the executable path is
	// determined by the compilation step.
	Command(entryFile string) []string

	// BindMounts returns read-only bind mounts required by this runtime
	// (e.g. the runtime installation directory).
	BindMounts() []BindMount

	// Env returns environment variables for the sandbox in "KEY=VALUE" format
	// (e.g. "PATH=/mise/installs/node/24.14.0/bin").
	Env() []string

	// PrepareDir performs runtime-specific preparation on the working directory
	// before execution (e.g. generating a go.mod file if absent).
	// dir is the host path that will be mounted as /code in the sandbox.
	PrepareDir(dir string) error

	// Rlimits returns the nsjail resource limits for the run step.
	Rlimits() Rlimits
}

// CompiledRuntime is an optional interface for runtimes that require a
// compilation step before execution (e.g. Go). Runner checks for this
// interface via type assertion.
type CompiledRuntime interface {
	Runtime
	// CompileCommand returns the full command and arguments for the compilation step.
	CompileCommand() []string
	// CompileBindMounts returns read-only bind mounts required during compilation.
	CompileBindMounts() []BindMount
	// CompileEnv returns environment variables for the compilation sandbox in "KEY=VALUE" format.
	CompileEnv() []string

	// CompileRlimits returns the nsjail resource limits for the compile step.
	CompileRlimits() Rlimits
}

var _ CompiledRuntime = goRuntime{}

// BindMount represents a read-only bind mount for nsjail (-R src:dst).
type BindMount struct {
	Src string
	Dst string
}

// Rlimits holds nsjail resource limit flags for a single execution step.
// Each field corresponds to a --rlimit_* nsjail flag.
// Valid values are a numeric string (e.g. "1024") or "hard" (system hard limit).
type Rlimits struct {
	AS     string // --rlimit_as (MiB or "hard")
	Fsize  string // --rlimit_fsize (MiB or "hard")
	Nofile string // --rlimit_nofile (count or "hard")
}

var runtimes = map[string]Runtime{
	"node": nodeRuntime{},
	"ruby": rubyRuntime{},
	"go":   goRuntime{},
}

// LookupRuntime returns the Runtime for the given name, or an error if unknown.
func LookupRuntime(name string) (Runtime, error) {
	rt, ok := runtimes[name]
	if !ok {
		names := make([]string, 0, len(runtimes))
		for k := range runtimes {
			names = append(names, fmt.Sprintf("%q", k))
		}
		sort.Strings(names)
		return nil, fmt.Errorf("invalid or missing runtime: must be one of %s", strings.Join(names, ", "))
	}
	return rt, nil
}

// --- Node.js ---

type nodeRuntime struct{}

func (nodeRuntime) Command(entryFile string) []string {
	return []string{"/mise/installs/node/24.14.0/bin/node", entryFile}
}

func (nodeRuntime) BindMounts() []BindMount {
	return []BindMount{{Src: "/mise/installs/node/24.14.0", Dst: "/mise/installs/node/24.14.0"}}
}

func (nodeRuntime) Env() []string {
	return []string{"PATH=/mise/installs/node/24.14.0/bin"}
}

func (nodeRuntime) PrepareDir(_ string) error {
	return nil
}

// Rlimits returns resource limits for Node.js execution.
// AS 4096 MiB: V8 uses mmap for heap management and requires a large virtual address space.
// Fsize 64 MiB: sufficient for typical output files.
// Nofile 64: covers stdin/stdout/stderr, nsjail internal fds, and V8 engine file descriptors.
func (nodeRuntime) Rlimits() Rlimits {
	return Rlimits{
		AS:     "4096",
		Fsize:  "64",
		Nofile: "64",
	}
}

// --- Ruby ---

type rubyRuntime struct{}

func (rubyRuntime) Command(entryFile string) []string {
	return []string{"/mise/installs/ruby/3.4.8/bin/ruby", entryFile}
}

func (rubyRuntime) BindMounts() []BindMount {
	return []BindMount{{Src: "/mise/installs/ruby/3.4.8", Dst: "/mise/installs/ruby/3.4.8"}}
}

func (rubyRuntime) Env() []string {
	return []string{"PATH=/mise/installs/ruby/3.4.8/bin"}
}

func (rubyRuntime) PrepareDir(_ string) error {
	return nil
}

// Rlimits returns resource limits for Ruby execution.
// AS 1024 MiB: sufficient for the Ruby interpreter and typical user scripts.
// Fsize 64 MiB: sufficient for typical output files.
// Nofile 64: covers stdin/stdout/stderr, nsjail internal fds, and Ruby runtime file descriptors.
func (rubyRuntime) Rlimits() Rlimits {
	return Rlimits{
		AS:     "1024",
		Fsize:  "64",
		Nofile: "64",
	}
}

// --- Go ---

type goRuntime struct{}

func (goRuntime) Command(_ string) []string {
	return []string{"/tmp/main"}
}

func (goRuntime) BindMounts() []BindMount {
	return nil
}

func (goRuntime) Env() []string {
	return nil
}

func (goRuntime) CompileCommand() []string {
	return []string{"/mise/installs/go/1.26.0/bin/go", "build", "-o", "/tmp/main", "."}
}

func (goRuntime) CompileBindMounts() []BindMount {
	return []BindMount{
		{Src: "/mise/installs/go/1.26.0", Dst: "/mise/installs/go/1.26.0"},
		{Src: "/mise/go-cache", Dst: "/mise/go-cache"},
	}
}

func (goRuntime) CompileEnv() []string {
	return []string{
		"PATH=/mise/installs/go/1.26.0/bin",
		"GOROOT=/mise/installs/go/1.26.0",
		"GOPATH=/tmp/gopath",
		"GOCACHE=/mise/go-cache",
		"GOPROXY=off",
		"GOTELEMETRY=off",
		"CGO_ENABLED=0",
	}
}

// CompileRlimits returns resource limits for the Go compilation step.
// AS 4096 MiB: the Go compiler and linker together consume significant virtual address space; 4 GiB provides comfortable headroom.
// Fsize 64 MiB: sufficient for compiled binaries (typically 2-20 MiB).
// Nofile 256: go build opens many source and object files concurrently.
func (goRuntime) CompileRlimits() Rlimits {
	return Rlimits{
		AS:     "4096",
		Fsize:  "64",
		Nofile: "256",
	}
}

func (goRuntime) PrepareDir(dir string) error {
	goModPath := filepath.Join(dir, "go.mod")
	_, err := os.Stat(goModPath)
	if err == nil {
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to check go.mod: %w", err)
	}
	return os.WriteFile(goModPath, []byte("module sandbox\n\ngo 1.26\n"), 0644)
}

// Rlimits returns resource limits for Go runtime execution.
// AS 1024 MiB: sufficient for typical compiled Go programs.
// Fsize 64 MiB: sufficient for typical output files.
// Nofile 64: covers stdin/stdout/stderr, nsjail internal fds, and minimal runtime file descriptors.
func (goRuntime) Rlimits() Rlimits {
	return Rlimits{
		AS:     "1024",
		Fsize:  "64",
		Nofile: "64",
	}
}
