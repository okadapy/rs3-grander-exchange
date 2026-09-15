package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/rs3-market/backend/ge-price-service/internal/poller"
)

// Admin exposes an on-demand trigger for the poller, useful in dev so
// you don't have to wait for the next hourly tick.
type Admin struct {
	p   *poller.Poller
	log *zap.Logger
}

func NewAdmin(p *poller.Poller, log *zap.Logger) *Admin {
	return &Admin{p: p, log: log}
}

func (a *Admin) Register(r *gin.Engine) {
	r.POST("/internal/poll", a.trigger)
	r.DELETE("/internal/poll/lock", a.clearLock)
}

func (a *Admin) trigger(c *gin.Context) {
	go func() {
		a.p.PollOnce(c.Request.Context())
	}()
	c.JSON(http.StatusAccepted, gin.H{"status": "poll triggered"})
}

// clearLock removes the distributed lock, useful when a crashed poller
// left it stuck. Only exposed for debugging.
func (a *Admin) clearLock(c *gin.Context) {
	a.p.ClearLock()
	c.JSON(http.StatusOK, gin.H{"status": "lock cleared"})
}
