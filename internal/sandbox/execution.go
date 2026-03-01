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
	"time"

	"golang.org/x/sys/unix"
)

type execution struct {
	runTimeout int
	rtCfg     runtimeConfig
	tmpDir    string
	entryFile string
	tmpHome   string

	stdoutR *os.File
	stdoutW *os.File
	stderrR *os.File
	stderrW *os.File
	logR    *os.File
	logW    *os.File

	stdoutBuf bytes.Buffer
	stderrBuf bytes.Buffer
	combined  bytes.Buffer
}

func (e *execution) buildArgs() []string {
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
		"-R", e.rtCfg.installDir+":"+e.rtCfg.installDir,
		"-R", "/dev/null:/dev/null",
		"-R", "/dev/urandom:/dev/urandom",
		"-B", e.tmpDir+":/code",
		"-B", e.tmpHome+":/tmp",
		"-m", "none:/proc:proc:ro",
		"-s", "/proc/self/fd:/dev/fd",
		"--rlimit_as", "hard",
		"--time_limit", fmt.Sprintf("%d", e.runTimeout),
		"-E", "PATH="+e.rtCfg.pathEnv,
		"-E", "HOME=/tmp",
		"--",
		e.rtCfg.binaryPath,
		"/code/"+e.entryFile,
	)

	return args
}

func (e *execution) openPipes() error {
	var err error

	e.stdoutR, e.stdoutW, err = os.Pipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	e.stderrR, e.stderrW, err = os.Pipe()
	if err != nil {
		_ = e.stdoutR.Close()
		_ = e.stdoutW.Close()
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	// Pipe for capturing nsjail log output. We read this after execution
	// to detect two conditions:
	// 1. Timeout: nsjail logs "run time >= time limit" before killing the
	//    child, letting us distinguish timeouts from other SIGKILL causes.
	// 2. Signal kills: nsjail logs "terminated with signal: ..." when the
	//    child is killed by a signal (WIFSIGNALED), letting us report the
	//    signal name and distinguish genuine kills from user code that
	//    voluntarily exits with a signal-like exit code.
	e.logR, e.logW, err = os.Pipe()
	if err != nil {
		_ = e.stdoutR.Close()
		_ = e.stdoutW.Close()
		_ = e.stderrR.Close()
		_ = e.stderrW.Close()
		return fmt.Errorf("failed to create log pipe: %w", err)
	}

	return nil
}

func (e *execution) start(ctx context.Context, args []string) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, nsjailPath, args...)

	cmd.Stdout = e.stdoutW
	cmd.Stderr = e.stderrW
	cmd.ExtraFiles = []*os.File{e.logW}

	if err := cmd.Start(); err != nil {
		_ = e.stdoutR.Close()
		_ = e.stdoutW.Close()
		_ = e.stderrR.Close()
		_ = e.stderrW.Close()
		_ = e.logR.Close()
		_ = e.logW.Close()
		return nil, fmt.Errorf("sandbox execution failed: %w", err)
	}

	// Close the parent's copy of the write ends. A pipe delivers EOF to
	// readers only when all write-end fds are closed; without this, the
	// read ends would block forever even after the child exits.
	_ = e.stdoutW.Close()
	_ = e.stderrW.Close()
	_ = e.logW.Close()

	return cmd, nil
}

func (e *execution) collectResult(waitErr error) (Result, error) {
	// Read nsjail log to detect timeout and signal kills. cmd.Wait() has
	// returned, so nsjail has exited and the write end of the log pipe is
	// guaranteed to be closed.
	logData, _ := io.ReadAll(e.logR)
	_ = e.logR.Close()
	logStr := string(logData)
	timedOut := strings.Contains(logStr, "run time >= time limit")

	result := Result{
		Stdout: base64.StdEncoding.EncodeToString(e.stdoutBuf.Bytes()),
		Stderr: base64.StdEncoding.EncodeToString(e.stderrBuf.Bytes()),
		Output: base64.StdEncoding.EncodeToString(e.combined.Bytes()),
		Status: StatusOK,
	}

	if waitErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(waitErr, &exitErr) {
			return Result{}, fmt.Errorf("sandbox execution failed: %w", waitErr)
		}
		result.ExitCode = exitErr.ExitCode()
		result.Signal = resolveSignal(result.ExitCode, logStr)
		if timedOut {
			result.Status = StatusTimeout
		}
	}

	return result, nil
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
