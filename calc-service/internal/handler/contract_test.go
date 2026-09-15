package handler

import (
	"net/http"
	"net/http/httptest"
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

// /calc/top is a static sibling of /calc/:itemID too. If the param route
// were registered first, "top" would be parsed as an item ID.
func TestTopRouteIsNotSwallowedByItemIDParam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	New(nil, zap.NewNop()).Register(r)

	routes := apicontract.RegisteredRoutes(r)
	if !routes["GET /calc/top"] {
		t.Error("GET /calc/top must be registered as a static route")
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
