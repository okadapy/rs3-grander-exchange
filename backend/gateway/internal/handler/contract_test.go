package handler

import (
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/rs3-market/backend/shared/apicontract"
	"github.com/rs3-market/backend/shared/config"
)

func TestRoutesMatchOpenAPISpec(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	NewHealth(&config.Config{}, zap.NewNop()).Register(r)
	Swagger(r, "../../../openapi")

	res, err := apicontract.Check(r, "../../../openapi/gateway.yaml", []string{
		// Documentation UI, not part of the generated client surface.
		"GET /swagger",
		"GET /swagger/",
		"GET /swagger/index.html",
		"GET /swagger/spec/{file}",
	})
	if err != nil {
		t.Fatalf("contract check: %v", err)
	}
	if !res.OK() {
		t.Error(res.Error())
	}
}

// The gateway's own routes are registered before the proxy fallback, so
// /health answers locally rather than being forwarded upstream.
func TestGatewayOwnsHealthRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	NewHealth(&config.Config{}, zap.NewNop()).Register(r)

	routes := apicontract.RegisteredRoutes(r)
	for _, want := range []string{"GET /health", "GET /health/all"} {
		if !routes[want] {
			t.Errorf("%s should be served by the gateway itself", want)
		}
	}
}
