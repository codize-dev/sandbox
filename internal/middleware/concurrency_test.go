package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/codize-dev/sandbox/internal/handler"
	"github.com/labstack/echo/v5"
)

func setupEcho(cfg ConcurrencyConfig, gate chan struct{}) (*echo.Echo, echo.MiddlewareFunc) {
	e := echo.New()
	mw := ConcurrencyLimiter(cfg)

	h := func(c *echo.Context) error {
		if gate != nil {
			<-gate // block until gate is closed or a value is sent
		}
		return c.String(http.StatusOK, "ok")
	}

	e.POST("/v1/run", h, mw)
	return e, mw
}

func doRequest(e *echo.Echo) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/run", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func doRequestWithContext(e *echo.Echo, ctx context.Context) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/run", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestConcurrencyLimiter_UnderLimit(t *testing.T) {
	e, _ := setupEcho(ConcurrencyConfig{
		MaxConcurrency: 2,
		MaxQueueSize:   5,
		QueueTimeout:   5 * time.Second,
	}, nil)

	rec := doRequest(e)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestConcurrencyLimiter_QueueAndSucceed(t *testing.T) {
	gate := make(chan struct{})
	e, _ := setupEcho(ConcurrencyConfig{
		MaxConcurrency: 1,
		MaxQueueSize:   5,
		QueueTimeout:   5 * time.Second,
	}, gate)

	// Fill the semaphore with a blocking request.
	var wg sync.WaitGroup
	wg.Add(1)
	var blockedRec *httptest.ResponseRecorder
	go func() {
		defer wg.Done()
		blockedRec = doRequest(e)
	}()

	// Wait for the blocking request to acquire the semaphore.
	time.Sleep(50 * time.Millisecond)

	// Send a queued request.
	var wg2 sync.WaitGroup
	wg2.Add(1)
	var queuedRec *httptest.ResponseRecorder
	go func() {
		defer wg2.Done()
		queuedRec = doRequest(e)
	}()

	// Wait for the queued request to enter the queue.
	time.Sleep(50 * time.Millisecond)

	// Release the gate — both requests should complete.
	close(gate)
	wg.Wait()
	wg2.Wait()

	if blockedRec.Code != http.StatusOK {
		t.Fatalf("blocked request: expected 200, got %d", blockedRec.Code)
	}
	if queuedRec.Code != http.StatusOK {
		t.Fatalf("queued request: expected 200, got %d", queuedRec.Code)
	}
}

func TestConcurrencyLimiter_QueueFull(t *testing.T) {
	gate := make(chan struct{})
	e, _ := setupEcho(ConcurrencyConfig{
		MaxConcurrency: 1,
		MaxQueueSize:   1,
		QueueTimeout:   5 * time.Second,
	}, gate)

	// Fill the semaphore.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		doRequest(e)
	}()
	time.Sleep(50 * time.Millisecond)

	// Fill the queue (1 slot).
	wg.Add(1)
	go func() {
		defer wg.Done()
		doRequest(e)
	}()
	time.Sleep(50 * time.Millisecond)

	// This request should be rejected — queue is full.
	rec := doRequest(e)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}

	var errResp handler.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if errResp.Code != handler.CodeServerBusy {
		t.Fatalf("expected SERVER_BUSY, got %s", errResp.Code)
	}

	// Cleanup: release gate and wait.
	close(gate)
	wg.Wait()
}

func TestConcurrencyLimiter_QueueTimeout(t *testing.T) {
	gate := make(chan struct{})
	e, _ := setupEcho(ConcurrencyConfig{
		MaxConcurrency: 1,
		MaxQueueSize:   5,
		QueueTimeout:   100 * time.Millisecond,
	}, gate)

	// Fill the semaphore.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		doRequest(e)
	}()
	time.Sleep(50 * time.Millisecond)

	// This request should time out in the queue.
	rec := doRequest(e)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}

	var errResp handler.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if errResp.Code != handler.CodeServerBusy {
		t.Fatalf("expected SERVER_BUSY, got %s", errResp.Code)
	}

	// Cleanup.
	close(gate)
	wg.Wait()
}

func TestConcurrencyLimiter_ClientCancel(t *testing.T) {
	gate := make(chan struct{})
	e, _ := setupEcho(ConcurrencyConfig{
		MaxConcurrency: 1,
		MaxQueueSize:   5,
		QueueTimeout:   10 * time.Second,
	}, gate)

	// Fill the semaphore.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		doRequest(e)
	}()
	time.Sleep(50 * time.Millisecond)

	// Send a request with a cancellable context.
	ctx, cancel := context.WithCancel(context.Background())
	var wg2 sync.WaitGroup
	wg2.Add(1)
	var cancelledRec *httptest.ResponseRecorder
	go func() {
		defer wg2.Done()
		cancelledRec = doRequestWithContext(e, ctx)
	}()

	// Wait for request to enter the queue, then cancel.
	time.Sleep(50 * time.Millisecond)
	cancel()
	wg2.Wait()

	if cancelledRec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", cancelledRec.Code)
	}

	var errResp handler.ErrorResponse
	if err := json.Unmarshal(cancelledRec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if errResp.Code != handler.CodeServerBusy {
		t.Fatalf("expected SERVER_BUSY, got %s", errResp.Code)
	}

	// Cleanup.
	close(gate)
	wg.Wait()
}
