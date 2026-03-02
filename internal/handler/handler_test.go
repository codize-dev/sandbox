package handler

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFile_Validate(t *testing.T) {
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

			err := File{Name: tc.fileName}.Validate()
			if tc.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errSubstr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_writeFiles(t *testing.T) {
	t.Parallel()

	t.Run("writes files with correct content", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		files := []decodedFile{
			{name: "index.js", content: []byte("console.log('hello')")},
			{name: "helper.js", content: []byte("module.exports = {}")},
		}

		err := writeFiles(tmpDir, files)
		require.NoError(t, err)

		for _, f := range files {
			got, err := os.ReadFile(filepath.Join(tmpDir, f.name))
			require.NoError(t, err)
			assert.Equal(t, f.content, got)
		}
	})

	t.Run("returns error for non-existent directory", func(t *testing.T) {
		t.Parallel()

		files := []decodedFile{
			{name: "index.js", content: []byte("hello")},
		}

		err := writeFiles("/nonexistent/path", files)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to write file")
	})
}
