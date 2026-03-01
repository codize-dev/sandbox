package sandbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

const nsjailPath = "/bin/nsjail"

type Runtime string

const (
	RuntimeNode Runtime = "node"
	RuntimeRuby Runtime = "ruby"
)

type Status string

const (
	StatusOK                  Status = "OK"
	StatusTimeout             Status = "TIMEOUT"
	StatusOutputLimitExceeded Status = "OUTPUT_LIMIT_EXCEEDED"
)

var errOutputLimitExceeded = errors.New("output limit exceeded")

// Config holds runtime-configurable parameters for the sandbox.
type Config struct {
	RunTimeout  int
	ExecTimeout time.Duration
	OutputLimit int
}

type runtimeConfig struct {
	binaryPath string
	installDir string
	pathEnv    string
}

var runtimes = map[Runtime]runtimeConfig{
	RuntimeNode: {
		binaryPath: "/mise/installs/node/24.14.0/bin/node",
		installDir: "/mise/installs/node/24.14.0",
		pathEnv:    "/mise/installs/node/24.14.0/bin",
	},
	RuntimeRuby: {
		binaryPath: "/mise/installs/ruby/3.4.8/bin/ruby",
		installDir: "/mise/installs/ruby/3.4.8",
		pathEnv:    "/mise/installs/ruby/3.4.8/bin",
	},
}

func (rt Runtime) Validate() error {
	if _, ok := runtimes[rt]; !ok {
		return errors.New("invalid or missing runtime: must be \"node\" or \"ruby\"")
	}
	return nil
}

type Result struct {
	Stdout   string  `json:"stdout"`
	Stderr   string  `json:"stderr"`
	Output   string  `json:"output"`
	ExitCode int     `json:"exit_code"`
	Status   Status  `json:"status"`
	Signal   *string `json:"signal"`
}

func resolveSignal(exitCode int, logOutput string) *string {
	if exitCode > 128 && strings.Contains(logOutput, "terminated with signal: ") {
		if name := unix.SignalName(syscall.Signal(exitCode - 128)); name != "" {
			return &name
		}
	}
	return nil
}

// Runner executes code in a sandboxed environment using nsjail.
type Runner struct {
	cfg Config
}

// NewRunner creates a Runner with the given configuration.
func NewRunner(cfg Config) *Runner {
	return &Runner{cfg: cfg}
}

// Run executes the given entryFile inside an nsjail sandbox.
func (r *Runner) Run(ctx context.Context, rt Runtime, tmpDir, entryFile string) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, r.cfg.ExecTimeout)
	defer cancel()

	tmpHome, err := os.MkdirTemp("", "sandbox-tmp-*")
	if err != nil {
		return Result{}, fmt.Errorf("failed to create tmp directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpHome) }()

	e := &execution{
		runTimeout:  r.cfg.RunTimeout,
		outputLimit: r.cfg.OutputLimit,
		rtCfg:       runtimes[rt],
		tmpDir:      tmpDir,
		entryFile:   entryFile,
		tmpHome:     tmpHome,
	}

	args := e.buildArgs()

	if err := e.openPipes(); err != nil {
		return Result{}, err
	}

	cmd, err := e.start(ctx, args)
	if err != nil {
		return Result{}, err
	}
	defer e.closeReadEnds()

	drainErr := e.drainPipes(ctx)
	if drainErr != nil && !errors.Is(drainErr, errOutputLimitExceeded) {
		_ = cmd.Wait()
		return Result{}, fmt.Errorf("sandbox execution failed: %w", drainErr)
	}
	outputLimitHit := errors.Is(drainErr, errOutputLimitExceeded)

	waitErr := cmd.Wait()

	logData, _ := io.ReadAll(e.logR)
	logStr := string(logData)

	if outputLimitHit {
		return Result{
			Stdout:   "",
			Stderr:   "",
			Output:   "",
			ExitCode: -1,
			Status:   StatusOutputLimitExceeded,
			Signal:   nil,
		}, nil
	}

	if ctx.Err() != nil {
		return Result{}, ctx.Err()
	}

	return e.collectResult(waitErr, logStr)
}
