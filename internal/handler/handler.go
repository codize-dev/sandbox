package handler

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

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

func RunHandler(c *echo.Context) error {
	var req RunRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body: " + err.Error(),
		})
	}

	rt := sandbox.Runtime(req.Runtime)
	if !sandbox.ValidRuntime(rt) {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid or missing runtime: must be \"node\" or \"ruby\"",
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
		if err := os.WriteFile(filepath.Join(tmpDir, f.Name), decoded, 0644); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("failed to write file %q", f.Name),
			})
		}
	}

	result, err := sandbox.Run(c.Request().Context(), rt, tmpDir, req.Files[0].Name)
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
