package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/rs3-market/backend/ge-price-service/internal/liquidity"
	"github.com/rs3-market/backend/ge-price-service/internal/service"
)

// maxIDsPerRequest bounds a single /prices/latest call. The poller feeds
// thousands of IDs into this service, so an unbounded `ids` list is an
// easy way for a client to build a query with a few thousand bind
// parameters.
const maxIDsPerRequest = 500

type Handler struct {
	svc *service.Service
	log *zap.Logger
}

func New(svc *service.Service, log *zap.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

func (h *Handler) Register(r *gin.Engine) {
	r.GET("/health", h.health)
	r.GET("/prices/latest", h.latest)
	r.GET("/prices/history/:itemID", h.history)
	r.GET("/prices/change/:itemID", h.change)
	r.GET("/prices/stats/:itemID", h.stats)
}

func (h *Handler) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) latest(c *gin.Context) {
	ids, err := parseIDs(c.Query("ids"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rows, err := h.svc.Latest(ids)
	if err != nil {
		h.log.Warn("latest prices", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": len(rows), "prices": rows})
}

func (h *Handler) history(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("itemID"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item id"})
		return
	}
	now := time.Now().UTC()
	from := parseTime(c.Query("from"), now.Add(-7*24*time.Hour))
	to := parseTime(c.Query("to"), now)
	if !from.Before(to) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "from must be before to"})
		return
	}
	rows, err := h.svc.History(id, from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"item_id": id,
		"from":    from,
		"to":      to,
		"count":   len(rows),
		"history": rows,
	})
}

func (h *Handler) change(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("itemID"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item id"})
		return
	}
	window, err := parseWindow(c.Query("window"), 24*time.Hour)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	res, err := h.svc.ChangeOver(id, window, time.Now().UTC())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) stats(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("itemID"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item id"})
		return
	}
	window, err := parseWindow(c.Query("window"), liquidity.WindowFor)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	st, err := h.svc.Stats(c.Request.Context(), id, window, time.Now().UTC())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, st)
}

// ---------- helpers ----------

// parseIDs splits a comma-separated ID list. A malformed entry is an
// error rather than a silent skip: quietly dropping IDs makes a typo
// look like "this item has no price data".
func parseIDs(raw string) ([]int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errBadRequest("ids required")
	}
	parts := strings.Split(raw, ",")
	if len(parts) > maxIDsPerRequest {
		return nil, errBadRequest("too many ids (max " + strconv.Itoa(maxIDsPerRequest) + ")")
	}
	ids := make([]int64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		id, err := strconv.ParseInt(p, 10, 64)
		if err != nil || id <= 0 {
			return nil, errBadRequest("invalid item id " + strconv.Quote(p))
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, errBadRequest("ids required")
	}
	return ids, nil
}

// parseWindow accepts a Go duration ("24h", "7d" is not valid Go so we
// also accept a plain number of hours).
func parseWindow(s string, def time.Duration) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return def, nil
	}
	if d, err := time.ParseDuration(s); err == nil {
		if d <= 0 {
			return 0, errBadRequest("window must be positive")
		}
		return d, nil
	}
	if hours, err := strconv.ParseFloat(s, 64); err == nil && hours > 0 {
		return time.Duration(hours * float64(time.Hour)), nil
	}
	return 0, errBadRequest("invalid window " + strconv.Quote(s))
}

func parseTime(s string, def time.Time) time.Time {
	if s == "" {
		return def
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return def
	}
	return t.UTC()
}

type badRequest string

func (e badRequest) Error() string { return string(e) }

func errBadRequest(msg string) error { return badRequest(msg) }
