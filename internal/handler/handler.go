package handler

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/codize-dev/sandbox/internal/sandbox"
	"github.com/labstack/echo/v5"
)

// RunRequest is the JSON request body for POST /v1/run.
type RunRequest struct {
	Runtime *string     `json:"runtime"`
	Files   []File      `json:"files"`
	Stdin   *StdinInput `json:"stdin"`
}

// File represents a single source file in the run request.
// The first file in the request's Files array is used as the entrypoint.
type File struct {
	Name          *string `json:"name"`
	Content       *string `json:"content"`
	Base64Encoded bool    `json:"base64_encoded"`
}

// StdinInput represents the optional stdin payload in a run request. The
// payload is delivered only to the run-step child process; compile-step
// children never receive stdin. When the top-level "stdin" field is
// omitted, the RunRequest.Stdin pointer is nil. When present, Content
// is required: a nil Content (e.g. JSON `"stdin": {}`) produces a
// `required` validation error. When Base64Encoded is true, Content is
// interpreted as a base64-encoded string and decoded server-side; when
// false (the default), the raw bytes of Content are forwarded to the
// run-step child's stdin directly.
type StdinInput struct {
	Content       *string `json:"content"`
	Base64Encoded bool    `json:"base64_encoded"`
}

// RunResponse is the JSON response for POST /v1/run.
// Compile is nil for interpreted runtimes. Run is nil when compilation fails.
type RunResponse struct {
	Compile *sandbox.Result `json:"compile"`
	Run     *sandbox.Result `json:"run"`
}

// Handler holds dependencies for the HTTP handler.
type Handler struct {
	Runner      *sandbox.Runner
	MaxFiles    int
	MaxFileSize int
	// MaxStdinSize bounds the post-decode stdin length in bytes. For
	// base64-encoded stdin this is the decoded byte count, not the wire
	// string length. The raw request body is independently bounded by
	// Echo's BodyLimit middleware configured at server startup; if
	// MaxStdinSize exceeds the body limit, the body limit wins.
	MaxStdinSize int
}

type decodedFile struct {
	name    string
	content []byte
}

func validationErr(path []any, msg string) *ErrorResponse {
	return &ErrorResponse{
		Code:    CodeValidationError,
		Message: CodeValidationError.Message(),
		Errors:  []ValidationError{{Path: path, Message: msg}},
	}
}

func (h *Handler) decodeRunRequest(req RunRequest) (sandbox.Runtime, []decodedFile, []byte, *ErrorResponse) {
	if req.Runtime == nil {
		return nil, nil, nil, validationErr([]any{"runtime"}, "required")
	}
	if *req.Runtime == "" {
		return nil, nil, nil, validationErr([]any{"runtime"}, "must not be empty")
	}
	rt, err := sandbox.LookupRuntime(sandbox.RuntimeName(*req.Runtime))
	if err != nil {
		return nil, nil, nil, validationErr([]any{"runtime"}, err.Error())
	}

	if req.Files == nil {
		return nil, nil, nil, validationErr([]any{"files"}, "required")
	}
	if len(req.Files) == 0 {
		return nil, nil, nil, validationErr([]any{"files"}, "must not be empty")
	}
	if len(req.Files) > h.MaxFiles {
		return nil, nil, nil, validationErr([]any{"files"}, fmt.Sprintf("too many files (max: %d)", h.MaxFiles))
	}

	// Validate stdin before the per-file decode loop so a malformed
	// stdin fails fast without copying (and in some cases base64-decoding)
	// every file payload.
	stdin, errResp := decodeStdin(req.Stdin, h.MaxStdinSize)
	if errResp != nil {
		return nil, nil, nil, errResp
	}

	files := make([]decodedFile, len(req.Files))
	for i, f := range req.Files {
		if f.Name == nil {
			return nil, nil, nil, validationErr([]any{"files", i, "name"}, "required")
		}
		if f.Content == nil {
			return nil, nil, nil, validationErr([]any{"files", i, "content"}, "required")
		}
		if err := f.Validate(); err != nil {
			return nil, nil, nil, validationErr([]any{"files", i, "name"}, err.Error())
		}
		if slices.Contains(rt.RestrictedFiles(), *f.Name) {
			return nil, nil, nil, validationErr([]any{"files", i, "name"}, "not allowed for this runtime")
		}
		var content []byte
		if f.Base64Encoded {
			var err error
			content, err = base64.StdEncoding.DecodeString(*f.Content)
			if err != nil {
				return nil, nil, nil, validationErr([]any{"files", i, "content"}, "invalid base64")
			}
		} else {
			content = []byte(*f.Content)
		}
		if len(content) > h.MaxFileSize {
			return nil, nil, nil, validationErr([]any{"files", i, "content"}, fmt.Sprintf("file too large (max: %d bytes)", h.MaxFileSize))
		}
		files[i] = decodedFile{name: *f.Name, content: content}
	}

	return rt, files, stdin, nil
}

