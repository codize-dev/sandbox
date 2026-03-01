package sandbox

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
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

func Run(ctx context.Context, cfg Config, rt Runtime, tmpDir, entryFile string) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, cfg.ExecTimeout)
	defer cancel()

	tmpHome, err := os.MkdirTemp("", "sandbox-tmp-*")
	if err != nil {
		return Result{}, fmt.Errorf("failed to create tmp directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpHome) }()
	rtCfg := runtimes[rt]

	args := []string{
		"-Mo",
		// Capture nsjail logs via a pipe (fd 3) to detect timeout kills.
		// ExtraFiles[0] is always mapped to fd 3 in the child process.
		"--log_fd", "3",
		"-D", "/code",
		"-R", "/lib:/lib",
		"-R", "/usr:/usr",
	}

	if _, err := os.Stat("/lib64"); err == nil {
		args = append(args, "-R", "/lib64:/lib64")
	}

	args = append(args,
		"-R", rtCfg.installDir+":"+rtCfg.installDir,
		"-R", "/dev/null:/dev/null",
		"-R", "/dev/urandom:/dev/urandom",
		"-B", tmpDir+":/code",
		"-B", tmpHome+":/tmp",
		"-m", "none:/proc:proc:ro",
		"-s", "/proc/self/fd:/dev/fd",
		"--rlimit_as", "hard",
		"--time_limit", fmt.Sprintf("%d", cfg.RunTimeout),
		"-E", "PATH="+rtCfg.pathEnv,
		"-E", "HOME=/tmp",
		"--",
		rtCfg.binaryPath,
		"/code/"+entryFile,
	)

	cmd := exec.CommandContext(ctx, nsjailPath, args...)

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		return Result{}, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		_ = stdoutR.Close()
		_ = stdoutW.Close()
		return Result{}, fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	// Pipe for capturing nsjail log output. We read this after execution
	// to detect two conditions:
	// 1. Timeout: nsjail logs "run time >= time limit" before killing the
	//    child, letting us distinguish timeouts from other SIGKILL causes.
	// 2. Signal kills: nsjail logs "terminated with signal: ..." when the
	//    child is killed by a signal (WIFSIGNALED), letting us report the
	//    signal name and distinguish genuine kills from user code that
	//    voluntarily exits with a signal-like exit code.
	logR, logW, err := os.Pipe()
	if err != nil {
		_ = stdoutR.Close()
		_ = stdoutW.Close()
		_ = stderrR.Close()
		_ = stderrW.Close()
		return Result{}, fmt.Errorf("failed to create log pipe: %w", err)
	}

	cmd.Stdout = stdoutW
	cmd.Stderr = stderrW
	cmd.ExtraFiles = []*os.File{logW}

	if err := cmd.Start(); err != nil {
		_ = stdoutR.Close()
		_ = stdoutW.Close()
		_ = stderrR.Close()
		_ = stderrW.Close()
		_ = logR.Close()
		_ = logW.Close()
		return Result{}, fmt.Errorf("sandbox execution failed: %w", err)
	}

	// Close the parent's copy of the write ends. A pipe delivers EOF to
	// readers only when all write-end fds are closed; without this, the
	// read ends would block forever even after the child exits.
	_ = stdoutW.Close()
	_ = stderrW.Close()
	_ = logW.Close()

	var stdoutBuf, stderrBuf, combined bytes.Buffer

	drainErr := drainPipes(ctx, cmd.Process, stdoutR, stderrR, &stdoutBuf, &stderrBuf, &combined, cfg.OutputLimit)
	if drainErr != nil && !errors.Is(drainErr, errOutputLimitExceeded) {
		_ = cmd.Wait()
		_ = stdoutR.Close()
		_ = stderrR.Close()
		_ = logR.Close()
		return Result{}, fmt.Errorf("sandbox execution failed: %w", drainErr)
	}
	outputLimitHit := errors.Is(drainErr, errOutputLimitExceeded)

	_ = stdoutR.Close()
	_ = stderrR.Close()

	waitErr := cmd.Wait()

	if outputLimitHit {
		_ = logR.Close()
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
		_ = logR.Close()
		return Result{}, ctx.Err()
	}

	// Read nsjail log to detect timeout and signal kills. cmd.Wait() has
	// returned, so nsjail has exited and the write end of the log pipe is
	// guaranteed to be closed.
	logData, _ := io.ReadAll(logR)
	_ = logR.Close()
	logStr := string(logData)
	timedOut := strings.Contains(logStr, "run time >= time limit")
	if waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			status := StatusOK
			if timedOut {
				status = StatusTimeout
			}
			exitCode := exitErr.ExitCode()
			sig := resolveSignal(exitCode, logStr)
			return Result{
				Stdout:   base64.StdEncoding.EncodeToString(stdoutBuf.Bytes()),
				Stderr:   base64.StdEncoding.EncodeToString(stderrBuf.Bytes()),
				Output:   base64.StdEncoding.EncodeToString(combined.Bytes()),
				ExitCode: exitCode,
				Status:   status,
				Signal:   sig,
			}, nil
		}
		return Result{}, fmt.Errorf("sandbox execution failed: %w", waitErr)
	}

	return Result{
		Stdout:   base64.StdEncoding.EncodeToString(stdoutBuf.Bytes()),
		Stderr:   base64.StdEncoding.EncodeToString(stderrBuf.Bytes()),
		Output:   base64.StdEncoding.EncodeToString(combined.Bytes()),
		ExitCode: 0,
		Status:   StatusOK,
	}, nil
}

