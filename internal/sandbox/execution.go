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

// execution manages the lifecycle of a single nsjail invocation: building
// arguments, creating pipes, starting the process, draining output, and
// collecting results.
type execution struct {
	timeout     int
	outputLimit int
	command     []string
	bindMounts  []BindMount
	env         []string
	tmpDir      string
	limits      Limits

	proc *os.Process

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
		// Load static config (execution mode, logging, cwd, environment,
		// static rlimits, and filesystem mounts). CLI flags below override
		// or append per-invocation settings.
		"-C", "/etc/nsjail/nsjail.cfg",
	}

	// Runtime-specific read-only bind mounts (e.g. the language runtime
	// install directory, Go module cache). Appended to the config mount list.
	for _, m := range e.bindMounts {
		args = append(args, "-R", m.Src+":"+m.Dst)
	}

	args = append(args,
		// Per-invocation resource limits: vary by runtime and execution step.
		"--rlimit_as", e.limits.Rlimits.AS,
		"--rlimit_fsize", e.limits.Rlimits.Fsize,
		"--rlimit_nofile", e.limits.Rlimits.Nofile,
		"--rlimit_nproc", e.limits.Rlimits.Nproc,
		"--rlimit_cpu", fmt.Sprintf("%d", e.timeout),
		"--time_limit", fmt.Sprintf("%d", e.timeout),
		"--cgroup_pids_max", e.limits.Cgroups.PidsMax,
		"--cgroup_mem_max", e.limits.Cgroups.MemMax,
		"--cgroup_mem_swap_max", e.limits.Cgroups.MemSwapMax,
		"--cgroup_cpu_ms_per_sec", e.limits.Cgroups.CpuMsPerSec,
	)

	// Runtime-specific environment variables (e.g. PATH, GOROOT).
	// HOME=/sandbox is set in the config file.
	for _, env := range e.env {
		args = append(args, "-E", env)
	}

	args = append(args, "--")
	args = append(args, e.command...)
	return args
}

func (e *execution) openPipes() (retErr error) {
	defer func() {
		if retErr != nil {
			e.closePipes()
		}
	}()

	var err error

	e.stdoutR, e.stdoutW, err = os.Pipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	e.stderrR, e.stderrW, err = os.Pipe()
	if err != nil {
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
		return fmt.Errorf("failed to create log pipe: %w", err)
	}

	return nil
}

// closePipes closes all pipe file descriptors. Nil-safe for partially
// created pipes (e.g. when openPipes fails midway).
func (e *execution) closePipes() {
	for _, f := range []*os.File{e.stdoutR, e.stdoutW, e.stderrR, e.stderrW, e.logR, e.logW} {
		if f != nil {
			_ = f.Close()
		}
	}
}

// closeWriteEnds closes the parent's copy of all pipe write ends.
// Must only be called after openPipes succeeds (all fields are non-nil).
func (e *execution) closeWriteEnds() {
	_ = e.stdoutW.Close()
	_ = e.stderrW.Close()
	_ = e.logW.Close()
}

// closeReadEnds closes all pipe read ends.
// Must only be called after openPipes succeeds (all fields are non-nil).
func (e *execution) closeReadEnds() {
	_ = e.stdoutR.Close()
	_ = e.stderrR.Close()
	_ = e.logR.Close()
}

func (e *execution) start(ctx context.Context, args []string) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, nsjailPath, args...)
	cmd.Env = append(os.Environ(), "NSJAIL_WORKING_DIR="+e.tmpDir)

	cmd.Stdout = e.stdoutW
	cmd.Stderr = e.stderrW
	cmd.ExtraFiles = []*os.File{e.logW}

	if err := cmd.Start(); err != nil {
		e.closePipes()
		return nil, fmt.Errorf("sandbox execution failed: %w", err)
	}

	e.proc = cmd.Process

	// Close the parent's copy of the write ends. A pipe delivers EOF to
	// readers only when all write-end fds are closed; without this, the
	// read ends would block forever even after the child exits.
	e.closeWriteEnds()

	return cmd, nil
}

func (e *execution) collectResult(waitErr error, logStr string) (Result, error) {
	// Detect nsjail timeout by searching its log output. This string must
	// match nsjail's actual log format (see nsjail/logs.cc).
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
		} else if result.Signal != nil {
			result.Status = StatusSignal
		}
	}

	return result, nil
}

type pipe struct {
	file *os.File
	buf  *bytes.Buffer
	open bool
}

// drainPipes multiplexes reads from e.stdoutR and e.stderrR using poll(2),
// writing to per-stream buffers and a combined buffer. Processing in a
// single goroutine eliminates races on the combined buffer. When both
// pipes are ready simultaneously, stdout is processed first. The poll
// timeout is derived from ctx's deadline so that the execution timeout
// and client disconnects are respected promptly.
func (e *execution) drainPipes(ctx context.Context) error {
	pipes := [2]pipe{
		{file: e.stdoutR, buf: &e.stdoutBuf, open: true},
		{file: e.stderrR, buf: &e.stderrBuf, open: true},
	}
	buf := make([]byte, 32*1024)

	for pipes[0].open || pipes[1].open {
		fds, idx, n := buildPollFds(pipes)

		pollTimeout, err := calcPollTimeout(ctx)
		if err != nil {
			return err
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
			if err := e.readPipe(&pipes[idx[j]], buf); err != nil {
				return err
			}
		}
	}
	return nil
}

func buildPollFds(pipes [2]pipe) (fds [2]unix.PollFd, idx [2]int, n int) {
	for i := range pipes {
		if pipes[i].open {
			fds[n] = unix.PollFd{Fd: int32(pipes[i].file.Fd()), Events: unix.POLLIN}
			idx[n] = i
			n++
		}
	}
	return fds, idx, n
}

func calcPollTimeout(ctx context.Context) (int, error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return -1, nil
	}
	ms := int(time.Until(deadline).Milliseconds())
	if ms <= 0 {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		ms = 0
	}
	return ms, nil
}

func (e *execution) readPipe(p *pipe, buf []byte) error {
	nr, err := p.file.Read(buf)
	if nr > 0 {
		p.buf.Write(buf[:nr])
		e.combined.Write(buf[:nr])
		if e.combined.Len() > e.outputLimit {
			_ = e.proc.Kill()
			return errOutputLimitExceeded
		}
	}
	if err != nil {
		if err == io.EOF {
			p.open = false
			return nil
		}
		return fmt.Errorf("read: %w", err)
	}
	return nil
}
