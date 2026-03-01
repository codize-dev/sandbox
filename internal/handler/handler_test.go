package handler

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/codize-dev/sandbox/internal/sandbox"
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

func TestRunRequest_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		req       RunRequest
		wantErr   bool
		errSubstr string
	}{
		{
			name: "valid node request",
			req:  RunRequest{Runtime: "node", Files: []File{{Name: "index.js", Content: ""}}},
		},
		{
			name: "valid ruby request",
			req:  RunRequest{Runtime: "ruby", Files: []File{{Name: "main.rb", Content: ""}}},
		},
		{
			name:      "missing runtime",
			req:       RunRequest{Runtime: "", Files: []File{{Name: "index.js", Content: ""}}},
			wantErr:   true,
			errSubstr: "invalid or missing runtime",
		},
		{
			name:      "unknown runtime",
			req:       RunRequest{Runtime: "python", Files: []File{{Name: "main.py", Content: ""}}},
			wantErr:   true,
			errSubstr: "invalid or missing runtime",
		},
		{
			name:      "empty files slice",
			req:       RunRequest{Runtime: "node", Files: []File{}},
			wantErr:   true,
			errSubstr: "files must not be empty",
		},
		{
			name:      "nil files slice",
			req:       RunRequest{Runtime: "node", Files: nil},
			wantErr:   true,
			errSubstr: "files must not be empty",
		},
		{
			name:      "invalid file name in files",
			req:       RunRequest{Runtime: "node", Files: []File{{Name: "../escape", Content: ""}}},
			wantErr:   true,
			errSubstr: "invalid characters",
		},
		{
			name: "invalid base64 content",
			req: RunRequest{
				Runtime: "node",
				Files:   []File{{Name: "index.js", Content: "not-valid-base64!@#$"}},
			},
			wantErr:   true,
			errSubstr: "invalid base64 content",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := tc.req.Validate()
			if tc.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errSubstr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRunRequest_Validate_DecodedFiles(t *testing.T) {
	t.Parallel()

	req := RunRequest{
		Runtime: "node",
		Files: []File{
			{Name: "index.js", Content: base64.StdEncoding.EncodeToString([]byte("console.log('hello')"))},
			{Name: "helper.js", Content: base64.StdEncoding.EncodeToString([]byte("module.exports = {}"))},
		},
	}

	rt, files, err := req.Validate()
	require.NoError(t, err)

	assert.Equal(t, sandbox.Runtime("node"), rt)
	require.Len(t, files, 2)
	assert.Equal(t, "index.js", files[0].name)
	assert.Equal(t, []byte("console.log('hello')"), files[0].content)
	assert.Equal(t, "helper.js", files[1].name)
	assert.Equal(t, []byte("module.exports = {}"), files[1].content)
}

func TestWriteFiles(t *testing.T) {
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
