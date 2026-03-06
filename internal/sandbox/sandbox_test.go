package sandbox

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_LookupRuntime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		runtime RuntimeName
		wantErr bool
	}{
		{name: "node is valid", runtime: RuntimeNode, wantErr: false},
		{name: "ruby is valid", runtime: RuntimeRuby, wantErr: false},
		{name: "go is valid", runtime: RuntimeGo, wantErr: false},
		{name: "bash is valid", runtime: RuntimeBash, wantErr: false},
		{name: "empty string is invalid", runtime: "", wantErr: true},
		{name: "unknown runtime is invalid", runtime: "python", wantErr: true},
		{name: "capitalized Node is invalid", runtime: "Node", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rt, err := LookupRuntime(tc.runtime)
			if tc.wantErr {
				assert.Nil(t, rt)
				assert.Error(t, err)
				if tc.runtime == "" {
					assert.Contains(t, err.Error(), "required")
				} else {
					assert.Contains(t, err.Error(), "must be one of")
				}
			} else {
				assert.NotNil(t, rt)
				assert.NoError(t, err)
			}
		})
	}
}

func TestNodeRuntime_Limits(t *testing.T) {
	t.Parallel()
	rt := nodeRuntime{}
	got := rt.Limits()
	assert.Equal(t, "4096", got.Rlimits.AS)
	assert.Equal(t, "64", got.Rlimits.Fsize)
	assert.Equal(t, "64", got.Rlimits.Nofile)
	assert.Equal(t, "soft", got.Rlimits.Nproc)
	assert.Equal(t, "64", got.Cgroups.PidsMax)
	assert.Equal(t, "268435456", got.Cgroups.MemMax)
	assert.Equal(t, "0", got.Cgroups.MemSwapMax)
	assert.Equal(t, "900", got.Cgroups.CpuMsPerSec)
}

func TestRubyRuntime_Limits(t *testing.T) {
	t.Parallel()
	rt := rubyRuntime{}
	got := rt.Limits()
	assert.Equal(t, "1024", got.Rlimits.AS)
	assert.Equal(t, "64", got.Rlimits.Fsize)
	assert.Equal(t, "64", got.Rlimits.Nofile)
	assert.Equal(t, "soft", got.Rlimits.Nproc)
	assert.Equal(t, "32", got.Cgroups.PidsMax)
	assert.Equal(t, "268435456", got.Cgroups.MemMax)
	assert.Equal(t, "0", got.Cgroups.MemSwapMax)
	assert.Equal(t, "900", got.Cgroups.CpuMsPerSec)
}

func TestGoRuntime_Limits(t *testing.T) {
	t.Parallel()
	rt := goRuntime{}

	run := rt.Limits()
	assert.Equal(t, "1024", run.Rlimits.AS)
	assert.Equal(t, "64", run.Rlimits.Fsize)
	assert.Equal(t, "64", run.Rlimits.Nofile)
	assert.Equal(t, "soft", run.Rlimits.Nproc)
	assert.Equal(t, "64", run.Cgroups.PidsMax)
	assert.Equal(t, "268435456", run.Cgroups.MemMax)
	assert.Equal(t, "0", run.Cgroups.MemSwapMax)
	assert.Equal(t, "900", run.Cgroups.CpuMsPerSec)

	compile := rt.CompileLimits()
	assert.Equal(t, "4096", compile.Rlimits.AS)
	assert.Equal(t, "64", compile.Rlimits.Fsize)
	assert.Equal(t, "256", compile.Rlimits.Nofile)
	assert.Equal(t, "soft", compile.Rlimits.Nproc)
	assert.Equal(t, "128", compile.Cgroups.PidsMax)
	assert.Equal(t, "268435456", compile.Cgroups.MemMax)
	assert.Equal(t, "0", compile.Cgroups.MemSwapMax)
	assert.Equal(t, "900", compile.Cgroups.CpuMsPerSec)
}

func TestBashRuntime_Limits(t *testing.T) {
	t.Parallel()
	rt := bashRuntime{}
	got := rt.Limits()
	assert.Equal(t, "512", got.Rlimits.AS)
	assert.Equal(t, "64", got.Rlimits.Fsize)
	assert.Equal(t, "64", got.Rlimits.Nofile)
	assert.Equal(t, "soft", got.Rlimits.Nproc)
	assert.Equal(t, "32", got.Cgroups.PidsMax)
	assert.Equal(t, "268435456", got.Cgroups.MemMax)
	assert.Equal(t, "0", got.Cgroups.MemSwapMax)
	assert.Equal(t, "900", got.Cgroups.CpuMsPerSec)
}

