package sandbox

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

//go:embed all:defaults
var defaultFiles embed.FS

// RuntimeName identifies a supported runtime.
type RuntimeName string

const (
	RuntimeNode RuntimeName = "node"
	RuntimeRuby RuntimeName = "ruby"
	RuntimeGo   RuntimeName = "go"
)

// Runtime defines the interface that all sandbox runtimes must implement.
type Runtime interface {
	// Name returns the runtime identifier.
	Name() RuntimeName

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

	// Limits returns the nsjail resource limits for the run step.
	Limits() Limits

	// RestrictedFiles returns file names that users are not allowed to submit
	// for this runtime (e.g. managed dependency files like go.mod).
	RestrictedFiles() []string
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

	// CompileLimits returns the nsjail resource limits for the compile step.
	CompileLimits() Limits
}

var _ CompiledRuntime = goRuntime{}

// BindMount represents a read-only bind mount for nsjail (-R src:dst).
type BindMount struct {
	Src string
	Dst string
}

// Rlimits holds nsjail POSIX resource limit flags for a single execution step.
// Each field corresponds to a --rlimit_* nsjail flag.
// Valid values are a numeric string (e.g. "1024") or "hard" (inherit system hard limit).
type Rlimits struct {
	AS     string // --rlimit_as (MiB or "hard")
	Fsize  string // --rlimit_fsize (MiB or "hard")
	Nofile string // --rlimit_nofile (count or "hard")
	Nproc  string // --rlimit_nproc (count or "hard")
}

// Cgroups holds nsjail cgroup limit flags for a single execution step.
// Each field corresponds to a --cgroup_* nsjail flag.
// Valid values are a numeric string (e.g. "64"); 0 disables the limit.
type Cgroups struct {
	PidsMax string // --cgroup_pids_max (count; 0 = disabled)
}

// Limits combines POSIX resource limits and cgroup limits for a single
// nsjail execution step.
type Limits struct {
	Rlimits Rlimits
	Cgroups Cgroups
}

// DefaultFile represents a file that should be written to the working directory
// before execution if a file with that name does not already exist.
type DefaultFile struct {
	Name    string // filename relative to the working directory (e.g. "go.mod")
	Content []byte
}

// readDefaultFiles reads all files from defaults/<name> in the embedded FS.
// Returns (nil, nil) if the subdirectory does not exist (i.e. the runtime has no defaults).
// Files stored with a .tmpl suffix have that suffix stripped when read (workaround for
// the Go toolchain treating directories containing go.mod as separate modules).
func readDefaultFiles(name RuntimeName) ([]DefaultFile, error) {
	dir := "defaults/" + string(name)
	entries, err := defaultFiles.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read defaults directory for runtime %s: %w", name, err)
	}
	files := make([]DefaultFile, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := defaultFiles.ReadFile(dir + "/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("failed to read embedded default file %s/%s: %w", name, e.Name(), err)
		}
		fileName := strings.TrimSuffix(e.Name(), ".tmpl")
		files = append(files, DefaultFile{Name: fileName, Content: data})
	}
	return files, nil
}

var runtimes = map[RuntimeName]Runtime{
	RuntimeNode: nodeRuntime{},
	RuntimeRuby: rubyRuntime{},
	RuntimeGo:   goRuntime{},
}

