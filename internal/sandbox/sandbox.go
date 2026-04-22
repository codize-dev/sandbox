package sandbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// nsjailPath is the path to the nsjail binary inside the Docker image.
const nsjailPath = "/bin/nsjail"

type Status string

const (
	StatusOK                  Status = "OK"
	StatusSignal              Status = "SIGNAL"
	StatusTimeout             Status = "TIMEOUT"
	StatusOutputLimitExceeded Status = "OUTPUT_LIMIT_EXCEEDED"
)

var errOutputLimitExceeded = errors.New("output limit exceeded")

// Config holds runtime-configurable parameters for the sandbox.
type Config struct {
	RunTimeout     int // nsjail --time_limit and --rlimit_cpu for the run step, in seconds
	CompileTimeout int // nsjail --time_limit and --rlimit_cpu for the compile step, in seconds
	OutputLimit    int // maximum combined stdout+stderr bytes before killing the process
}

type Result struct {
	Stdout   string  `json:"stdout"`
	Stderr   string  `json:"stderr"`
	Output   string  `json:"output"`
	ExitCode int     `json:"exit_code"`
	Status   Status  `json:"status"`
	Signal   *string `json:"signal"`
	// DurationMs is the wall-clock milliseconds between cmd.Start() and
	// cmd.Wait() returning for this step's nsjail invocation. It is always
	// non-negative and is populated on every Result that Runner.exec
	// returns alongside a nil error, covering every Status value: OK,
	// SIGNAL, TIMEOUT, and OUTPUT_LIMIT_EXCEEDED. Error returns yield a
	// zero-valued Result that the caller discards. Includes nsjail's own
	// startup/teardown in addition to user code runtime.
	DurationMs int64 `json:"duration_ms"`
}

// RunOutput holds the results of a sandbox execution.
// Compile is non-nil only for compiled runtimes (e.g. Go).
// Run is nil when compilation fails.
type RunOutput struct {
	Compile *Result
	Run     *Result
}

// execParams holds the parameters for a single nsjail invocation.
type execParams struct {
	command    []string    // command and arguments to run inside the sandbox
	bindMounts []BindMount // runtime-specific bind mounts (mounted read-only by buildArgs)
	env        []string    // environment variables in "KEY=VALUE" format
	tmpDir     string      // host directory bind-mounted as /sandbox (sandbox working directory)
	limits     Limits      // nsjail resource limits (rlimits + cgroups)
	timeout    int         // nsjail --time_limit and --rlimit_cpu value for this invocation, in seconds
	stdin      []byte      // raw stdin bytes for the child process; nil or empty leaves the child's stdin empty (read returns EOF immediately; see Runner.Run godoc)
}

// resolveSignal decodes Unix signal-encoded exit codes. By convention, shells
// encode signal kills as exit code 128 + signal number. Returns the signal
// name (e.g. "SIGKILL") if applicable, nil otherwise.
func resolveSignal(exitCode int, logOutput string) *string {
	if exitCode > 128 && strings.Contains(logOutput, "terminated with signal: ") {
		if name := unix.SignalName(syscall.Signal(exitCode - 128)); name != "" {
			return &name
		}
	}
	return nil
}

