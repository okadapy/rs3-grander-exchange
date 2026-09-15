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
	NewAdmin(nil, zap.NewNop()).Register(r)

	res, err := apicontract.Check(r, "../../../openapi/ge-price-service.yaml", []string{
		// Dev-only poller controls; not routed through the gateway.
		"POST /internal/poll",
		"DELETE /internal/poll/lock",
	})
	if err != nil {
		t.Fatalf("contract check: %v", err)
	}
	if !res.OK() {
		t.Error(res.Error())
	}
}
