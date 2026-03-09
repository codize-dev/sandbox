package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetricsHandler(t *testing.T) {
	metrics := &ConcurrencyMetrics{}
	metrics.Active.Store(3)
	metrics.Queued.Store(7)

	e := echo.New()
	e.GET("/metrics", MetricsHandler(metrics, 10, 50))

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/plain")

	body := rec.Body.String()
	assert.Contains(t, body, "sandbox_concurrency_active 3")
	assert.Contains(t, body, "sandbox_queue_length 7")
	assert.Contains(t, body, "sandbox_concurrency_max 10")
	assert.Contains(t, body, "sandbox_queue_max 50")
}
