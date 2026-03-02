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

type Result struct {
	Stdout   string  `json:"stdout"`
	Stderr   string  `json:"stderr"`
	Output   string  `json:"output"`
	ExitCode int     `json:"exit_code"`
	Status   Status  `json:"status"`
	Signal   *string `json:"signal"`
}

// RunOutput holds the results of a sandbox execution.
// Compile is non-nil only for compiled runtimes (e.g. Go).
// Run is nil when compilation fails.
type RunOutput struct {
	Compile *Result
	Run     *Result
}

type execParams struct {
	command    []string
	bindMounts []BindMount
	env        []string
	tmpDir     string
	tmpHome    string
	rlimits    Rlimits
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

// exec runs a single nsjail invocation with the given parameters and returns
// the result. Called once for interpreted runtimes, or twice (compile + run)
// for compiled runtimes.
func (r *Runner) exec(ctx context.Context, params execParams) (Result, error) {
	e := &execution{
		runTimeout:  r.cfg.RunTimeout,
		outputLimit: r.cfg.OutputLimit,
		command:     params.command,
		bindMounts:  params.bindMounts,
		env:         params.env,
		tmpDir:      params.tmpDir,
		tmpHome:     params.tmpHome,
		rlimits:     params.rlimits,
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

// Run executes the given entryFile inside an nsjail sandbox.
func (r *Runner) Run(ctx context.Context, rt Runtime, tmpDir, entryFile string) (RunOutput, error) {
	timeout := r.cfg.ExecTimeout
	if _, ok := rt.(CompiledRuntime); ok {
		timeout += time.Duration(r.cfg.RunTimeout) * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	tmpHome, err := os.MkdirTemp("", "sandbox-tmp-*")
	if err != nil {
		return RunOutput{}, fmt.Errorf("failed to create tmp directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpHome) }()

	if err := rt.PrepareDir(tmpDir); err != nil {
		return RunOutput{}, fmt.Errorf("failed to prepare directory: %w", err)
	}

	var compileResult *Result

	if cr, ok := rt.(CompiledRuntime); ok {
		result, err := r.exec(ctx, execParams{
			command:    cr.CompileCommand(),
			bindMounts: cr.CompileBindMounts(),
			env:        cr.CompileEnv(),
			tmpDir:     tmpDir,
			tmpHome:    tmpHome,
			rlimits:    cr.CompileRlimits(),
		})
		if err != nil {
			return RunOutput{}, fmt.Errorf("compile: %w", err)
		}
		compileResult = &result

		if result.ExitCode != 0 || result.Status != StatusOK {
			return RunOutput{Compile: compileResult}, nil
		}
	}

	result, err := r.exec(ctx, execParams{
		command:    rt.Command("/code/" + entryFile),
		bindMounts: rt.BindMounts(),
		env:        rt.Env(),
		tmpDir:     tmpDir,
		tmpHome:    tmpHome,
		rlimits:    rt.Rlimits(),
	})
	if err != nil {
		return RunOutput{}, fmt.Errorf("run: %w", err)
	}

	return RunOutput{Compile: compileResult, Run: &result}, nil
}
