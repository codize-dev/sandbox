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
	Runtime string `json:"runtime"`
	Files   []File `json:"files"`
}

// File represents a single source file in the run request.
// The first file in the request's Files array is used as the entrypoint.
type File struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

// RunResponse is the JSON response for POST /v1/run.
// Compile is nil for interpreted runtimes. Run is nil when compilation fails.
type RunResponse struct {
	Compile *sandbox.Result `json:"compile"`
	Run     *sandbox.Result `json:"run"`
}

// Handler holds dependencies for the HTTP handler.
type Handler struct {
	Runner *sandbox.Runner
}

type decodedFile struct {
	name    string
	content []byte
}

func (h *Handler) RunHandler(c *echo.Context) error {
	var req RunRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body: " + err.Error(),
		})
	}

	rt, err := sandbox.LookupRuntime(sandbox.RuntimeName(req.Runtime))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}
	if len(req.Files) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "files must not be empty",
		})
	}

	files := make([]decodedFile, len(req.Files))
	for i, f := range req.Files {
		if err := f.Validate(); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": err.Error(),
			})
		}
		content, err := base64.StdEncoding.DecodeString(f.Content)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("file %q: invalid base64 content", f.Name),
			})
		}
		files[i] = decodedFile{name: f.Name, content: content}
	}

	tmpDir, err := os.MkdirTemp("", "sandbox-*")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to create temporary directory",
		})
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := writeFiles(tmpDir, files); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	output, err := h.Runner.Run(c.Request().Context(), rt, tmpDir, files[0].name)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return c.JSON(http.StatusGatewayTimeout, map[string]string{
				"error": "execution timed out",
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
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
// It rejects names containing '/' or null bytes, empty names, and "." / "..".
// Because all slashes are rejected here, writeFiles does not need a secondary
// path traversal check after filepath.Join.
func (f File) Validate() error {
	if f.Name == "" {
		return errors.New("file name must not be empty")
	}
	if strings.ContainsRune(f.Name, '/') || strings.ContainsRune(f.Name, 0) {
		return fmt.Errorf("file name %q contains invalid characters", f.Name)
	}
	if f.Name == "." || f.Name == ".." {
		return fmt.Errorf("file name %q is not allowed", f.Name)
	}
	return nil
}
