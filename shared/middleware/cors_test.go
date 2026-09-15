package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/rs3-market/backend/shared/config"
)

func corsEngine(cfg config.CORSConfig) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS(cfg))
	r.GET("/x", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	return r
}

func request(r *gin.Engine, method, origin string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/x", nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestCORSWildcardOrigin(t *testing.T) {
	r := corsEngine(config.CORSConfig{AllowedOrigins: []string{"*"}})
	w := request(r, http.MethodGet, "https://app.example.com")

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("allow-origin = %q, want *", got)
	}
}

// The CORS spec forbids pairing a wildcard origin with credentials; a
// browser rejects the response outright, so the origin must be echoed.
func TestCORSWildcardWithCredentialsEchoesOrigin(t *testing.T) {
	r := corsEngine(config.CORSConfig{AllowedOrigins: []string{"*"}, AllowCreds: true})
	w := request(r, http.MethodGet, "https://app.example.com")

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Errorf("allow-origin = %q, want the request origin echoed back", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("allow-credentials = %q, want true", got)
	}
	if got := w.Header().Get("Vary"); got != "Origin" {
		t.Errorf("Vary = %q, want Origin so caches do not mix origins", got)
	}
}

func TestCORSAllowlistedOrigin(t *testing.T) {
	r := corsEngine(config.CORSConfig{AllowedOrigins: []string{"https://ok.example.com"}})

	w := request(r, http.MethodGet, "https://ok.example.com")
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://ok.example.com" {
		t.Errorf("allow-origin = %q, want the allowlisted origin", got)
	}
}

func TestCORSDisallowedOriginGetsNoHeaders(t *testing.T) {
	r := corsEngine(config.CORSConfig{AllowedOrigins: []string{"https://ok.example.com"}})
	w := request(r, http.MethodGet, "https://evil.example.com")

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("allow-origin = %q, want no header for a disallowed origin", got)
	}
	// The request itself still succeeds; the browser is what blocks it.
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestCORSPreflightTerminates(t *testing.T) {
	r := corsEngine(config.CORSConfig{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST"},
		AllowedHeaders: []string{"Content-Type"},
		MaxAge:         5 * time.Minute,
	})
	w := request(r, http.MethodOptions, "https://app.example.com")

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST" {
		t.Errorf("allow-methods = %q", got)
	}
	if got := w.Header().Get("Access-Control-Max-Age"); got != "300" {
		t.Errorf("max-age = %q, want 300", got)
	}
}

func TestCORSPreflightFromDisallowedOriginStillTerminates(t *testing.T) {
	r := corsEngine(config.CORSConfig{AllowedOrigins: []string{"https://ok.example.com"}})
	w := request(r, http.MethodOptions, "https://evil.example.com")

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204; preflight must not reach an upstream", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("allow-origin = %q, want none", got)
	}
}

// Same-origin requests carry no Origin header and need no CORS headers.
func TestCORSNoOriginHeaderIsPassthrough(t *testing.T) {
	r := corsEngine(config.CORSConfig{AllowedOrigins: []string{"*"}})
	w := request(r, http.MethodGet, "")

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("allow-origin = %q, want none without an Origin header", got)
	}
	if w.Body.String() != "ok" {
		t.Error("the handler should still run")
	}
}

func TestCORSDefaultIsPermissiveWithoutCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORSDefault())
	r.GET("/x", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := request(r, http.MethodGet, "https://anywhere.example.com")
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("allow-origin = %q, want *", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Errorf("allow-credentials = %q, want none by default", got)
	}
}
