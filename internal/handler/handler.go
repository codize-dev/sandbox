package handler

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
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
	Runner   *sandbox.Runner
	MaxFiles int
}

type decodedFile struct {
	name    string
	content []byte
}

func (h *Handler) RunHandler(c *echo.Context) error {
	var req RunRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    CodeInvalidRequestBody,
			Message: CodeInvalidRequestBody.Message(),
		})
	}

	if req.Runtime == nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    CodeValidationError,
			Message: CodeValidationError.Message(),
			Errors: []ValidationError{
				{Path: []any{"runtime"}, Message: "required"},
			},
		})
	}
	if *req.Runtime == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    CodeValidationError,
			Message: CodeValidationError.Message(),
			Errors: []ValidationError{
				{Path: []any{"runtime"}, Message: "must not be empty"},
			},
		})
	}
	rt, err := sandbox.LookupRuntime(sandbox.RuntimeName(*req.Runtime))
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    CodeValidationError,
			Message: CodeValidationError.Message(),
			Errors: []ValidationError{
				{Path: []any{"runtime"}, Message: err.Error()},
			},
		})
	}

	if req.Files == nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    CodeValidationError,
			Message: CodeValidationError.Message(),
			Errors: []ValidationError{
				{Path: []any{"files"}, Message: "required"},
			},
		})
	}
	if len(req.Files) == 0 {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    CodeValidationError,
			Message: CodeValidationError.Message(),
			Errors: []ValidationError{
				{Path: []any{"files"}, Message: "must not be empty"},
			},
		})
	}
	if len(req.Files) > h.MaxFiles {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    CodeValidationError,
			Message: CodeValidationError.Message(),
			Errors: []ValidationError{
				{Path: []any{"files"}, Message: fmt.Sprintf("too many files (max: %d)", h.MaxFiles)},
			},
		})
	}

	files := make([]decodedFile, len(req.Files))
	for i, f := range req.Files {
		if f.Name == nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{
				Code:    CodeValidationError,
				Message: CodeValidationError.Message(),
				Errors: []ValidationError{
					{Path: []any{"files", i, "name"}, Message: "required"},
				},
			})
		}
		if f.Content == nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{
				Code:    CodeValidationError,
				Message: CodeValidationError.Message(),
				Errors: []ValidationError{
					{Path: []any{"files", i, "content"}, Message: "required"},
				},
			})
		}
		if err := f.Validate(); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{
				Code:    CodeValidationError,
				Message: CodeValidationError.Message(),
				Errors: []ValidationError{
					{Path: []any{"files", i, "name"}, Message: err.Error()},
				},
			})
		}
		for _, restricted := range rt.RestrictedFiles() {
			if *f.Name == restricted {
				return c.JSON(http.StatusBadRequest, ErrorResponse{
					Code:    CodeValidationError,
					Message: CodeValidationError.Message(),
					Errors: []ValidationError{
						{Path: []any{"files", i, "name"}, Message: "not allowed for this runtime"},
					},
				})
			}
		}
		content, err := base64.StdEncoding.DecodeString(*f.Content)
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{
				Code:    CodeValidationError,
				Message: CodeValidationError.Message(),
				Errors: []ValidationError{
					{Path: []any{"files", i, "content"}, Message: "invalid base64"},
				},
			})
		}
		files[i] = decodedFile{name: *f.Name, content: content}
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
	return nil
}