// drainPipes multiplexes reads from stdoutR and stderrR using poll(2),
// writing to per-stream buffers and a combined buffer. Processing in a
// single goroutine eliminates races on the combined buffer. When both
// pipes are ready simultaneously, stdout is processed first. The poll
// timeout is derived from ctx's deadline so that the execution timeout
// and client disconnects are respected promptly.
func drainPipes(ctx context.Context, proc *os.Process, stdoutR, stderrR *os.File, stdoutBuf, stderrBuf, combined *bytes.Buffer, outputLimit int) error {
	type pipe struct {
		file *os.File
		buf  *bytes.Buffer
		open bool
	}
	pipes := [2]pipe{
		{file: stdoutR, buf: stdoutBuf, open: true},
		{file: stderrR, buf: stderrBuf, open: true},
	}
	buf := make([]byte, 32*1024)

	deadline, hasDeadline := ctx.Deadline()

	for pipes[0].open || pipes[1].open {
		var fds [2]unix.PollFd
		var idx [2]int
		n := 0
		for i := range pipes {
			if pipes[i].open {
				fds[n] = unix.PollFd{Fd: int32(pipes[i].file.Fd()), Events: unix.POLLIN}
				idx[n] = i
				n++
			}
		}

		pollTimeout := -1
		if hasDeadline {
			ms := int(time.Until(deadline).Milliseconds())
			if ms <= 0 {
				if err := ctx.Err(); err != nil {
					return err
				}
				ms = 0
			}
			pollTimeout = ms
		}

		count, err := unix.Poll(fds[:n], pollTimeout)
		if err != nil {
			if err == unix.EINTR {
				continue
			}
			return fmt.Errorf("poll: %w", err)
		}

		if count == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
			continue
		}

		for j := range n {
			if fds[j].Revents&(unix.POLLIN|unix.POLLHUP|unix.POLLERR) == 0 {
				continue
			}
			i := idx[j]
			nr, readErr := pipes[i].file.Read(buf)
			if nr > 0 {
				pipes[i].buf.Write(buf[:nr])
				combined.Write(buf[:nr])
				if combined.Len() > outputLimit {
					_ = proc.Kill()
					return errOutputLimitExceeded
				}
			}
			if readErr != nil {
				if readErr == io.EOF {
					pipes[i].open = false
				} else {
					return fmt.Errorf("read: %w", readErr)
				}
			}
		}
	}
	return nil
}
