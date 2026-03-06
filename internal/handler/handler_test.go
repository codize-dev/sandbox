package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/codize-dev/sandbox/internal/sandbox"
	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func strPtr(s string) *string { return &s }

func TestFile_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		fileName  *string
		wantErr   bool
		errSubstr string
	}{
		{
			name:     "simple js file",
			fileName: strPtr("index.js"),
		},
		{
			name:     "simple ruby file",
			fileName: strPtr("main.rb"),
		},
		{
			name:     "hidden file starting with dot",
			fileName: strPtr(".hidden"),
		},
		{
			name:     "file name with spaces",
			fileName: strPtr("file with spaces.js"),
		},
		{
			name:     "three consecutive dots",
			fileName: strPtr("..."),
		},
		{
			name:      "nil name",
			fileName:  nil,
			wantErr:   true,
			errSubstr: "must not be empty",
		},
		{
			name:      "empty name",
			fileName:  strPtr(""),
			wantErr:   true,
			errSubstr: "must not be empty",
		},
		{
			name:      "single dot",
			fileName:  strPtr("."),
			wantErr:   true,
			errSubstr: "not allowed",
		},
		{
			name:      "double dot",
			fileName:  strPtr(".."),
			wantErr:   true,
			errSubstr: "not allowed",
		},
		{
			name:      "path traversal with leading dotdot-slash",
			fileName:  strPtr("../escape"),
			wantErr:   true,
			errSubstr: "invalid characters",
		},
		{
			name:      "deep path traversal",
			fileName:  strPtr("../../etc/passwd"),
			wantErr:   true,
			errSubstr: "invalid characters",
		},
		{
			name:      "path traversal embedded in path",
			fileName:  strPtr("foo/../../bar"),
			wantErr:   true,
			errSubstr: "invalid characters",
		},
		{
			name:      "subdirectory slash",
			fileName:  strPtr("a/b"),
			wantErr:   true,
			errSubstr: "invalid characters",
		},
		{
			name:      "null byte at start",
			fileName:  strPtr("\x00hidden"),
			wantErr:   true,
			errSubstr: "invalid characters",
		},
		{
			name:      "null byte embedded",
			fileName:  strPtr("foo\x00bar"),
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

func TestRunHandler_TooManyFiles(t *testing.T) {
	t.Parallel()

	e := echo.New()

	body := RunRequest{
		Runtime: strPtr("node"),
		Files: []File{
			{Name: strPtr("a.js"), Content: strPtr("Y29uc29sZS5sb2coImEiKQ==")},
			{Name: strPtr("b.js"), Content: strPtr("Y29uc29sZS5sb2coImIiKQ==")},
			{Name: strPtr("c.js"), Content: strPtr("Y29uc29sZS5sb2coImMiKQ==")},
		},
	}
	jsonBody, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/v1/run", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := &Handler{
		Runner:      sandbox.NewRunner(sandbox.Config{RunTimeout: 30, CompileTimeout: 30, OutputLimit: 1 << 20}),
		MaxFiles:    2,
		MaxFileSize: 1 << 20,
	}

	err = h.RunHandler(c)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var resp ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, CodeValidationError, resp.Code)
	require.Len(t, resp.Errors, 1)
	assert.Equal(t, []any{"files"}, resp.Errors[0].Path)
	assert.Equal(t, "too many files (max: 2)", resp.Errors[0].Message)
}

func TestRunHandler_FileTooLarge(t *testing.T) {
	t.Parallel()

	e := echo.New()

	// 17 bytes "hello world 12345" -> base64 "aGVsbG8gd29ybGQgMTIzNDU="
	body := RunRequest{
		Runtime: strPtr("node"),
		Files: []File{
			{Name: strPtr("index.js"), Content: strPtr("aGVsbG8gd29ybGQgMTIzNDU=")},
		},
	}
	jsonBody, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/v1/run", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := &Handler{
		Runner:      sandbox.NewRunner(sandbox.Config{RunTimeout: 30, CompileTimeout: 30, OutputLimit: 1 << 20}),
		MaxFiles:    10,
		MaxFileSize: 16,
	}

	err = h.RunHandler(c)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var resp ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, CodeValidationError, resp.Code)
	require.Len(t, resp.Errors, 1)
	assert.Equal(t, []any{"files", float64(0), "content"}, resp.Errors[0].Path)
	assert.Equal(t, "file too large (max: 16 bytes)", resp.Errors[0].Message)
}
