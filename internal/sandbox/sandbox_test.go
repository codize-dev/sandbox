package sandbox

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookupRuntime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		runtime RuntimeName
		wantErr bool
	}{
		{name: "node is valid", runtime: RuntimeNode, wantErr: false},
		{name: "ruby is valid", runtime: RuntimeRuby, wantErr: false},
		{name: "go is valid", runtime: RuntimeGo, wantErr: false},
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
				assert.Contains(t, err.Error(), "invalid or missing runtime")
			} else {
				assert.NotNil(t, rt)
				assert.NoError(t, err)
			}
		})
	}
}

func TestNodeRuntimeRlimits(t *testing.T) {
	t.Parallel()
	rt := nodeRuntime{}
	got := rt.Rlimits()
	assert.Equal(t, "4096", got.AS)
	assert.Equal(t, "64", got.Fsize)
	assert.Equal(t, "64", got.Nofile)
	assert.Equal(t, "64", got.Nproc)
}

func TestRubyRuntimeRlimits(t *testing.T) {
	t.Parallel()
	rt := rubyRuntime{}
	got := rt.Rlimits()
	assert.Equal(t, "1024", got.AS)
	assert.Equal(t, "64", got.Fsize)
	assert.Equal(t, "64", got.Nofile)
	assert.Equal(t, "32", got.Nproc)
}

func TestGoRuntimeRlimits(t *testing.T) {
	t.Parallel()
	rt := goRuntime{}

	run := rt.Rlimits()
	assert.Equal(t, "1024", run.AS)
	assert.Equal(t, "64", run.Fsize)
	assert.Equal(t, "64", run.Nofile)
	assert.Equal(t, "64", run.Nproc)

	compile := rt.CompileRlimits()
	assert.Equal(t, "4096", compile.AS)
	assert.Equal(t, "64", compile.Fsize)
	assert.Equal(t, "256", compile.Nofile)
	assert.Equal(t, "128", compile.Nproc)
}

func TestReadDefaultFiles(t *testing.T) {
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

	t.Run("unknown runtime has no defaults", func(t *testing.T) {
		t.Parallel()
		files, err := readDefaultFiles("unknown")
		assert.NoError(t, err)
		assert.Empty(t, files)
	})
}

func TestApplyDefaultFiles(t *testing.T) {
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

func TestRuntimeRestrictedFiles(t *testing.T) {
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

	t.Run("go restricts go.mod and go.sum", func(t *testing.T) {
		t.Parallel()
		rt, err := LookupRuntime(RuntimeGo)
		require.NoError(t, err)
		restricted := rt.RestrictedFiles()
		assert.ElementsMatch(t, []string{"go.mod", "go.sum"}, restricted)
	})
}
