package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/rs3-market/backend/shared/config"
)

func limitedEngine(cfg config.RateLimitConfig) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RateLimit(cfg))
	r.GET("/x", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	return r
}

func call(r *gin.Engine, ip string, headers map[string]string) int {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = ip + ":12345"
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

func TestRateLimitAllowsUpToBurst(t *testing.T) {
	r := limitedEngine(config.RateLimitConfig{RequestsPerSecond: 0.001, Burst: 3})

	for i := 1; i <= 3; i++ {
		if code := call(r, "10.0.0.1", nil); code != http.StatusOK {
			t.Fatalf("request %d: status %d, want 200 within burst", i, code)
		}
	}
	if code := call(r, "10.0.0.1", nil); code != http.StatusTooManyRequests {
		t.Errorf("request 4: status %d, want 429 past burst", code)
	}
}

func TestRateLimitIsPerClient(t *testing.T) {
	r := limitedEngine(config.RateLimitConfig{RequestsPerSecond: 0.001, Burst: 1})

	if code := call(r, "10.0.0.1", nil); code != http.StatusOK {
		t.Fatalf("first client: status %d, want 200", code)
	}
	if code := call(r, "10.0.0.1", nil); code != http.StatusTooManyRequests {
		t.Fatalf("first client again: status %d, want 429", code)
	}
	// A second client must not inherit the first client's exhausted bucket.
	if code := call(r, "10.0.0.2", nil); code != http.StatusOK {
		t.Errorf("second client: status %d, want 200", code)
	}
}

func TestRateLimitRefillsOverTime(t *testing.T) {
	r := limitedEngine(config.RateLimitConfig{RequestsPerSecond: 100, Burst: 1})

	if code := call(r, "10.0.0.3", nil); code != http.StatusOK {
		t.Fatalf("status %d, want 200", code)
	}
	if code := call(r, "10.0.0.3", nil); code != http.StatusTooManyRequests {
		t.Fatalf("status %d, want 429 immediately after", code)
	}

	// At 100 rps a token is back within ~10ms.
	time.Sleep(50 * time.Millisecond)
	if code := call(r, "10.0.0.3", nil); code != http.StatusOK {
		t.Errorf("status %d, want 200 after the bucket refilled", code)
	}
}

// WebSocket connections are long-lived and rate-limited at the message
// level by the realtime service; counting the upgrade against a
// per-second HTTP budget would drop reconnect storms on the floor.
func TestRateLimitSkipsWebSocketUpgrades(t *testing.T) {
	r := limitedEngine(config.RateLimitConfig{RequestsPerSecond: 0.001, Burst: 1})
	upgrade := map[string]string{"Upgrade": "websocket"}

	for i := 0; i < 5; i++ {
		if code := call(r, "10.0.0.4", upgrade); code != http.StatusOK {
			t.Fatalf("upgrade %d: status %d, want 200", i, code)
		}
	}
}

func TestRateLimitDefaultsWhenUnconfigured(t *testing.T) {
	r := limitedEngine(config.RateLimitConfig{})
	if code := call(r, "10.0.0.5", nil); code != http.StatusOK {
		t.Errorf("status %d, want 200 under the built-in defaults", code)
	}
}

func TestRateLimitSetsRetryAfter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RateLimit(config.RateLimitConfig{RequestsPerSecond: 0.001, Burst: 1}))
	r.GET("/x", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "10.0.0.6:1"
	r.ServeHTTP(httptest.NewRecorder(), req)

	req2 := httptest.NewRequest(http.MethodGet, "/x", nil)
	req2.RemoteAddr = "10.0.0.6:1"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req2)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("a 429 should tell the client when to retry")
	}
}
