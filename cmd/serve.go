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

	serveCmd.Flags().Int("port", defaultPort(), "port to listen on (default overridden by PORT env var)")
	serveCmd.Flags().Int("run-timeout", 30, "sandbox run timeout in seconds")
	serveCmd.Flags().Int("compile-timeout", 30, "sandbox compile timeout in seconds")
	serveCmd.Flags().Int("output-limit", 1<<20, "maximum combined output bytes")
	serveCmd.Flags().Int("max-files", 10, "maximum number of files per request")
	serveCmd.Flags().Int("max-file-size", 256<<10, "maximum file size in bytes per file")
	serveCmd.Flags().Int("max-body-size", 5<<20, "maximum request body size in bytes")
}

func runServe(cmd *cobra.Command, _ []string) error {
	port, err := cmd.Flags().GetInt("port")
	if err != nil {
		return err
	}

	runTimeout, err := cmd.Flags().GetInt("run-timeout")
	if err != nil {
		return err
	}

	compileTimeout, err := cmd.Flags().GetInt("compile-timeout")
	if err != nil {
		return err
	}

	outputLimit, err := cmd.Flags().GetInt("output-limit")
	if err != nil {
		return err
	}

	maxFiles, err := cmd.Flags().GetInt("max-files")
	if err != nil {
		return err
	}

	maxFileSize, err := cmd.Flags().GetInt("max-file-size")
	if err != nil {
		return err
	}

	maxBodySize, err := cmd.Flags().GetInt("max-body-size")
	if err != nil {
		return err
	}

	if runTimeout <= 0 {
		return fmt.Errorf("--run-timeout must be a positive integer, got %d", runTimeout)
	}
	if compileTimeout <= 0 {
		return fmt.Errorf("--compile-timeout must be a positive integer, got %d", compileTimeout)
	}
	if outputLimit <= 0 {
		return fmt.Errorf("--output-limit must be a positive integer, got %d", outputLimit)
	}
	if maxFiles <= 0 {
		return fmt.Errorf("--max-files must be a positive integer, got %d", maxFiles)
	}
	if maxFileSize <= 0 {
		return fmt.Errorf("--max-file-size must be a positive integer, got %d", maxFileSize)
	}
	if maxBodySize <= 0 {
		return fmt.Errorf("--max-body-size must be a positive integer, got %d", maxBodySize)
	}

	cfg := sandbox.Config{
		RunTimeout:     runTimeout,
		CompileTimeout: compileTimeout,
		OutputLimit:    outputLimit,
	}

	h := &handler.Handler{Runner: sandbox.NewRunner(cfg), MaxFiles: maxFiles, MaxFileSize: maxFileSize}

	e := echo.New()
	e.HTTPErrorHandler = handler.NewHTTPErrorHandler()
	e.Use(middleware.RequestLogger())
	e.Use(middleware.BodyLimit(int64(maxBodySize)))
	e.POST("/v1/run", h.RunHandler)

	if err := e.Start(fmt.Sprintf(":%d", port)); err != nil {
		e.Logger.Error("failed to start server", "error", err)
	}
	return nil
}