// applyDefaultFiles writes each default file into dir if a file with that
// name does not already exist. User-provided files are never overwritten.
func applyDefaultFiles(dir string, files []DefaultFile) error {
	for _, f := range files {
		dest := filepath.Join(dir, f.Name)
		if _, err := os.Stat(dest); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to check %s: %w", f.Name, err)
		}
		if err := os.WriteFile(dest, f.Content, 0644); err != nil {
			return fmt.Errorf("failed to write default file %s: %w", f.Name, err)
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

// newOutputLimitResult builds the Result returned from Runner.exec when
// the sandboxed process is killed for exceeding the combined output
// limit. Lives as a package-level helper so DurationMs propagation on
// this path can be unit-tested in isolation, mirroring collectResult.
//
// ExitCode is hardcoded to 137 (128 + SIGKILL). The Go process kills
// nsjail via proc.Kill() (SIGKILL), so nsjail never gets a chance to
// report the child's exit status. Go's ExitCode() returns -1 for
// signal-terminated processes, but 137 is used here to stay consistent
// with other SIGKILL scenarios (timeout, OOM).
func newOutputLimitResult(durationMs int64) Result {
	return Result{
		Stdout:     "",
		Stderr:     "",
		Output:     "",
		ExitCode:   137,
		Status:     StatusOutputLimitExceeded,
		Signal:     nil,
		DurationMs: durationMs,
	}
}

// exec runs a single nsjail invocation with the given parameters and returns
// the result. Called once for interpreted runtimes, or twice (compile + run)
// for compiled runtimes.
func (r *Runner) exec(ctx context.Context, params execParams) (Result, error) {
	e := &execution{
		timeout:     params.timeout,
		outputLimit: r.cfg.OutputLimit,
		command:     params.command,
		bindMounts:  params.bindMounts,
		env:         params.env,
		tmpDir:      params.tmpDir,
		limits:      params.limits,
		stdin:       params.stdin,
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
		if waitErr := cmd.Wait(); waitErr != nil {
			slog.WarnContext(ctx, "cmd.Wait failed after pipe drain error",
				"wait_error", waitErr,
				"drain_error", drainErr,
				"stdin_bytes", len(e.stdin),
			)
		}
		return Result{}, fmt.Errorf("sandbox execution failed: %w", drainErr)
	}
	outputLimitHit := errors.Is(drainErr, errOutputLimitExceeded)

	waitErr := cmd.Wait()
	durationMs := e.elapsedMs()

	logData, err := io.ReadAll(e.logR)
	if err != nil {
		return Result{}, fmt.Errorf("failed to read nsjail log output: %w", err)
	}
	logStr := string(logData)

	if outputLimitHit {
		return newOutputLimitResult(durationMs), nil
	}

	if ctx.Err() != nil {
		return Result{}, ctx.Err()
	}

	return e.collectResult(waitErr, logStr, durationMs)
}

// execTimeoutBuffer is the grace period added beyond the nsjail --time_limit /
// --rlimit_cpu values so nsjail can terminate the sandboxed process and
// exec.Cmd can return before the Go context fires.
const execTimeoutBuffer = 10 * time.Second

// Run executes the given entryFile inside an nsjail sandbox. stdin is piped
// to the run-step child process; a nil or empty stdin leaves the child's
// stdin empty (read returns EOF immediately). Compile-step children never
// receive stdin — the compile step is expected to be deterministic with
// respect to the provided files alone.
func (r *Runner) Run(ctx context.Context, rt Runtime, tmpDir, entryFile string, stdin []byte) (RunOutput, error) {
	// Compute a single Go-level context timeout covering the full lifecycle
	// (compile + run). Per-step time limits are enforced by nsjail's
	// --time_limit; this context is a coarse safety net that fires only if
	// nsjail fails to enforce its own limit.
	execTimeout := time.Duration(r.cfg.RunTimeout)*time.Second + execTimeoutBuffer
	if _, ok := rt.(CompiledRuntime); ok {
		execTimeout += time.Duration(r.cfg.CompileTimeout) * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, execTimeout)
	defer cancel()

	defaults, err := readDefaultFiles(rt.Name())
	if err != nil {
		return RunOutput{}, fmt.Errorf("failed to read default files: %w", err)
	}
	if err := applyDefaultFiles(tmpDir, defaults); err != nil {
		return RunOutput{}, fmt.Errorf("failed to apply default files: %w", err)
	}

	var compileResult *Result

	if cr, ok := rt.(CompiledRuntime); ok {
		result, err := r.exec(ctx, execParams{
			command:    cr.CompileCommand(),
			bindMounts: cr.CompileBindMounts(),
			env:        cr.CompileEnv(),
			tmpDir:     tmpDir,
			limits:     cr.CompileLimits(),
			timeout:    r.cfg.CompileTimeout,
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
		command:    rt.Command(filepath.Join("/sandbox", entryFile)),
		bindMounts: rt.BindMounts(),
		env:        rt.Env(),
		tmpDir:     tmpDir,
		limits:     rt.Limits(),
		timeout:    r.cfg.RunTimeout,
		stdin:      stdin,
	})
	if err != nil {
		return RunOutput{}, fmt.Errorf("run: %w", err)
	}

	return RunOutput{Compile: compileResult, Run: &result}, nil
}
