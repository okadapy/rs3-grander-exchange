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

func TestRoutesMatchOpenAPISpec(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	New(nil, zap.NewNop()).Register(r)

	res, err := apicontract.Check(r, "../../../openapi/calc-service.yaml", nil)
	if err != nil {
		t.Fatalf("contract check: %v", err)
	}
	if !res.OK() {
		t.Error(res.Error())
	}
}

// /calc/batch is a static sibling of /calc/:itemID. If the param route
// were registered first, "batch" would be parsed as an item ID.
func TestBatchRouteIsNotSwallowedByItemIDParam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	New(nil, zap.NewNop()).Register(r)

	routes := apicontract.RegisteredRoutes(r)
	if !routes["GET /calc/batch"] {
		t.Error("GET /calc/batch must be registered as a static route")
	}
	if !routes["GET /calc/{itemID}"] {
		t.Error("GET /calc/{itemID} must be registered")
	}
}

// A request to /calc/top must reach the ranking handler rather than
// /calc/:itemID with itemID="top". The error body is what makes this
// load-bearing: both handlers reject an empty query with a 400, but
// only h.top names the metric, while h.calc says "invalid item id".
//
// This is not a test of registration order. Gin 1.10 prefers the static
// segment whichever way round the two are registered, so an assertion
// about order would hold with the order reversed and prove nothing.
// Registering static-first stays a convention of this file; what is
// verified here is only where the request actually lands.
func TestTopRequestReachesTheRankingHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	New(nil, zap.NewNop()).Register(r)

	req := httptest.NewRequest(http.MethodGet, "/calc/top", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("GET /calc/top = %d, want 400 from the ranking handler", w.Code)
	}
	if !strings.Contains(w.Body.String(), "metric") {
		t.Errorf("error body = %s; want the ranking handler's complaint about metric, "+
			"not /calc/:itemID parsing \"top\" as an item id", w.Body.String())
	}
}

// metric is required and has no default: asking for a ranking without
// saying of what is a mistake worth reporting, not a silent fallback.
// Neither case reaches the service, so a nil svc is safe here.
func TestTopRequiresAKnownMetric(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	New(nil, zap.NewNop()).Register(r)

	cases := []string{
		"/calc/top",
		"/calc/top?metric=",
		"/calc/top?metric=profit",
	}
	for _, path := range cases {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("GET %s = %d, want 400", path, w.Code)
		}
	}
}
