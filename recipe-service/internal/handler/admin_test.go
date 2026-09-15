package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type fakeScraper struct {
	ran chan struct{}
	err error
}

func newFakeScraper() *fakeScraper {
	return &fakeScraper{ran: make(chan struct{}, 1)}
}

func (f *fakeScraper) ScrapeAll() (int, error) {
	f.ran <- struct{}{}
	if f.err != nil {
		return 0, f.err
	}
	return 42, nil
}

type fakeResolver struct {
	resolved chan struct{}
}

func newFakeResolver() *fakeResolver {
	return &fakeResolver{resolved: make(chan struct{}, 3)}
}

func (f *fakeResolver) BackfillOutputsFromGEIDs() (int64, error) { return f.note() }
func (f *fakeResolver) BackfillInputItemIDs() (int64, error)     { return f.note() }
func (f *fakeResolver) BackfillInputsFromGEIDs() (int64, error)  { return f.note() }

func (f *fakeResolver) note() (int64, error) {
	select {
	case f.resolved <- struct{}{}:
	default:
	}
	return 1, nil
}

func adminServer(t *testing.T, s *fakeScraper, r *fakeResolver) *httptest.Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	e := gin.New()
	NewAdmin(s, r, zap.NewNop()).Register(e)

	srv := httptest.NewServer(e)
	t.Cleanup(srv.Close)
	return srv
}

func post(t *testing.T, srv *httptest.Server, path string) *http.Response {
	t.Helper()
	resp, err := srv.Client().Post(srv.URL+path, "", strings.NewReader(""))
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func waitFor(t *testing.T, c chan struct{}, what string) {
	t.Helper()
	select {
	case <-c:
	case <-time.After(2 * time.Second):
		t.Fatalf("%s never happened", what)
	}
}

// A manual scrape clears every input's item ID exactly as the scheduled
// one does, so it has to re-resolve them too. Triggering a scrape by
// hand and quietly wiping the catalogue's IDs is the worse of the two
// failures, because someone is watching and sees it succeed.
func TestTriggerScrapeResolvesIDsAfterwards(t *testing.T) {
	s, r := newFakeScraper(), newFakeResolver()
	resp := post(t, adminServer(t, s, r), "/internal/scrape")

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusAccepted)
	}
	waitFor(t, s.ran, "the scrape")
	waitFor(t, r.resolved, "resolution after the scrape")
}

func TestTriggerScrapeSkipsResolutionWhenTheScrapeFails(t *testing.T) {
	s, r := newFakeScraper(), newFakeResolver()
	s.err = errors.New("wiki unreachable")
	post(t, adminServer(t, s, r), "/internal/scrape")

	waitFor(t, s.ran, "the scrape")
	select {
	case <-r.resolved:
		t.Error("resolution ran after a failed scrape")
	case <-time.After(300 * time.Millisecond):
	}
}
