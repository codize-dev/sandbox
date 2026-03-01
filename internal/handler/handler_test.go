package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateFileName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		fileName  string
		wantErr   bool
		errSubstr string
	}{
		{
			name:     "simple js file",
			fileName: "index.js",
		},
		{
			name:     "simple ruby file",
			fileName: "main.rb",
		},
		{
			name:     "hidden file starting with dot",
			fileName: ".hidden",
		},
		{
			name:     "file name with spaces",
			fileName: "file with spaces.js",
		},
		{
			name:     "three consecutive dots",
			fileName: "...",
		},
		{
			name:      "empty name",
			fileName:  "",
			wantErr:   true,
			errSubstr: "must not be empty",
		},
		{
			name:      "single dot",
			fileName:  ".",
			wantErr:   true,
			errSubstr: "is not allowed",
		},
		{
			name:      "double dot",
			fileName:  "..",
			wantErr:   true,
			errSubstr: "is not allowed",
		},
		{
			name:      "path traversal with leading dotdot-slash",
			fileName:  "../escape",
			wantErr:   true,
			errSubstr: "invalid characters",
		},
		{
			name:      "deep path traversal",
			fileName:  "../../etc/passwd",
			wantErr:   true,
			errSubstr: "invalid characters",
		},
		{
			name:      "path traversal embedded in path",
			fileName:  "foo/../../bar",
			wantErr:   true,
			errSubstr: "invalid characters",
		},
		{
			name:      "subdirectory slash",
			fileName:  "a/b",
			wantErr:   true,
			errSubstr: "invalid characters",
		},
		{
			name:      "null byte at start",
			fileName:  "\x00hidden",
			wantErr:   true,
			errSubstr: "invalid characters",
		},
		{
			name:      "null byte embedded",
			fileName:  "foo\x00bar",
			wantErr:   true,
			errSubstr: "invalid characters",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validateFileName(tc.fileName)
			if tc.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errSubstr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
