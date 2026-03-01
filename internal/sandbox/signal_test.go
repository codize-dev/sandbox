package sandbox

import (
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/unix"
)

func TestResolveSignal(t *testing.T) {
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
