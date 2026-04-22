package handler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
		{
			name:     "file name exactly 255 bytes",
			fileName: strPtr(strings.Repeat("a", 255)),
		},
		{
			name:      "file name exceeding 255 bytes",
			fileName:  strPtr(strings.Repeat("a", 256)),
			wantErr:   true,
			errSubstr: "file name too long",
		},
		{
			name:     "multi-byte UTF-8 filename exactly 255 bytes",
			fileName: strPtr(strings.Repeat("あ", 85)), // 85 * 3 = 255 bytes
		},
		{
			name:      "multi-byte UTF-8 filename exceeding 255 bytes",
			fileName:  strPtr(strings.Repeat("あ", 86)), // 86 * 3 = 258 bytes
			wantErr:   true,
			errSubstr: "file name too long",
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
			{Name: strPtr("a.js"), Content: strPtr(`console.log("a")`)},
			{Name: strPtr("b.js"), Content: strPtr(`console.log("b")`)},
			{Name: strPtr("c.js"), Content: strPtr(`console.log("c")`)},
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

	// 17 bytes plain text
	body := RunRequest{
		Runtime: strPtr("node"),
		Files: []File{
			{Name: strPtr("index.js"), Content: strPtr("hello world 12345")},
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

func TestDecodeRunRequest_Base64Encoded(t *testing.T) {
	t.Parallel()

	h := &Handler{
		Runner:      sandbox.NewRunner(sandbox.Config{RunTimeout: 30, CompileTimeout: 30, OutputLimit: 1 << 20}),
		MaxFiles:    10,
		MaxFileSize: 1 << 20,
	}

	t.Run("base64_encoded true decodes content", func(t *testing.T) {
		t.Parallel()
		// "hello" → base64 "aGVsbG8="
		req := RunRequest{
			Runtime: strPtr("node"),
			Files: []File{
				{Name: strPtr("index.js"), Content: strPtr("aGVsbG8="), Base64Encoded: true},
			},
		}
		_, files, _, errResp := h.decodeRunRequest(req)
		assert.Nil(t, errResp)
		require.Len(t, files, 1)
		assert.Equal(t, []byte("hello"), files[0].content)
	})

	t.Run("base64_encoded false uses content as-is", func(t *testing.T) {
		t.Parallel()
		req := RunRequest{
			Runtime: strPtr("node"),
			Files: []File{
				{Name: strPtr("index.js"), Content: strPtr("console.log('hi')")},
			},
		}
		_, files, _, errResp := h.decodeRunRequest(req)
		assert.Nil(t, errResp)
		require.Len(t, files, 1)
		assert.Equal(t, []byte("console.log('hi')"), files[0].content)
	})

	t.Run("base64_encoded true with invalid base64 returns error", func(t *testing.T) {
		t.Parallel()
		req := RunRequest{
			Runtime: strPtr("node"),
			Files: []File{
				{Name: strPtr("index.js"), Content: strPtr("not-valid!@#"), Base64Encoded: true},
			},
		}
		_, _, _, errResp := h.decodeRunRequest(req)
		require.NotNil(t, errResp)
		assert.Equal(t, CodeValidationError, errResp.Code)
		require.Len(t, errResp.Errors, 1)
		assert.Equal(t, "invalid base64", errResp.Errors[0].Message)
	})
}

func TestDecodeStdin(t *testing.T) {
	t.Parallel()

	const maxSize = 1024

	t.Run("nil input returns nil bytes and no error", func(t *testing.T) {
		t.Parallel()
		b, errResp := decodeStdin(nil, maxSize)
		assert.Nil(t, errResp)
		assert.Nil(t, b)
	})

	t.Run("content nil returns validation error", func(t *testing.T) {
		t.Parallel()
		b, errResp := decodeStdin(&StdinInput{Content: nil}, maxSize)
		assert.Nil(t, b)
		require.NotNil(t, errResp)
		assert.Equal(t, CodeValidationError, errResp.Code)
		require.Len(t, errResp.Errors, 1)
		assert.Equal(t, []any{"stdin", "content"}, errResp.Errors[0].Path)
		assert.Equal(t, "required", errResp.Errors[0].Message)
	})

	t.Run("plain content returns bytes as-is", func(t *testing.T) {
		t.Parallel()
		b, errResp := decodeStdin(&StdinInput{Content: strPtr("hello")}, maxSize)
		assert.Nil(t, errResp)
		assert.Equal(t, []byte("hello"), b)
	})

	t.Run("empty plain content returns empty bytes", func(t *testing.T) {
		t.Parallel()
		b, errResp := decodeStdin(&StdinInput{Content: strPtr("")}, maxSize)
		assert.Nil(t, errResp)
		assert.Equal(t, []byte{}, b)
	})

	t.Run("base64_encoded true decodes valid base64", func(t *testing.T) {
		t.Parallel()
		// "hello" → base64 "aGVsbG8="
		b, errResp := decodeStdin(&StdinInput{Content: strPtr("aGVsbG8="), Base64Encoded: true}, maxSize)
		assert.Nil(t, errResp)
		assert.Equal(t, []byte("hello"), b)
	})

	t.Run("base64_encoded true with invalid base64 returns error", func(t *testing.T) {
		t.Parallel()
		b, errResp := decodeStdin(&StdinInput{Content: strPtr("not-valid!@#"), Base64Encoded: true}, maxSize)
		assert.Nil(t, b)
		require.NotNil(t, errResp)
		assert.Equal(t, CodeValidationError, errResp.Code)
		require.Len(t, errResp.Errors, 1)
		assert.Equal(t, []any{"stdin", "content"}, errResp.Errors[0].Path)
		assert.Equal(t, "invalid base64", errResp.Errors[0].Message)
	})

	t.Run("decoded size equal to max succeeds", func(t *testing.T) {
		t.Parallel()
		content := strings.Repeat("a", maxSize)
		b, errResp := decodeStdin(&StdinInput{Content: &content}, maxSize)
		assert.Nil(t, errResp)
		assert.Equal(t, []byte(content), b)
	})

	t.Run("decoded size above max returns error", func(t *testing.T) {
		t.Parallel()
		content := strings.Repeat("a", maxSize+1)
		b, errResp := decodeStdin(&StdinInput{Content: &content}, maxSize)
		assert.Nil(t, b)
		require.NotNil(t, errResp)
		assert.Equal(t, CodeValidationError, errResp.Code)
		require.Len(t, errResp.Errors, 1)
		assert.Equal(t, []any{"stdin", "content"}, errResp.Errors[0].Path)
		assert.Contains(t, errResp.Errors[0].Message, "stdin too large")
		assert.Contains(t, errResp.Errors[0].Message, "max: 1024")
	})

	t.Run("base64 decoded size above max returns error", func(t *testing.T) {
		t.Parallel()
		// Pin that the size check runs AFTER base64 decoding, not on
		// the wire string. Raw bytes of maxSize+1 encode to a base64
		// string of ceil((maxSize+1)/3)*4 characters (wider than the
		// decoded payload), so a wire-length check would report a
		// different over-threshold figure.
		raw := make([]byte, maxSize+1)
		content := base64.StdEncoding.EncodeToString(raw)
		b, errResp := decodeStdin(&StdinInput{Content: &content, Base64Encoded: true}, maxSize)
		assert.Nil(t, b)
		require.NotNil(t, errResp)
		assert.Equal(t, CodeValidationError, errResp.Code)
		require.Len(t, errResp.Errors, 1)
		assert.Equal(t, []any{"stdin", "content"}, errResp.Errors[0].Path)
		assert.Contains(t, errResp.Errors[0].Message, "stdin too large")
		assert.Contains(t, errResp.Errors[0].Message, "max: 1024")
	})

	t.Run("base64 wire grossly over EncodedLen short-circuits before decode", func(t *testing.T) {
		t.Parallel()
		// Pin the early-guard: a wire length strictly larger than
		// EncodedLen(maxSize) must be rejected without allocating the
		// decoded buffer. The payload (a long run of 'A's) is actually
		// valid base64 — we choose it precisely so that if the early
		// guard were removed, the decode path would succeed with a
		// huge allocation instead of returning `stdin too large`.
		wireLen := base64.StdEncoding.EncodedLen(maxSize) + 4
		content := strings.Repeat("A", wireLen)
		b, errResp := decodeStdin(&StdinInput{Content: &content, Base64Encoded: true}, maxSize)
		assert.Nil(t, b)
		require.NotNil(t, errResp)
		assert.Equal(t, CodeValidationError, errResp.Code)
		require.Len(t, errResp.Errors, 1)
		assert.Equal(t, []any{"stdin", "content"}, errResp.Errors[0].Path)
		assert.Contains(t, errResp.Errors[0].Message, "stdin too large")
	})
}

// TestDecodeRunRequest_StdinPropagation pins that decodeRunRequest
// threads the decoded stdin through its third return value and that a
// malformed stdin fails the request before the per-file decode loop
// runs.
func TestDecodeRunRequest_StdinPropagation(t *testing.T) {
	t.Parallel()

	h := &Handler{
		Runner:       sandbox.NewRunner(sandbox.Config{RunTimeout: 30, CompileTimeout: 30, OutputLimit: 1 << 20}),
		MaxFiles:     10,
		MaxFileSize:  1 << 20,
		MaxStdinSize: 1024,
	}

	t.Run("valid stdin is returned to the caller", func(t *testing.T) {
		t.Parallel()
		req := RunRequest{
			Runtime: strPtr("node"),
			Files: []File{
				{Name: strPtr("index.js"), Content: strPtr("console.log('hi')")},
			},
			Stdin: &StdinInput{Content: strPtr("hello stdin")},
		}
		_, _, stdin, errResp := h.decodeRunRequest(req)
		assert.Nil(t, errResp)
		assert.Equal(t, []byte("hello stdin"), stdin)
	})

	t.Run("invalid stdin fails before file decoding", func(t *testing.T) {
		t.Parallel()
		// Order-of-validation regression guard. The file here has
		// INVALID base64 content — if a future refactor moves per-file
		// decode ahead of stdin validation, the test would see
		// `{"files", 0, "content"}: "invalid base64"` instead of the
		// stdin error. The current order ensures the stdin-too-large
		// error wins, proving stdin is validated first.
		invalidBase64File := "not-valid-base64!@#"
		tooLargeStdin := strings.Repeat("s", h.MaxStdinSize+1)
		req := RunRequest{
			Runtime: strPtr("node"),
			Files: []File{
				{Name: strPtr("index.js"), Content: &invalidBase64File, Base64Encoded: true},
			},
			Stdin: &StdinInput{Content: &tooLargeStdin},
		}
		_, files, stdin, errResp := h.decodeRunRequest(req)
		require.NotNil(t, errResp)
		assert.Nil(t, files)
		assert.Nil(t, stdin)
		require.Len(t, errResp.Errors, 1)
		assert.Equal(t, []any{"stdin", "content"}, errResp.Errors[0].Path)
		assert.Contains(t, errResp.Errors[0].Message, "stdin too large")
	})
}

// TestRunRequest_StdinNullLiteral pins that a JSON wire body containing
// `"stdin": null` (literal null, as many typed-language clients emit for
// optional/nullable fields) is observationally identical to omitting the
// field. Go's json.Unmarshal resolves both to a nil *StdinInput pointer,
// and decodeRunRequest's third return stays nil ([]byte). The E2E
// framework cannot emit literal `null` (the apiStdin pointer is elided
// by omitempty when nil), so this invariant must live as a unit test.
func TestRunRequest_StdinNullLiteral(t *testing.T) {
	t.Parallel()

	h := &Handler{
		Runner:       sandbox.NewRunner(sandbox.Config{RunTimeout: 30, CompileTimeout: 30, OutputLimit: 1 << 20}),
		MaxFiles:     10,
		MaxFileSize:  1 << 20,
		MaxStdinSize: 1 << 20,
	}

	t.Run("literal null decodes to nil *StdinInput", func(t *testing.T) {
		t.Parallel()
		body := []byte(`{"runtime":"node","files":[{"name":"index.js","content":"x"}],"stdin":null}`)
		var req RunRequest
		require.NoError(t, json.Unmarshal(body, &req))
		assert.Nil(t, req.Stdin, "JSON null must decode to nil *StdinInput")
	})

	t.Run("omitted stdin key decodes to nil *StdinInput", func(t *testing.T) {
		t.Parallel()
		body := []byte(`{"runtime":"node","files":[{"name":"index.js","content":"x"}]}`)
		var req RunRequest
		require.NoError(t, json.Unmarshal(body, &req))
		assert.Nil(t, req.Stdin)
	})

	t.Run("literal null propagates as nil []byte through decodeRunRequest", func(t *testing.T) {
		t.Parallel()
		body := []byte(`{"runtime":"node","files":[{"name":"index.js","content":"x"}],"stdin":null}`)
		var req RunRequest
		require.NoError(t, json.Unmarshal(body, &req))

		_, _, stdin, errResp := h.decodeRunRequest(req)
		assert.Nil(t, errResp)
		assert.Nil(t, stdin, "null stdin must produce nil []byte (equivalent to omitted)")
	})

	t.Run("literal null and omitted stdin produce identical decode results", func(t *testing.T) {
		t.Parallel()
		bodyNull := []byte(`{"runtime":"node","files":[{"name":"index.js","content":"x"}],"stdin":null}`)
		bodyOmitted := []byte(`{"runtime":"node","files":[{"name":"index.js","content":"x"}]}`)

		var reqNull, reqOmitted RunRequest
		require.NoError(t, json.Unmarshal(bodyNull, &reqNull))
		require.NoError(t, json.Unmarshal(bodyOmitted, &reqOmitted))

		_, _, stdinNull, errRespNull := h.decodeRunRequest(reqNull)
		_, _, stdinOmitted, errRespOmitted := h.decodeRunRequest(reqOmitted)

		assert.Nil(t, errRespNull)
		assert.Nil(t, errRespOmitted)
		assert.Equal(t, stdinOmitted, stdinNull, "null and omitted stdin must decode identically")
	})
}
