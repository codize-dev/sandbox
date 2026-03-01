package cmd

import (
	"time"

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

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().String("addr", ":8080", "TCP address to listen on")
	serveCmd.Flags().Int("timeout", 30, "sandbox run timeout in seconds")
	serveCmd.Flags().Int("output-limit", 1<<20, "maximum combined output bytes")
}

func runServe(cmd *cobra.Command, _ []string) error {
	addr, err := cmd.Flags().GetString("addr")
	if err != nil {
		return err
	}

	timeout, err := cmd.Flags().GetInt("timeout")
	if err != nil {
		return err
	}

	outputLimit, err := cmd.Flags().GetInt("output-limit")
	if err != nil {
		return err
	}

	cfg := sandbox.Config{
		RunTimeout:  timeout,
		ExecTimeout: time.Duration(timeout+10) * time.Second,
		OutputLimit: outputLimit,
	}

	h := &handler.Handler{Config: cfg}

	e := echo.New()
	e.Use(middleware.RequestLogger())
	e.POST("/v1/run", h.RunHandler)

	if err := e.Start(addr); err != nil {
		e.Logger.Error("failed to start server", "error", err)
	}
	return nil
}
