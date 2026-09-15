package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// pollTimeout bounds a manually triggered cycle. A full pass over every
// wanted item ID is ~75 batches spaced by weirdgloop.min_gap, so this is
// a ceiling for a wedged run rather than a deadline anyone should hit.
const pollTimeout = 30 * time.Minute

// poll is the part of the poller the admin endpoints drive, as an
// interface so a test can observe what they hand it.
type poll interface {
	PollOnce(context.Context)
	ClearLock()
}

// Admin exposes an on-demand trigger for the poller, useful in dev so
// you don't have to wait for the next hourly tick.
type Admin struct {
	p poll
	// ctx is the service's own context, not a request's.
	ctx context.Context
	log *zap.Logger
}

func NewAdmin(ctx context.Context, p poll, log *zap.Logger) *Admin {
	return &Admin{p: p, ctx: ctx, log: log}
}

func (a *Admin) Register(r *gin.Engine) {
	r.POST("/internal/poll", a.trigger)
	r.DELETE("/internal/poll/lock", a.clearLock)
}

func (a *Admin) trigger(c *gin.Context) {
	// The cycle deliberately does not run on the request context. Gin
	// cancels that as soon as this handler returns its 202, which it
	// does immediately — so every manual poll died while fetching the
	// item ID list and logged nothing but "context canceled". Deriving
	// from the service context instead keeps shutdown cancelling the
	// run without the response ending it.
	go func() {
		ctx, cancel := context.WithTimeout(a.ctx, pollTimeout)
		defer cancel()
		a.p.PollOnce(ctx)
	}()
	c.JSON(http.StatusAccepted, gin.H{"status": "poll triggered"})
}

// clearLock removes the distributed lock, useful when a crashed poller
// left it stuck. Only exposed for debugging.
func (a *Admin) clearLock(c *gin.Context) {
	a.p.ClearLock()
	c.JSON(http.StatusOK, gin.H{"status": "lock cleared"})
}
