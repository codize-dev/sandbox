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

type RunRequest struct {
	Runtime string `json:"runtime"`
	Files   []File `json:"files"`
}

type File struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type RunResponse struct {
	Run sandbox.Result `json:"run"`
}

// Handler holds dependencies for the HTTP handler.
type Handler struct {
	Runner *sandbox.Runner
}

func (h *Handler) RunHandler(c *echo.Context) error {
	var req RunRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body: " + err.Error(),
		})
	}

	rt, err := req.Validate()
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	tmpDir, err := os.MkdirTemp("", "sandbox-*")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to create temporary directory",
		})
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	for _, f := range req.Files {
		decoded, err := base64.StdEncoding.DecodeString(f.Content)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("file %q: invalid base64 content", f.Name),
			})
		}

		dest := filepath.Join(tmpDir, f.Name)
		if filepath.Dir(dest) != tmpDir {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("file name %q results in a path outside the sandbox", f.Name),
			})
		}

		if err := os.WriteFile(dest, decoded, 0644); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("failed to write file %q", f.Name),
			})
		}
	}

	result, err := h.Runner.Run(c.Request().Context(), rt, tmpDir, req.Files[0].Name)
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

	return c.JSON(http.StatusOK, RunResponse{Run: result})
}

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

func (req RunRequest) Validate() (sandbox.Runtime, error) {
	rt := sandbox.Runtime(req.Runtime)
	if err := rt.Validate(); err != nil {
		return "", err
	}
	if len(req.Files) == 0 {
		return "", errors.New("files must not be empty")
	}
	for _, f := range req.Files {
		if err := f.Validate(); err != nil {
			return "", err
		}
	}
	return rt, nil
}
