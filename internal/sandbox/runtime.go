package sandbox

import (
	"fmt"
	"sort"
	"strings"
)

// Runtime defines the interface that all sandbox runtimes must implement.
type Runtime interface {
	// Command returns the full command and arguments to execute inside the sandbox.
	// entryFile is the absolute path inside the sandbox (e.g. "/code/index.js").
	Command(entryFile string) []string

	// BindMounts returns read-only bind mounts required by this runtime
	// (e.g. the runtime installation directory).
	BindMounts() []BindMount

	// Env returns environment variables for the sandbox in "KEY=VALUE" format
	// (e.g. "PATH=/mise/installs/node/24.14.0/bin").
	Env() []string
}

// BindMount represents a read-only bind mount for nsjail (-R src:dst).
type BindMount struct {
	Src string
	Dst string
}

var runtimes = map[string]Runtime{
	"node": nodeRuntime{},
	"ruby": rubyRuntime{},
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
