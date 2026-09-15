package handler

import (
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
