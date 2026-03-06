package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/codize-dev/sandbox/internal/handler"
	"github.com/codize-dev/sandbox/internal/sandbox"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:  "serve",
	RunE: runServe,
}

var (
	flagPort           int
	flagRunTimeout     int
	flagCompileTimeout int
	flagOutputLimit    int
	flagMaxFiles       int
	flagMaxFileSize    int
	flagMaxBodySize    int
)

func defaultPort() int {
	if v := os.Getenv("PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			return p
		}
	}
	return 8080
}

func init() {
	rootCmd.AddCommand(serveCmd)

	f := serveCmd.Flags()
	f.IntVar(&flagPort, "port", defaultPort(), "port to listen on (default overridden by PORT env var)")
	f.IntVar(&flagRunTimeout, "run-timeout", 30, "sandbox run timeout in seconds")
	f.IntVar(&flagCompileTimeout, "compile-timeout", 30, "sandbox compile timeout in seconds")
	f.IntVar(&flagOutputLimit, "output-limit", 1<<20, "maximum combined output bytes")
	f.IntVar(&flagMaxFiles, "max-files", 10, "maximum number of files per request")
	f.IntVar(&flagMaxFileSize, "max-file-size", 256<<10, "maximum file size in bytes per file")
	f.IntVar(&flagMaxBodySize, "max-body-size", 5<<20, "maximum request body size in bytes")
}

func runServe(_ *cobra.Command, _ []string) error {
	for _, c := range []struct {
		name  string
		value int
	}{
		{"--port", flagPort},
		{"--run-timeout", flagRunTimeout},
		{"--compile-timeout", flagCompileTimeout},
		{"--output-limit", flagOutputLimit},
		{"--max-files", flagMaxFiles},
		{"--max-file-size", flagMaxFileSize},
		{"--max-body-size", flagMaxBodySize},
	} {
		if c.value <= 0 {
			return fmt.Errorf("%s must be a positive integer, got %d", c.name, c.value)
		}
	}

	cfg := sandbox.Config{
		RunTimeout:     flagRunTimeout,
		CompileTimeout: flagCompileTimeout,
		OutputLimit:    flagOutputLimit,
	}

	h := &handler.Handler{Runner: sandbox.NewRunner(cfg), MaxFiles: flagMaxFiles, MaxFileSize: flagMaxFileSize}

	e := echo.New()
	e.HTTPErrorHandler = handler.NewHTTPErrorHandler()
	e.Use(middleware.RequestLogger())
	e.Use(middleware.BodyLimit(int64(flagMaxBodySize)))
	e.POST("/v1/run", h.RunHandler)

	if err := e.Start(fmt.Sprintf(":%d", flagPort)); err != nil {
		e.Logger.Error("failed to start server", "error", err)
	}
	return nil
}
