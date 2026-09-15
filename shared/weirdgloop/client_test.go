package weirdgloop

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rs3-market/backend/shared/config"
)

func newTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	return New(config.WeirdGloopConf{
		BaseURL:     srv.URL,
		UserAgent:   "test-agent",
		HTTPTimeout: 5 * time.Second,
		MinGap:      0,
	})
}

func TestLatestItemUnmarshalTimestampForms(t *testing.T) {
	want := time.Date(2026, 9, 15, 7, 15, 41, 0, time.UTC)

	tests := []struct {
		name string
		body string
		ts   time.Time
	}{
		{"iso8601 with millis", `{"price":234,"volume":7,"timestamp":"2026-09-15T07:15:41.000Z"}`, want},
		{"rfc3339", `{"price":234,"volume":7,"timestamp":"2026-09-15T07:15:41Z"}`, want},
		{"unix seconds", `{"price":234,"volume":7,"timestamp":1789456541}`, want},
		{"space separated", `{"price":234,"volume":7,"timestamp":"2026-09-15 07:15:41"}`, want},
		{"null timestamp", `{"price":234,"volume":7,"timestamp":null}`, time.Time{}},
		{"absent timestamp", `{"price":234,"volume":7}`, time.Time{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var item LatestItem
			if err := json.Unmarshal([]byte(tc.body), &item); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if !item.Timestamp.Equal(tc.ts) {
				t.Errorf("timestamp = %v, want %v", item.Timestamp, tc.ts)
			}
			if item.Price != 234 || item.Volume != 7 {
				t.Errorf("price/volume = %d/%d, want 234/7", item.Price, item.Volume)
			}
		})
	}
}

func TestLatestItemUnmarshalStringNumbers(t *testing.T) {
	var item LatestItem
	body := `{"price":"79472","volume":"563","timestamp":"2026-09-15T07:15:41.000Z"}`
	if err := json.Unmarshal([]byte(body), &item); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if item.Price != 79472 {
		t.Errorf("price = %d, want 79472", item.Price)
	}
	if item.Volume != 563 {
		t.Errorf("volume = %d, want 563", item.Volume)
	}
}

func TestLatestItemUnmarshalRejectsGarbage(t *testing.T) {
	var item LatestItem
	err := json.Unmarshal([]byte(`{"price":{"nested":1},"volume":0}`), &item)
	if err == nil {
		t.Fatal("expected an error for a non-scalar price, got nil")
	}
	if !strings.Contains(err.Error(), "price") {
		t.Errorf("error should name the offending field, got %q", err)
	}
}

func TestDecodeLatestShapes(t *testing.T) {
	t.Run("direct map", func(t *testing.T) {
		body := []byte(`{"4151":{"id":"4151","price":79472,"volume":563,"timestamp":1789456541}}`)
		got, err := decodeLatest(body)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got[4151].Price != 79472 {
			t.Errorf("price = %d, want 79472", got[4151].Price)
		}
	})

	t.Run("items envelope", func(t *testing.T) {
		body := []byte(`{"success":true,"items":{"2":{"price":732,"volume":3110238,"timestamp":1789456541}}}`)
		got, err := decodeLatest(body)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got[2].Volume != 3110238 {
			t.Errorf("volume = %d, want 3110238", got[2].Volume)
		}
	})

	t.Run("non-numeric keys are skipped", func(t *testing.T) {
		body := []byte(`{"4151":{"price":1,"volume":0,"timestamp":1},"notanid":{"price":2,"volume":0,"timestamp":1}}`)
		got, err := decodeLatest(body)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(got) != 1 {
			t.Errorf("got %d entries, want 1", len(got))
		}
	})

	t.Run("unrecognised shape errors", func(t *testing.T) {
		if _, err := decodeLatest([]byte(`["not", "an", "object"]`)); err == nil {
			t.Fatal("expected an error for an unrecognised shape")
		}
	})
}

// A batch containing only untradeable items is a normal outcome, not a
// failure: treating it as an error would abort the whole poll cycle and
// lose the prices we did fetch.
func TestDecodeLatestNoResultsIsEmptyNotError(t *testing.T) {
	for _, msg := range []string{"No results returned", "no results", "EMPTY RESULT"} {
		body := []byte(`{"success":false,"error":"` + msg + `"}`)
		got, err := decodeLatest(body)
		if err != nil {
			t.Errorf("%q: unexpected error %v", msg, err)
		}
		if len(got) != 0 {
			t.Errorf("%q: got %d entries, want 0", msg, len(got))
		}
	}
}

func TestDecodeLatestRealErrorPropagates(t *testing.T) {
	body := []byte(`{"success":false,"error":"rate limited"}`)
	if _, err := decodeLatest(body); err == nil {
		t.Fatal("expected a real upstream error to propagate")
	}
}

// Weirdgloop rejects percent-encoded pipes, so the query string must be
// built by hand rather than through url.Values.
func TestLatestBatchSendsUnescapedPipes(t *testing.T) {
	var gotRawQuery, gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRawQuery = r.URL.RawQuery
		gotUA = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(`{"4151":{"price":1,"volume":2,"timestamp":1}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	if _, err := c.LatestBatch([]int64{4151, 4152, 2}); err != nil {
		t.Fatalf("LatestBatch: %v", err)
	}
	if want := "id=4151|4152|2"; gotRawQuery != want {
		t.Errorf("query = %q, want %q", gotRawQuery, want)
	}
	if strings.Contains(gotRawQuery, "%7C") {
		t.Error("pipes must not be percent-encoded")
	}
	if gotUA != "test-agent" {
		t.Errorf("user agent = %q, want test-agent", gotUA)
	}
}

func TestLatestBatchEmptyMakesNoRequest(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()

	got, err := newTestClient(t, srv).LatestBatch(nil)
	if err != nil {
		t.Fatalf("LatestBatch: %v", err)
	}
	if called {
		t.Error("an empty ID list should not reach the network")
	}
	if len(got) != 0 {
		t.Errorf("got %d entries, want 0", len(got))
	}
}

func TestDoGETSurfacesHTTPErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("slow down"))
	}))
	defer srv.Close()

	_, err := newTestClient(t, srv).LatestBatch([]int64{1})
	if err == nil {
		t.Fatal("expected an error for a 429 response")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("error should carry the status code, got %q", err)
	}
}

func TestHistoryShapes(t *testing.T) {
	t.Run("timestamp keyed map", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"1789456541":100,"1789542941":110}`))
		}))
		defer srv.Close()

		got, err := newTestClient(t, srv).History(4151)
		if err != nil {
			t.Fatalf("History: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("got %d points, want 2", len(got))
		}
	})

	t.Run("unknown shape errors", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`"a bare string"`))
		}))
		defer srv.Close()

		if _, err := newTestClient(t, srv).History(4151); err == nil {
			t.Fatal("expected an error for an unknown history shape")
		}
	})
}

func TestThrottleSpacesCalls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"1":{"price":1,"volume":0,"timestamp":1}}`))
	}))
	defer srv.Close()

	c := New(config.WeirdGloopConf{
		BaseURL:     srv.URL,
		HTTPTimeout: 5 * time.Second,
		MinGap:      120 * time.Millisecond,
	})

	start := time.Now()
	for i := 0; i < 3; i++ {
		if _, err := c.LatestBatch([]int64{1}); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	// Three calls means two enforced gaps.
	if elapsed := time.Since(start); elapsed < 240*time.Millisecond {
		t.Errorf("3 calls took %v, want at least 240ms of throttling", elapsed)
	}
}
