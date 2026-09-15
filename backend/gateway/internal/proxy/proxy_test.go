package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/rs3-market/backend/shared/config"
	"github.com/rs3-market/backend/shared/middleware"
)

func testRoutes(t *testing.T) []*Route {
	t.Helper()
	routes, err := Build([]config.RouteConfig{
		{Prefix: "/recipes", Target: "http://recipe:8082"},
		{Prefix: "/search", Target: "http://recipe:8082"},
		{Prefix: "/items", Target: "http://recipe:8082"},
		{Prefix: "/prices", Target: "http://price:8083"},
		{Prefix: "/ws", Target: "http://realtime:8085", WebSocket: true},
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return routes
}

func TestMatchPrefixBoundaries(t *testing.T) {
	routes := testRoutes(t)

	tests := []struct {
		path string
		want string // "" means no match
	}{
		{"/recipes", "/recipes"},
		{"/recipes/", "/recipes"},
		{"/recipes/4151", "/recipes"},
		{"/recipes/4151/tree", "/recipes"},
		{"/prices/latest", "/prices"},
		{"/ws", "/ws"},

		// The search endpoints live under their own prefixes; without
		// routes for them the gateway 404s every search the frontend
		// makes.
		{"/search/recipes", "/search"},
		{"/search/items", "/search"},
		{"/items/limits", "/items"},

		// A longer path segment must not match a shorter prefix.
		{"/recipes-archive", ""},
		{"/recipesfoo", ""},
		{"/pricesX", ""},
		{"/unknown", ""},
		{"/", ""},
	}

	for _, tc := range tests {
		got := Match(routes, tc.path)
		switch {
		case tc.want == "" && got != nil:
			t.Errorf("%s matched %q, want no match", tc.path, got.Prefix)
		case tc.want != "" && got == nil:
			t.Errorf("%s did not match, want %q", tc.path, tc.want)
		case tc.want != "" && got != nil && got.Prefix != tc.want:
			t.Errorf("%s matched %q, want %q", tc.path, got.Prefix, tc.want)
		}
	}
}

func TestMatchIsFirstWins(t *testing.T) {
	routes, err := Build([]config.RouteConfig{
		{Prefix: "/a", Target: "http://first:1"},
		{Prefix: "/a", Target: "http://second:2"},
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got := Match(routes, "/a/thing"); got.Target.Host != "first:1" {
		t.Errorf("matched %q, want the first declared route", got.Target.Host)
	}
}

func TestBuildRejectsBadTarget(t *testing.T) {
	_, err := Build([]config.RouteConfig{{Prefix: "/x", Target: "://nonsense"}}, zap.NewNop())
	if err == nil {
		t.Fatal("expected an error for an unparseable target URL")
	}
}

// gatewayFor serves the proxy over a real listener.
//
// httptest.ResponseRecorder cannot be used here: gin's ResponseWriter
// advertises http.CloseNotifier and panics when the writer beneath it
// does not implement it, which ReverseProxy triggers. A real server is
// also a more faithful test of a component whose whole job is to move
// bytes between two connections.
func gatewayFor(t *testing.T, cfgs []config.RouteConfig) *httptest.Server {
	t.Helper()
	routes, err := Build(cfgs, zap.NewNop())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.NoRoute(Handler(routes, zap.NewNop()))

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

func get(t *testing.T, srv *httptest.Server, path string) (int, string) {
	t.Helper()
	resp, err := srv.Client().Get(srv.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func TestHandlerProxiesToUpstream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("upstream saw " + r.URL.Path + "?" + r.URL.RawQuery))
	}))
	defer upstream.Close()

	gw := gatewayFor(t, []config.RouteConfig{{Prefix: "/prices", Target: upstream.URL}})

	status, body := get(t, gw, "/prices/latest?ids=1")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if want := "upstream saw /prices/latest?ids=1"; body != want {
		t.Errorf("body = %q, want %q", body, want)
	}
}

func TestHandlerUnmatchedPathReturns404(t *testing.T) {
	gw := gatewayFor(t, []config.RouteConfig{{Prefix: "/prices", Target: "http://127.0.0.1:1"}})

	if status, _ := get(t, gw, "/nope"); status != http.StatusNotFound {
		t.Errorf("status = %d, want 404", status)
	}
}

func TestHandlerUnreachableUpstreamReturns502(t *testing.T) {
	// Port 1 on localhost refuses connections.
	gw := gatewayFor(t, []config.RouteConfig{{Prefix: "/dead", Target: "http://127.0.0.1:1"}})

	status, body := get(t, gw, "/dead/thing")
	if status != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", status)
	}
	if !strings.Contains(body, "upstream unavailable") {
		t.Errorf("body = %q, want an explanatory JSON error", body)
	}
}

func TestStripPrefix(t *testing.T) {
	paths := make(chan string, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths <- r.URL.Path
	}))
	defer upstream.Close()

	gw := gatewayFor(t, []config.RouteConfig{
		{Prefix: "/api", Target: upstream.URL, StripPrefix: true},
	})
	get(t, gw, "/api/health")

	if got := <-paths; got != "/health" {
		t.Errorf("upstream path = %q, want /health", got)
	}
}

func TestForwardedHostHeaderIsSet(t *testing.T) {
	hosts := make(chan string, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hosts <- r.Header.Get("X-Forwarded-Host")
	}))
	defer upstream.Close()

	gw := gatewayFor(t, []config.RouteConfig{{Prefix: "/x", Target: upstream.URL}})
	get(t, gw, "/x/thing")

	if got := <-hosts; got == "" {
		t.Error("X-Forwarded-Host should carry the original host")
	}
}

// Every service behind the gateway also answers on its own port during
// development, so each one applies its own permissive CORS middleware.
// Without stripping, a browser receives "Access-Control-Allow-Origin"
// twice and refuses the response outright — the gateway's own header is
// correct, and the duplicate is what breaks it.
func TestProxyStripsUpstreamCORSHeaders(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
		w.Header().Set("Access-Control-Max-Age", "600")
		w.Header().Set("X-Upstream", "kept")
		_, _ = w.Write([]byte("ok"))
	}))
	defer upstream.Close()

	routes, err := Build([]config.RouteConfig{{Prefix: "/prices", Target: upstream.URL}}, zap.NewNop())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.CORS(config.CORSConfig{AllowedOrigins: []string{"*"}}))
	r.NoRoute(Handler(routes, zap.NewNop()))
	gw := httptest.NewServer(r)
	defer gw.Close()

	req, _ := http.NewRequest(http.MethodGet, gw.URL+"/prices/latest", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	resp, err := gw.Client().Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	for _, h := range []string{
		"Access-Control-Allow-Origin",
		"Access-Control-Allow-Methods",
		"Access-Control-Allow-Headers",
		"Access-Control-Expose-Headers",
		"Access-Control-Max-Age",
	} {
		if got := resp.Header.Values(h); len(got) != 1 {
			t.Errorf("%s = %v (%d values), want exactly one — the gateway's", h, got, len(got))
		}
	}
	if got := resp.Header.Get("X-Upstream"); got != "kept" {
		t.Errorf("X-Upstream = %q, want the upstream's own headers left alone", got)
	}
}