func TestExecution_buildArgs(t *testing.T) {
	t.Parallel()

	e := &execution{
		timeout: 10,
		command: []string{"/usr/bin/node", "/code/index.js"},
		bindMounts: []BindMount{
			{Src: "/mise/installs/node/24", Dst: "/mise/installs/node/24"},
		},
		env:    []string{"PATH=/usr/bin:/bin"},
		tmpDir: "/tmp/sandbox-code",
		limits: Limits{
			Rlimits: Rlimits{AS: "4096", Fsize: "64", Nofile: "64", Nproc: "32"},
			Cgroups: Cgroups{PidsMax: "42", MemMax: "1000000", MemSwapMax: "0", CpuMsPerSec: "750"},
		},
	}

	want := []string{
		"-C", "/etc/nsjail/nsjail.cfg",
		"-R", "/mise/installs/node/24:/mise/installs/node/24",
		"-B", "/tmp/sandbox-code:/code",
		"--rlimit_as", "4096",
		"--rlimit_fsize", "64",
		"--rlimit_nofile", "64",
		"--rlimit_nproc", "32",
		"--rlimit_cpu", "10",
		"--time_limit", "10",
		"--cgroup_pids_max", "42",
		"--cgroup_mem_max", "1000000",
		"--cgroup_mem_swap_max", "0",
		"--cgroup_cpu_ms_per_sec", "750",
		"-E", "PATH=/usr/bin:/bin",
		"--",
		"/usr/bin/node", "/code/index.js",
	}

	assert.Equal(t, want, e.buildArgs())
}

func Test_readDefaultFiles(t *testing.T) {
	t.Parallel()

	t.Run("go has go.mod and go.sum", func(t *testing.T) {
		t.Parallel()
		files, err := readDefaultFiles(RuntimeGo)
		require.NoError(t, err)
		require.Len(t, files, 2)
		assert.Equal(t, "go.mod", files[0].Name)
		assert.Contains(t, string(files[0].Content), "module sandbox")
		assert.Contains(t, string(files[0].Content), "golang.org/x/text")
		assert.Equal(t, "go.sum", files[1].Name)
		assert.Contains(t, string(files[1].Content), "golang.org/x/text")
	})

	t.Run("node has no defaults", func(t *testing.T) {
		t.Parallel()
		files, err := readDefaultFiles(RuntimeNode)
		assert.NoError(t, err)
		assert.Empty(t, files)
	})

	t.Run("ruby has no defaults", func(t *testing.T) {
		t.Parallel()
		files, err := readDefaultFiles(RuntimeRuby)
		assert.NoError(t, err)
		assert.Empty(t, files)
	})

	t.Run("bash has no defaults", func(t *testing.T) {
		t.Parallel()
		files, err := readDefaultFiles(RuntimeBash)
		assert.NoError(t, err)
		assert.Empty(t, files)
	})

	t.Run("unknown runtime has no defaults", func(t *testing.T) {
		t.Parallel()
		files, err := readDefaultFiles("unknown")
		assert.NoError(t, err)
		assert.Empty(t, files)
	})
}

func Test_applyDefaultFiles(t *testing.T) {
	t.Parallel()

	t.Run("writes file when absent", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		files := []DefaultFile{{Name: "go.mod", Content: []byte("module test\n")}}
		err := applyDefaultFiles(dir, files)
		assert.NoError(t, err)
		data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		assert.NoError(t, err)
		assert.Equal(t, "module test\n", string(data))
	})

	t.Run("skips file when already exists", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		existing := filepath.Join(dir, "go.mod")
		err := os.WriteFile(existing, []byte("user content\n"), 0644)
		assert.NoError(t, err)

		files := []DefaultFile{{Name: "go.mod", Content: []byte("default content\n")}}
		err = applyDefaultFiles(dir, files)
		assert.NoError(t, err)
		data, err := os.ReadFile(existing)
		assert.NoError(t, err)
		assert.Equal(t, "user content\n", string(data))
	})

	t.Run("no-op when files is nil", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		err := applyDefaultFiles(dir, nil)
		assert.NoError(t, err)
	})
}

func TestRuntime_RestrictedFiles(t *testing.T) {
	t.Parallel()

	t.Run("node has no restricted files", func(t *testing.T) {
		t.Parallel()
		rt, err := LookupRuntime(RuntimeNode)
		require.NoError(t, err)
		assert.Empty(t, rt.RestrictedFiles())
	})

	t.Run("ruby has no restricted files", func(t *testing.T) {
		t.Parallel()
		rt, err := LookupRuntime(RuntimeRuby)
		require.NoError(t, err)
		assert.Empty(t, rt.RestrictedFiles())
	})

	t.Run("go restricts go.mod, go.sum, and main", func(t *testing.T) {
		t.Parallel()
		rt, err := LookupRuntime(RuntimeGo)
		require.NoError(t, err)
		restricted := rt.RestrictedFiles()
		assert.ElementsMatch(t, []string{"go.mod", "go.sum", "main"}, restricted)
	})

	t.Run("bash has no restricted files", func(t *testing.T) {
		t.Parallel()
		rt, err := LookupRuntime(RuntimeBash)
		require.NoError(t, err)
		assert.Empty(t, rt.RestrictedFiles())
	})
}
