package middleware

import (
	"net/http"
	"sync/atomic"
	"time"

	"github.com/codize-dev/sandbox/internal/handler"
	"github.com/labstack/echo/v5"
)

// ConcurrencyConfig holds parameters for the concurrency limiter middleware.
type ConcurrencyConfig struct {
	MaxConcurrency int
	MaxQueueSize   int
	QueueTimeout   time.Duration
}

// ConcurrencyLimiter returns an Echo middleware that limits concurrent handler
// executions. Excess requests are queued up to MaxQueueSize. Requests that
// cannot enter the queue receive 503 (QUEUE_FULL). Requests that wait longer
// than QueueTimeout receive 503 (QUEUE_TIMEOUT).
func ConcurrencyLimiter(cfg ConcurrencyConfig) echo.MiddlewareFunc {
	sem := make(chan struct{}, cfg.MaxConcurrency)
	var queued atomic.Int64

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			// Fast path: try to acquire a semaphore slot without blocking.
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
				return next(c)
			default:
			}

			// Slow path: semaphore is full — enter the queue.
			q := queued.Add(1)
			defer queued.Add(-1)

			if q > int64(cfg.MaxQueueSize) {
				return c.JSON(http.StatusServiceUnavailable, handler.ErrorResponse{
					Code:    handler.CodeQueueFull,
					Message: handler.CodeQueueFull.Message(),
				})
			}

			timer := time.NewTimer(cfg.QueueTimeout)
			defer timer.Stop()

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
				return next(c)
			case <-timer.C:
				return c.JSON(http.StatusServiceUnavailable, handler.ErrorResponse{
					Code:    handler.CodeQueueTimeout,
					Message: handler.CodeQueueTimeout.Message(),
				})
			case <-c.Request().Context().Done():
				return c.JSON(http.StatusServiceUnavailable, handler.ErrorResponse{
					Code:    handler.CodeQueueTimeout,
					Message: handler.CodeQueueTimeout.Message(),
				})
			}
		}
	}
}
