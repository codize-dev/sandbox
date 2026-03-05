package sandbox

import (
	"fmt"
	"os/exec"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func Test_resolveSignal(t *testing.T) {
	t.Parallel()

	sigkill := unix.SignalName(syscall.SIGKILL)
	sighup := unix.SignalName(syscall.SIGHUP)
	sigsegv := unix.SignalName(syscall.SIGSEGV)

	tests := []struct {
		name     string
		exitCode int
		log      string
		want     *string
	}{
		{
			name:     "SIGKILL with signal log",
			exitCode: 137,
			log:      "terminated with signal: SIGKILL (9), (PIDs left: 0)",
			want:     &sigkill,
		},
		{
			name:     "SIGHUP with signal log",
			exitCode: 129,
			log:      "terminated with signal: SIGHUP (1), (PIDs left: 0)",
			want:     &sighup,
		},
		{
			name:     "SIGSEGV with signal log",
			exitCode: 139,
			log:      "terminated with signal: SIGSEGV (11), (PIDs left: 0)",
			want:     &sigsegv,
		},
		{
			name:     "exit code 137 without signal log",
			exitCode: 137,
			log:      "exited with status: 137",
			want:     nil,
		},
		{
			name:     "exit code 139 without signal log",
			exitCode: 139,
			log:      "exited with status: 139",
			want:     nil,
		},
		{
			name:     "exit code exactly 128 with signal log",
			exitCode: 128,
			log:      "terminated with signal: SIGKILL (9), (PIDs left: 0)",
			want:     nil,
		},
		{
			name:     "exit code 0 no signal",
			exitCode: 0,
			log:      "exited with status: 0",
			want:     nil,
		},
		{
			name:     "exit code 1 no signal",
			exitCode: 1,
			log:      "exited with status: 1",
			want:     nil,
		},
		{
			name:     "empty log with signal-like exit code",
			exitCode: 137,
			log:      "",
			want:     nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := resolveSignal(tc.exitCode, tc.log)
			assert.Equal(t, tc.want, got)
		})
	}
}

func Test_collectResult(t *testing.T) {
	t.Parallel()

	sigsegv := unix.SignalName(syscall.SIGSEGV)
	sigkill := unix.SignalName(syscall.SIGKILL)
	sigsys := unix.SignalName(syscall.SIGSYS)

	makeExitError := func(t *testing.T, code int) error {
		t.Helper()
		cmd := exec.Command("/bin/sh", "-c", fmt.Sprintf("exit %d", code))
		err := cmd.Run()
		if err == nil {
			t.Fatal("expected non-zero exit")
		}
		return err
	}

	tests := []struct {
		name       string
		exitCode   int
		logStr     string
		wantStatus Status
		wantSignal *string
	}{
		{
			name:       "clean exit",
			exitCode:   0,
			logStr:     "",
			wantStatus: StatusOK,
			wantSignal: nil,
		},
		{
			name:       "non-zero exit without signal",
			exitCode:   1,
			logStr:     "exited with status: 1",
			wantStatus: StatusOK,
			wantSignal: nil,
		},
		{
			name:       "signal-like exit code without nsjail log",
			exitCode:   139,
			logStr:     "exited with status: 139",
			wantStatus: StatusOK,
			wantSignal: nil,
		},
		{
			name:       "SIGSEGV with nsjail log",
			exitCode:   139,
			logStr:     "terminated with signal: SIGSEGV (11), (PIDs left: 0)",
			wantStatus: StatusSignal,
			wantSignal: &sigsegv,
		},
		{
			name:       "SIGKILL with nsjail log (non-timeout)",
			exitCode:   137,
			logStr:     "terminated with signal: SIGKILL (9), (PIDs left: 0)",
			wantStatus: StatusSignal,
			wantSignal: &sigkill,
		},
		{
			name:       "SIGSYS with nsjail log",
			exitCode:   128 + int(syscall.SIGSYS),
			logStr:     fmt.Sprintf("terminated with signal: SIGSYS (%d), (PIDs left: 0)", syscall.SIGSYS),
			wantStatus: StatusSignal,
			wantSignal: &sigsys,
		},
		{
			name:       "timeout takes precedence over signal",
			exitCode:   137,
			logStr:     "run time >= time limit\nterminated with signal: SIGKILL (9), (PIDs left: 0)",
			wantStatus: StatusTimeout,
			wantSignal: &sigkill,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			e := &execution{}

			var waitErr error
			if tc.exitCode != 0 {
				waitErr = makeExitError(t, tc.exitCode)
			}

			result, err := e.collectResult(waitErr, tc.logStr)
			require.NoError(t, err)
			assert.Equal(t, tc.wantStatus, result.Status)
			assert.Equal(t, tc.wantSignal, result.Signal)
		})
	}
}
