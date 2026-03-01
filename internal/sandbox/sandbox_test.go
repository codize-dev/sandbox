package sandbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRuntime_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		runtime Runtime
		wantErr bool
	}{
		{name: "node is valid", runtime: RuntimeNode, wantErr: false},
		{name: "ruby is valid", runtime: RuntimeRuby, wantErr: false},
		{name: "empty string is invalid", runtime: "", wantErr: true},
		{name: "unknown runtime is invalid", runtime: "python", wantErr: true},
		{name: "capitalized Node is invalid", runtime: "Node", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.runtime.Validate()
			if tc.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "invalid or missing runtime")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
