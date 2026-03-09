package middleware

import (
	"net/http"
	"sync/atomic"
	"time"

	"github.com/codize-dev/sandbox/internal/handler"
	"github.com/labstack/echo/v5"
)

// ConcurrencyMetrics exposes live concurrency and queue counters for external
// consumption (e.g. Prometheus /metrics endpoint).
type ConcurrencyMetrics struct {
	Active atomic.Int64
	Queued atomic.Int64
}

// ConcurrencyConfig holds parameters for the concurrency limiter middleware.
type ConcurrencyConfig struct {
	MaxConcurrency int
	MaxQueueSize   int
	QueueTimeout   time.Duration
	Metrics        *ConcurrencyMetrics
}

// ConcurrencyLimiter returns an Echo middleware that limits concurrent handler
// executions. Excess requests are queued up to MaxQueueSize. Requests that
// cannot enter the queue receive 503 (SERVER_BUSY). Requests that wait longer
// than QueueTimeout receive 503 (SERVER_BUSY).
func ConcurrencyLimiter(cfg ConcurrencyConfig) echo.MiddlewareFunc {
	sem := make(chan struct{}, cfg.MaxConcurrency)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			// Fast path: try to acquire a semaphore slot without blocking.
			select {
			case sem <- struct{}{}:
				cfg.Metrics.Active.Add(1)
				defer func() {
					<-sem
					cfg.Metrics.Active.Add(-1)
				}()
				return next(c)
			default:
			}

			// Slow path: semaphore is full — enter the queue.
			q := cfg.Metrics.Queued.Add(1)
			defer cfg.Metrics.Queued.Add(-1)

			if q > int64(cfg.MaxQueueSize) {
				return c.JSON(http.StatusServiceUnavailable, handler.ErrorResponse{
					Code:    handler.CodeServerBusy,
					Message: handler.CodeServerBusy.Message(),
				})
			}

			timer := time.NewTimer(cfg.QueueTimeout)
			defer timer.Stop()

			select {
			case sem <- struct{}{}:
				cfg.Metrics.Active.Add(1)
				defer func() {
					<-sem
					cfg.Metrics.Active.Add(-1)
				}()
				return next(c)
			case <-timer.C:
				return c.JSON(http.StatusServiceUnavailable, handler.ErrorResponse{
					Code:    handler.CodeServerBusy,
					Message: handler.CodeServerBusy.Message(),
				})
			case <-c.Request().Context().Done():
				return c.JSON(http.StatusServiceUnavailable, handler.ErrorResponse{
					Code:    handler.CodeServerBusy,
					Message: handler.CodeServerBusy.Message(),
				})
			}
		}
	}
}