// LookupRuntime returns the Runtime for the given name, or an error if unknown.
func LookupRuntime(name RuntimeName) (Runtime, error) {
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

func (nodeRuntime) Name() RuntimeName { return RuntimeNode }

func (nodeRuntime) Command(entryFile string) []string {
	return []string{"/mise/installs/node/24.14.0/bin/node", entryFile}
}

func (nodeRuntime) BindMounts() []BindMount {
	return []BindMount{{Src: "/mise/installs/node/24.14.0", Dst: "/mise/installs/node/24.14.0"}}
}

func (nodeRuntime) Env() []string {
	return []string{"PATH=/mise/installs/node/24.14.0/bin"}
}

// Limits returns resource limits for Node.js execution.
// Rlimits:
//   - AS 4096 MiB: V8 uses mmap for heap management and requires a large virtual address space.
//   - Fsize 64 MiB: sufficient for typical output files.
//   - Nofile 64: covers stdin/stdout/stderr, nsjail internal fds, and V8 engine file descriptors.
//   - Nproc soft: inherits the system soft limit; per-sandbox process limiting is handled by cgroup_pids_max.
//
// Cgroups:
//   - PidsMax 64: per-cgroup task limit (processes + threads); set equal to Nproc for consistency.
func (nodeRuntime) Limits() Limits {
	return Limits{
		Rlimits: Rlimits{
			AS:     "4096",
			Fsize:  "64",
			Nofile: "64",
			Nproc:  "soft",
		},
		Cgroups: Cgroups{
			PidsMax: "64",
		},
	}
}

func (nodeRuntime) RestrictedFiles() []string { return nil }

// --- Ruby ---

type rubyRuntime struct{}

func (rubyRuntime) Name() RuntimeName { return RuntimeRuby }

func (rubyRuntime) Command(entryFile string) []string {
	return []string{"/mise/installs/ruby/3.4.8/bin/ruby", entryFile}
}

func (rubyRuntime) BindMounts() []BindMount {
	return []BindMount{{Src: "/mise/installs/ruby/3.4.8", Dst: "/mise/installs/ruby/3.4.8"}}
}

func (rubyRuntime) Env() []string {
	return []string{"PATH=/mise/installs/ruby/3.4.8/bin"}
}

// Limits returns resource limits for Ruby execution.
// Rlimits:
//   - AS 1024 MiB: sufficient for the Ruby interpreter and typical user scripts.
//   - Fsize 64 MiB: sufficient for typical output files.
//   - Nofile 64: covers stdin/stdout/stderr, nsjail internal fds, and Ruby runtime file descriptors.
//   - Nproc soft: inherits the system soft limit; per-sandbox process limiting is handled by cgroup_pids_max.
//
// Cgroups:
//   - PidsMax 32: per-cgroup task limit (processes + threads); set equal to Nproc for consistency.
func (rubyRuntime) Limits() Limits {
	return Limits{
		Rlimits: Rlimits{
			AS:     "1024",
			Fsize:  "64",
			Nofile: "64",
			Nproc:  "soft",
		},
		Cgroups: Cgroups{
			PidsMax: "32",
		},
	}
}

func (rubyRuntime) RestrictedFiles() []string { return nil }

// --- Go ---

type goRuntime struct{}

func (goRuntime) Name() RuntimeName { return RuntimeGo }

// Command returns the path to the compiled binary. The entryFile parameter is
// unused because the output path is determined by CompileCommand.
func (goRuntime) Command(_ string) []string {
	return []string{"/tmp/main"}
}

// BindMounts returns nil because the compiled binary is statically linked
// and needs no runtime directories.
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
		{Src: "/mise/go-cache", Dst: "/mise/go-cache"},       // pre-built Go stdlib cache (read-only)
		{Src: "/mise/go-modcache", Dst: "/mise/go-modcache"}, // pre-downloaded module cache (read-only)
	}
}

func (goRuntime) CompileEnv() []string {
	return []string{
		"PATH=/mise/installs/go/1.26.0/bin",
		"GOROOT=/mise/installs/go/1.26.0",
		"GOPATH=/tmp/gopath",                                    // writable location for module metadata and build artifacts
		"GOMODCACHE=/mise/go-modcache",                          // read-only pre-downloaded module cache from Docker image
		"GOCACHEPROG=/usr/local/bin/gocacheprog /mise/go-cache", // read-only cache backed by pre-built stdlib cache from Docker image
		"GOPROXY=off",     // prevent network access from the compiler
		"GOTELEMETRY=off", // disable Go telemetry in the sandbox
		"CGO_ENABLED=0",   // no C compiler in the sandbox; produces a static binary
	}
}

// CompileLimits returns resource limits for the Go compilation step.
// Rlimits:
//   - AS 4096 MiB: the Go compiler and linker together consume significant virtual address space; 4 GiB provides comfortable headroom.
//   - Fsize 64 MiB: sufficient for compiled binaries (typically 2-20 MiB).
//   - Nofile 256: go build opens many source and object files concurrently.
//   - Nproc soft: inherits the system soft limit; per-sandbox process limiting is handled by cgroup_pids_max.
//
// Cgroups:
//   - PidsMax 128: per-cgroup task limit (processes + threads); set equal to Nproc for consistency.
func (goRuntime) CompileLimits() Limits {
	return Limits{
		Rlimits: Rlimits{
			AS:     "4096",
			Fsize:  "64",
			Nofile: "256",
			Nproc:  "soft",
		},
		Cgroups: Cgroups{
			PidsMax: "128",
		},
	}
}

// Limits returns resource limits for Go runtime execution.
// Rlimits:
//   - AS 1024 MiB: sufficient for typical compiled Go programs.
//   - Fsize 64 MiB: sufficient for typical output files.
//   - Nofile 64: covers stdin/stdout/stderr, nsjail internal fds, and minimal runtime file descriptors.
//   - Nproc soft: inherits the system soft limit; per-sandbox process limiting is handled by cgroup_pids_max.
//
// Cgroups:
//   - PidsMax 64: per-cgroup task limit (processes + threads); set equal to Nproc for consistency.
func (goRuntime) Limits() Limits {
	return Limits{
		Rlimits: Rlimits{
			AS:     "1024",
			Fsize:  "64",
			Nofile: "64",
			Nproc:  "soft",
		},
		Cgroups: Cgroups{
			PidsMax: "64",
		},
	}
}

// RestrictedFiles prevents users from overriding go.mod and go.sum, which
// must match the pre-downloaded module cache (GOPROXY=off forbids fetching).
func (goRuntime) RestrictedFiles() []string {
	return []string{"go.mod", "go.sum"}
}
