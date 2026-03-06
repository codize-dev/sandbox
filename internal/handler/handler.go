package handler

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
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
	Runtime *string `json:"runtime"`
	Files   []File  `json:"files"`
}

// File represents a single source file in the run request.
// The first file in the request's Files array is used as the entrypoint.
type File struct {
	Name    *string `json:"name"`
	Content *string `json:"content"`
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

func (h *Handler) decodeRunRequest(req RunRequest) (sandbox.Runtime, []decodedFile, *ErrorResponse) {
	if req.Runtime == nil {
		return nil, nil, validationErr([]any{"runtime"}, "required")
	}
	if *req.Runtime == "" {
		return nil, nil, validationErr([]any{"runtime"}, "must not be empty")
	}
	rt, err := sandbox.LookupRuntime(sandbox.RuntimeName(*req.Runtime))
	if err != nil {
		return nil, nil, validationErr([]any{"runtime"}, err.Error())
	}

	if req.Files == nil {
		return nil, nil, validationErr([]any{"files"}, "required")
	}
	if len(req.Files) == 0 {
		return nil, nil, validationErr([]any{"files"}, "must not be empty")
	}
	if len(req.Files) > h.MaxFiles {
		return nil, nil, validationErr([]any{"files"}, fmt.Sprintf("too many files (max: %d)", h.MaxFiles))
	}

	files := make([]decodedFile, len(req.Files))
	for i, f := range req.Files {
		if f.Name == nil {
			return nil, nil, validationErr([]any{"files", i, "name"}, "required")
		}
		if f.Content == nil {
			return nil, nil, validationErr([]any{"files", i, "content"}, "required")
		}
		if err := f.Validate(); err != nil {
			return nil, nil, validationErr([]any{"files", i, "name"}, err.Error())
		}
		if slices.Contains(rt.RestrictedFiles(), *f.Name) {
			return nil, nil, validationErr([]any{"files", i, "name"}, "not allowed for this runtime")
		}
		content, err := base64.StdEncoding.DecodeString(*f.Content)
		if err != nil {
			return nil, nil, validationErr([]any{"files", i, "content"}, "invalid base64")
		}
		if len(content) > h.MaxFileSize {
			return nil, nil, validationErr([]any{"files", i, "content"}, fmt.Sprintf("file too large (max: %d bytes)", h.MaxFileSize))
		}
		files[i] = decodedFile{name: *f.Name, content: content}
	}

	return rt, files, nil
}

func (h *Handler) RunHandler(c *echo.Context) error {
	var req RunRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    CodeInvalidRequestBody,
			Message: CodeInvalidRequestBody.Message(),
		})
	}

	rt, files, errResp := h.decodeRunRequest(req)
	if errResp != nil {
		return c.JSON(http.StatusBadRequest, *errResp)
	}

	tmpDir, err := os.MkdirTemp("", "sandbox-*")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    CodeInternalError,
			Message: CodeInternalError.Message(),
		})
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := writeFiles(tmpDir, files); err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    CodeInternalError,
			Message: CodeInternalError.Message(),
		})
	}

	output, err := h.Runner.Run(c.Request().Context(), rt, tmpDir, files[0].name)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return c.JSON(http.StatusGatewayTimeout, ErrorResponse{
				Code:    CodeExecutionTimeout,
				Message: CodeExecutionTimeout.Message(),
			})
		}
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    CodeInternalError,
			Message: CodeInternalError.Message(),
		})
	}

	return c.JSON(http.StatusOK, RunResponse{Compile: output.Compile, Run: output.Run})
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
