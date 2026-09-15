package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// stubPoll records the context a trigger hands it and then blocks, so
// the context is inspected while the call is still in flight — exactly
// when a real cycle would be using it. Returning straight away would
// let the trigger's own deferred cancel fire first and make every
// context look dead.
type stubPoll struct {
	got     chan context.Context
	release chan struct{}
	closed  chan struct{}
}

func newStubPoll() *stubPoll {
	return &stubPoll{
		got:     make(chan context.Context, 1),
		release: make(chan struct{}),
		closed:  make(chan struct{}, 1),
	}
}

func (s *stubPoll) PollOnce(ctx context.Context) {
	s.got <- ctx
	<-s.release
}

func (s *stubPoll) ClearLock() { s.closed <- struct{}{} }

// started returns the context the trigger passed, failing the test if
// the cycle never began.
func (s *stubPoll) started(t *testing.T) context.Context {
	t.Helper()
	select {
	case ctx := <-s.got:
		return ctx
	case <-time.After(2 * time.Second):
		t.Fatal("poll was never started")
		return nil
	}
}

// adminServer serves the admin routes over a real listener.
// httptest.NewRequest alone would not do: it builds a request carrying
// context.Background(), which nothing ever cancels, so the very bug
// these tests exist for is invisible through it. Only net/http cancels
// a request context when its handler returns.
func adminServer(t *testing.T, ctx context.Context, p poll) *httptest.Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	NewAdmin(ctx, p, zap.NewNop()).Register(r)

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

func do(t *testing.T, srv *httptest.Server, method, path string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+path, strings.NewReader(""))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

// The bug this guards: the trigger used to pass the request context
// into the goroutine, and net/http cancels that the moment the 202 is
// written, so the cycle never got as far as its first HTTP call and
// logged nothing but "context canceled".
func TestTriggerPollSurvivesTheResponse(t *testing.T) {
	p := newStubPoll()
	defer close(p.release)

	srv := adminServer(t, context.Background(), p)
	resp := do(t, srv, http.MethodPost, "/internal/poll")

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusAccepted)
	}

	pollCtx := p.started(t)

	// The response is complete and its context is gone; the cycle's
	// context must outlive it.
	select {
	case <-pollCtx.Done():
		t.Fatalf("poll context done after the response: %v", pollCtx.Err())
	case <-time.After(100 * time.Millisecond):
	}
}

// Shutdown still has to reach a poll that is already running.
func TestTriggerPollCancelsWithTheService(t *testing.T) {
	p := newStubPoll()
	defer close(p.release)

	ctx, cancel := context.WithCancel(context.Background())
	srv := adminServer(t, ctx, p)
	resp := do(t, srv, http.MethodPost, "/internal/poll")

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusAccepted)
	}

	pollCtx := p.started(t)

	cancel()
	select {
	case <-pollCtx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("cancelling the service context did not reach the poll")
	}
}

func TestClearLock(t *testing.T) {
	p := newStubPoll()
	defer close(p.release)

	srv := adminServer(t, context.Background(), p)
	resp := do(t, srv, http.MethodDelete, "/internal/poll/lock")

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	select {
	case <-p.closed:
	case <-time.After(2 * time.Second):
		t.Fatal("lock was not cleared")
	}
}
