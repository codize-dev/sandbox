package middleware

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v5"
)

// MetricsHandler returns an Echo handler that renders Prometheus text exposition
// format for the sandbox concurrency and queue gauges.
func MetricsHandler(metrics *ConcurrencyMetrics, maxConcurrency, maxQueueSize int) echo.HandlerFunc {
	return func(c *echo.Context) error {
		body := fmt.Sprintf("# HELP sandbox_concurrency_active Number of requests currently executing.\n"+
			"# TYPE sandbox_concurrency_active gauge\n"+
			"sandbox_concurrency_active %d\n"+
			"# HELP sandbox_queue_length Number of requests waiting in queue.\n"+
			"# TYPE sandbox_queue_length gauge\n"+
			"sandbox_queue_length %d\n"+
			"# HELP sandbox_concurrency_max Configured maximum concurrent executions.\n"+
			"# TYPE sandbox_concurrency_max gauge\n"+
			"sandbox_concurrency_max %d\n"+
			"# HELP sandbox_queue_max Configured maximum queue size.\n"+
			"# TYPE sandbox_queue_max gauge\n"+
			"sandbox_queue_max %d\n",
			metrics.Active.Load(), metrics.Queued.Load(), maxConcurrency, maxQueueSize)
		c.Response().Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		return c.String(http.StatusOK, body)
	}
}