func (h *Handler) RunHandler(c *echo.Context) error {
	var req RunRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    CodeInvalidRequestBody,
			Message: CodeInvalidRequestBody.Message(),
		})
	}

	rt, files, stdin, errResp := h.decodeRunRequest(req)
	if errResp != nil {
		return c.JSON(http.StatusBadRequest, *errResp)
	}

	tmpDir, err := os.MkdirTemp("", "sandbox-*")
	if err != nil {
		slog.ErrorContext(c.Request().Context(), "failed to create temp directory", "error", err)
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    CodeInternalError,
			Message: CodeInternalError.Message(),
		})
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := writeFiles(tmpDir, files); err != nil {
		slog.ErrorContext(c.Request().Context(), "failed to write files to temp directory",
			"error", err,
			"runtime", *req.Runtime,
		)
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    CodeInternalError,
			Message: CodeInternalError.Message(),
		})
	}

	output, err := h.Runner.Run(c.Request().Context(), rt, tmpDir, files[0].name, stdin)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return c.JSON(http.StatusGatewayTimeout, ErrorResponse{
				Code:    CodeTimeout,
				Message: CodeTimeout.Message(),
			})
		}
		slog.ErrorContext(c.Request().Context(), "sandbox execution failed",
			"error", err,
			"runtime", *req.Runtime,
			"stdin_bytes", len(stdin),
		)
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    CodeInternalError,
			Message: CodeInternalError.Message(),
		})
	}

	return c.JSON(http.StatusOK, RunResponse{Compile: output.Compile, Run: output.Run})
}

// decodeStdin validates and decodes the optional stdin payload. Returns
// (nil, nil) when input is nil (stdin omitted entirely). When input is
// non-nil but Content is nil (e.g. JSON `"stdin": {}`), returns a
// "required" validation error. Also fails on invalid base64 or when the
// decoded byte length exceeds maxSize (the check is applied after base64
// decoding, so maxSize bounds the raw stdin bytes delivered to the
// sandbox, not the wire string length).
func decodeStdin(input *StdinInput, maxSize int) ([]byte, *ErrorResponse) {
	if input == nil {
		return nil, nil
	}
	if input.Content == nil {
		return nil, validationErr([]any{"stdin", "content"}, "required")
	}
	var content []byte
	if input.Base64Encoded {
		// Short-circuit grossly over-sized base64 before allocating
		// the decoded buffer: any wire longer than EncodedLen(maxSize)
		// is guaranteed to decode to more than maxSize bytes, so we
		// reject without decoding. Without this guard, a client could
		// force the server to allocate up to MaxBodySize * 3/4 bytes
		// per request just to fail validation.
		if len(*input.Content) > base64.StdEncoding.EncodedLen(maxSize) {
			return nil, validationErr([]any{"stdin", "content"}, fmt.Sprintf("stdin too large (max: %d bytes)", maxSize))
		}
		decoded, err := base64.StdEncoding.DecodeString(*input.Content)
		if err != nil {
			return nil, validationErr([]any{"stdin", "content"}, "invalid base64")
		}
		// EncodedLen is coarse at the last 3-byte boundary (several
		// raw sizes share the same encoded length), so a wire that
		// passed the early bound can still decode to 1-2 bytes over
		// maxSize. Catch that here.
		if len(decoded) > maxSize {
			return nil, validationErr([]any{"stdin", "content"}, fmt.Sprintf("stdin too large (max: %d bytes)", maxSize))
		}
		content = decoded
	} else {
		// len(s) == len([]byte(s)) in Go, so bound the wire directly
		// to avoid copying an oversized string.
		if len(*input.Content) > maxSize {
			return nil, validationErr([]any{"stdin", "content"}, fmt.Sprintf("stdin too large (max: %d bytes)", maxSize))
		}
		content = []byte(*input.Content)
	}
	return content, nil
}

// writeFiles writes each decoded file into tmpDir.
func writeFiles(tmpDir string, files []decodedFile) error {
	for _, f := range files {
		dest := filepath.Join(tmpDir, f.name)
		if err := os.WriteFile(dest, f.content, 0644); err != nil {
			return fmt.Errorf("failed to write file %q: %w", f.name, err)
		}
	}
	return nil
}

// Validate checks that f.Name is safe to use as a flat filename.
// It rejects nil, empty names, names containing '/' or null bytes, and "." / "..".
// Because all slashes are rejected here, writeFiles does not need a secondary
// path traversal check after filepath.Join.
func (f File) Validate() error {
	if f.Name == nil || *f.Name == "" {
		return errors.New("must not be empty")
	}
	if strings.ContainsRune(*f.Name, '/') || strings.ContainsRune(*f.Name, 0) {
		return errors.New("contains invalid characters")
	}
	if *f.Name == "." || *f.Name == ".." {
		return errors.New("not allowed")
	}
	if len(*f.Name) > 255 {
		return errors.New("file name too long (max: 255 bytes)")
	}
	return nil
}
