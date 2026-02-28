package sandbox

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
)

const (
	nsjailPath = "/bin/nsjail"
	nodePath   = "/mise/installs/node/24.14.0/bin/node"
)

type Result struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	Output   string `json:"output"`
	ExitCode int    `json:"exit_code"`
}

type lockedWriter struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (lw *lockedWriter) Write(p []byte) (n int, err error) {
	lw.mu.Lock()
	defer lw.mu.Unlock()
	return lw.buf.Write(p)
}

func Run(tmpDir, entryFile string) (Result, error) {
	tmpHome, err := os.MkdirTemp("", "sandbox-tmp-*")
	if err != nil {
		return Result{}, fmt.Errorf("failed to create tmp directory: %w", err)
	}
	defer os.RemoveAll(tmpHome)

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
		"-R", "/mise/installs/node/24.14.0:/mise/installs/node/24.14.0",
		"-R", "/dev/null:/dev/null",
		"-R", "/dev/urandom:/dev/urandom",
		"-B", tmpDir+":/code",
		"-B", tmpHome+":/tmp",
		"-m", "none:/proc:proc:ro",
		"-s", "/proc/self/fd:/dev/fd",
		"--rlimit_as", "hard",
		"-E", "PATH=/mise/installs/node/24.14.0/bin",
		"-E", "HOME=/tmp",
		"--",
		nodePath,
		"/code/"+entryFile,
	)

	cmd := exec.Command(nsjailPath, args...)

	var stdoutBuf, stderrBuf bytes.Buffer
	combined := &lockedWriter{}

	cmd.Stdout = io.MultiWriter(&stdoutBuf, combined)
	cmd.Stderr = io.MultiWriter(&stderrBuf, combined)

	err = cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return Result{
				Stdout:   base64.StdEncoding.EncodeToString(stdoutBuf.Bytes()),
				Stderr:   base64.StdEncoding.EncodeToString(stderrBuf.Bytes()),
				Output:   base64.StdEncoding.EncodeToString(combined.buf.Bytes()),
				ExitCode: exitErr.ExitCode(),
			}, nil
		}
		return Result{}, fmt.Errorf("sandbox execution failed: %w", err)
	}

	return Result{
		Stdout:   base64.StdEncoding.EncodeToString(stdoutBuf.Bytes()),
		Stderr:   base64.StdEncoding.EncodeToString(stderrBuf.Bytes()),
		Output:   base64.StdEncoding.EncodeToString(combined.buf.Bytes()),
		ExitCode: 0,
	}, nil
}
