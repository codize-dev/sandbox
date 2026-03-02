package sandbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLookupRuntime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		runtime string
		wantErr bool
	}{
		{name: "node is valid", runtime: "node", wantErr: false},
		{name: "ruby is valid", runtime: "ruby", wantErr: false},
		{name: "go is valid", runtime: "go", wantErr: false},
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
	assert.Equal(t, "hard", got.AS)
	assert.Equal(t, "1024", got.Fsize)
	assert.Equal(t, "hard", got.Nofile)
}

func TestRubyRuntimeRlimits(t *testing.T) {
	t.Parallel()
	rt := rubyRuntime{}
	got := rt.Rlimits()
	assert.Equal(t, "hard", got.AS)
	assert.Equal(t, "1024", got.Fsize)
	assert.Equal(t, "hard", got.Nofile)
}

func TestGoRuntimeRlimits(t *testing.T) {
	t.Parallel()
	rt := goRuntime{}

	run := rt.Rlimits()
	assert.Equal(t, "hard", run.AS)
	assert.Equal(t, "1024", run.Fsize)
	assert.Equal(t, "hard", run.Nofile)

	compile := rt.CompileRlimits()
	assert.Equal(t, "hard", compile.AS)
	assert.Equal(t, "1024", compile.Fsize)
	assert.Equal(t, "hard", compile.Nofile)
}
