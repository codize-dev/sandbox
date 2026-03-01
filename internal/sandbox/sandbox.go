package sandbox

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"golang.org/x/sys/unix"
)

const nsjailPath = "/bin/nsjail"

type Runtime string

const (
	RuntimeNode Runtime = "node"
	RuntimeRuby Runtime = "ruby"
)

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

func ValidRuntime(rt Runtime) bool {
	_, ok := runtimes[rt]
	return ok
}

type Result struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	Output   string `json:"output"`
	ExitCode int    `json:"exit_code"`
}

func Run(rt Runtime, tmpDir, entryFile string) (Result, error) {
	tmpHome, err := os.MkdirTemp("", "sandbox-tmp-*")
	if err != nil {
		return Result{}, fmt.Errorf("failed to create tmp directory: %w", err)
	}
	defer os.RemoveAll(tmpHome)
	cfg := runtimes[rt]

	args := []string{
		"-Mo",
		"--log", "/dev/null",
		"-D", "/code",
		"-R", "/lib:/lib",
		"-R", "/usr:/usr",
	}

	if _, err := os.Stat("/lib64"); err == nil {
		args = append(args, "-R", "/lib64:/lib64")
	}

	args = append(args,
		"-R", cfg.installDir+":"+cfg.installDir,
		"-R", "/dev/null:/dev/null",
		"-R", "/dev/urandom:/dev/urandom",
		"-B", tmpDir+":/code",
		"-B", tmpHome+":/tmp",
		"-m", "none:/proc:proc:ro",
		"-s", "/proc/self/fd:/dev/fd",
		"--rlimit_as", "hard",
		"-E", "PATH="+cfg.pathEnv,
		"-E", "HOME=/tmp",
		"--",
		cfg.binaryPath,
		"/code/"+entryFile,
	)

	cmd := exec.Command(nsjailPath, args...)

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

	cmd.Stdout = stdoutW
	cmd.Stderr = stderrW

	if err := cmd.Start(); err != nil {
		_ = stdoutR.Close()
		_ = stdoutW.Close()
		_ = stderrR.Close()
		_ = stderrW.Close()
		return Result{}, fmt.Errorf("sandbox execution failed: %w", err)
	}

	// Close the parent's copy of the write ends. A pipe delivers EOF to
	// readers only when all write-end fds are closed; without this, the
	// read ends would block forever even after the child exits.
	_ = stdoutW.Close()
	_ = stderrW.Close()

	var stdoutBuf, stderrBuf, combined bytes.Buffer

	if err := drainPipes(stdoutR, stderrR, &stdoutBuf, &stderrBuf, &combined); err != nil {
		_ = cmd.Wait()
		_ = stdoutR.Close()
		_ = stderrR.Close()
		return Result{}, fmt.Errorf("sandbox execution failed: %w", err)
	}

	_ = stdoutR.Close()
	_ = stderrR.Close()

	waitErr := cmd.Wait()
	if waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			return Result{
				Stdout:   base64.StdEncoding.EncodeToString(stdoutBuf.Bytes()),
				Stderr:   base64.StdEncoding.EncodeToString(stderrBuf.Bytes()),
				Output:   base64.StdEncoding.EncodeToString(combined.Bytes()),
				ExitCode: exitErr.ExitCode(),
			}, nil
		}
		return Result{}, fmt.Errorf("sandbox execution failed: %w", waitErr)
	}

	return Result{
		Stdout:   base64.StdEncoding.EncodeToString(stdoutBuf.Bytes()),
		Stderr:   base64.StdEncoding.EncodeToString(stderrBuf.Bytes()),
		Output:   base64.StdEncoding.EncodeToString(combined.Bytes()),
		ExitCode: 0,
	}, nil
}

// drainPipes multiplexes reads from stdoutR and stderrR using poll(2),
// writing to per-stream buffers and a combined buffer. Processing in a
// single goroutine eliminates races on the combined buffer. When both
// pipes are ready simultaneously, stdout is processed first.
func drainPipes(stdoutR, stderrR *os.File, stdoutBuf, stderrBuf, combined *bytes.Buffer) error {
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

		if _, err := unix.Poll(fds[:n], -1); err != nil {
			if err == unix.EINTR {
				continue
			}
			return fmt.Errorf("poll: %w", err)
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
