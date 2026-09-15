package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/rs3-market/backend/shared/apicontract"
)

// Route registration mixes static segments with a wildcard at the same
// level (/recipes/ids next to /recipes/:itemID). Gin resolves this, but
// a future reshuffle could make it panic at startup — which would be a
// crash loop rather than a test failure. Registering against a real
// engine pins the behaviour.
func TestRegisterBuildsRouteTree(t *testing.T) {
	gin.SetMode(gin.TestMode)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("route registration panicked: %v", r)
		}
	}()

	r := gin.New()
	(&Handler{log: zap.NewNop()}).Register(r)

	registered := map[string]bool{}
	for _, route := range r.Routes() {
		registered[route.Method+" "+route.Path] = true
	}

	want := []string{
		"GET /health",
		"GET /recipes",
		"GET /recipes/ids",
		"GET /recipes/:itemID",
		"GET /recipes/:itemID/tree",
		"GET /search/recipes",
		"GET /search/items",
		"GET /items/limits",
		"GET /internal/all-item-ids",
		"GET /internal/input-id-stats",
		"POST /internal/backfill-inputs",
	}
	for _, route := range want {
		if !registered[route] {
			t.Errorf("route %q was not registered", route)
		}
	}
}

// The old paths moved under /search; keeping the redirects means a
// client pinned to the old URL is pointed at the new one rather than
// silently 404ing.
func TestLegacySearchPathsRedirect(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	(&Handler{log: zap.NewNop()}).Register(r)

	tests := []struct{ from, to string }{
		{"/recipes/search?q=bar", "/search/recipes?q=bar"},
		{"/items/search?q=coal", "/search/items?q=coal"},
	}
	for _, tc := range tests {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.from, nil))

		if w.Code != http.StatusMovedPermanently {
			t.Errorf("%s: status = %d, want 301", tc.from, w.Code)
		}
		if got := w.Header().Get("Location"); got != tc.to {
			t.Errorf("%s: Location = %q, want %q (query string preserved)", tc.from, got, tc.to)
		}
	}
}

func TestParseIDs(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := parseIDs(" 453, 1513 ,2363")
		if err != nil {
			t.Fatalf("parseIDs: %v", err)
		}
		want := []int64{453, 1513, 2363}
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("got %v, want %v", got, want)
			}
		}
	})

	t.Run("rejections", func(t *testing.T) {
		for _, raw := range []string{"", "  ", "abc", "453,abc", "0", "-3"} {
			if _, err := parseIDs(raw); err == nil {
				t.Errorf("parseIDs(%q) should have failed", raw)
			}
		}
	})

	t.Run("cap", func(t *testing.T) {
		tooMany := strings.TrimSuffix(strings.Repeat("1,", maxIDsPerRequest+1), ",")
		if _, err := parseIDs(tooMany); err == nil {
			t.Error("expected a cap on the number of IDs")
		}
	})
}

func TestPagination(t *testing.T) {
	tests := []struct {
		query      string
		wantLimit  int
		wantOffset int
	}{
		{"", defaultLimit, 0},
		{"limit=10&offset=20", 10, 20},
		{"limit=0", defaultLimit, 0},
		{"limit=-5", defaultLimit, 0},
		{"limit=99999", maxLimit, 0},
		{"offset=-1", defaultLimit, 0},
		{"limit=abc", defaultLimit, 0},
	}

	gin.SetMode(gin.TestMode)
	for _, tc := range tests {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, "/x?"+tc.query, nil)

		limit, offset := pagination(c)
		if limit != tc.wantLimit || offset != tc.wantOffset {
			t.Errorf("pagination(%q) = %d/%d, want %d/%d",
				tc.query, limit, offset, tc.wantLimit, tc.wantOffset)
		}
	}
}

// The generated spec must describe exactly what this service serves.
// A documented route that does not exist becomes a generated client
// method that 404s; an undocumented one is unreachable from the client.
func TestRoutesMatchOpenAPISpec(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	(&Handler{log: zap.NewNop()}).Register(r)

	res, err := apicontract.Check(r, "../../../openapi/recipe-service.yaml", []string{
		// Operational tooling, not routed through the gateway.
		"GET /internal/all-item-ids",
		"GET /internal/input-id-stats",
		"POST /internal/backfill-inputs",
		// Redirects kept for clients pinned to the pre-/search paths.
		// Documenting them would generate dead methods aimed at a 301.
		"GET /recipes/search",
		"GET /items/search",
	})
	if err != nil {
		t.Fatalf("contract check: %v", err)
	}
	if !res.OK() {
		t.Error(res.Error())
	}
}
