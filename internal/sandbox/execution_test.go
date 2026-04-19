package sandbox

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_execution_elapsedMs(t *testing.T) {
	t.Parallel()

	t.Run("zero startTime returns 0", func(t *testing.T) {
		t.Parallel()
		e := &execution{}
		assert.Equal(t, int64(0), e.elapsedMs())
	})

	t.Run("non-zero startTime returns elapsed ms", func(t *testing.T) {
		t.Parallel()
		e := &execution{startTime: time.Now().Add(-1500 * time.Millisecond)}
		got := e.elapsedMs()
		assert.GreaterOrEqual(t, got, int64(1500))
		assert.Less(t, got, int64(5000), "elapsed should be close to 1500ms plus test scheduling slack")
	})
}

// Test_execution_collectResult_propagatesDurationMs locks in the contract
// that collectResult writes its durationMs argument into Result.DurationMs.
// Without this test the E2E regex /^[0-9]+$/ would silently accept a
// regression that leaves DurationMs at its zero value.
func Test_execution_collectResult_propagatesDurationMs(t *testing.T) {
	t.Parallel()

	e := &execution{}
	const want = int64(1234)
	result, err := e.collectResult(nil, "", want)
	require.NoError(t, err)
	assert.Equal(t, want, result.DurationMs)
}

// Test_newOutputLimitResult_propagatesDurationMs mirrors the collectResult
// propagation test for the OUTPUT_LIMIT_EXCEEDED assembly path in
// Runner.exec, which bypasses collectResult. Without this the E2E regex
// /^[0-9]+$/ would silently accept a regression that leaves DurationMs
// at its zero value on output-limit kills.
func Test_newOutputLimitResult_propagatesDurationMs(t *testing.T) {
	t.Parallel()

	const want = int64(5678)
	result := newOutputLimitResult(want)
	assert.Equal(t, want, result.DurationMs)
	assert.Equal(t, StatusOutputLimitExceeded, result.Status)
	assert.Equal(t, 137, result.ExitCode)
}
