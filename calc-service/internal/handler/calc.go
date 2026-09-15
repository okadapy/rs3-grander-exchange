package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/rs3-market/backend/calc-service/internal/service"
	"github.com/rs3-market/backend/shared/rates"
)

// maxBatchIDs bounds /calc/batch. Each ID fans out into a recipe-tree
// fetch, a price lookup and a buy-limit lookup, so an unbounded list is
// an easy way to turn one request into thousands of upstream calls.
const maxBatchIDs = 50

type Handler struct {
	svc *service.Service
	log *zap.Logger
}

func New(svc *service.Service, log *zap.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

func (h *Handler) Register(r *gin.Engine) {
	r.GET("/health", h.health)
	// Static before param so "batch" is never read as an item ID.
	r.GET("/calc/batch", h.batch)
	r.GET("/calc/:itemID", h.calc)
}

func (h *Handler) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) calc(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("itemID"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item id"})
		return
	}
	opts, err := parseOpts(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	res, err := h.svc.Calculate(c.Request.Context(), id, opts)
	if err != nil {
		h.log.Warn("calc failed", zap.Int64("item_id", id), zap.Error(err))
		// No producible path is a fact about the catalogue, not a server
		// fault — a 500 here would make the frontend show an outage for
		// an item that simply has no recipe we can price.
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

// BatchEntry is one item's outcome inside a batch response. Errors are
// per-entry so one unpriceable item does not sink the whole request.
type BatchEntry struct {
	ItemID int64           `json:"item_id"`
	Result *service.Result `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

func (h *Handler) batch(c *gin.Context) {
	raw := strings.TrimSpace(c.Query("ids"))
	if raw == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ids required"})
		return
	}
	opts, err := parseOpts(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	parts := strings.Split(raw, ",")
	if len(parts) > maxBatchIDs {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "too many ids (max " + strconv.Itoa(maxBatchIDs) + ")",
		})
		return
	}

	results := make([]BatchEntry, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		id, err := strconv.ParseInt(p, 10, 64)
		if err != nil || id <= 0 {
			results = append(results, BatchEntry{Error: "invalid item id " + strconv.Quote(p)})
			continue
		}
		res, err := h.svc.Calculate(c.Request.Context(), id, opts)
		if err != nil {
			results = append(results, BatchEntry{ItemID: id, Error: err.Error()})
			continue
		}
		results = append(results, BatchEntry{ItemID: id, Result: res})
	}
	c.JSON(http.StatusOK, gin.H{"count": len(results), "results": results})
}

func parseOpts(c *gin.Context) (service.Options, error) {
	opts := service.Options{
		Player:            strings.TrimSpace(c.Query("player")),
		Mode:              c.DefaultQuery("mode", "normal"),
		IncludeIncomplete: c.Query("include_incomplete") == "true",
	}

	if raw := c.Query("aph"); raw != "" {
		aph, err := strconv.Atoi(raw)
		if err != nil || aph < 0 {
			return opts, errInvalid("aph must be a non-negative integer")
		}
		opts.ActionsPerHourOverride = aph
	}

	if raw := c.Query("spread_pct"); raw != "" {
		spread, err := strconv.ParseFloat(raw, 64)
		if err != nil || spread < 0 || spread > 100 {
			return opts, errInvalid("spread_pct must be between 0 and 100")
		}
		opts.SpreadPctOverride = &spread
	}

	switch opts.Mode {
	case "normal", "ironman", "hardcore":
	default:
		return opts, errInvalid("mode must be normal, ironman or hardcore")
	}

	boosts, err := rates.ParseBoosts(c.Query("boosts"))
	if err != nil {
		return opts, errInvalid(err.Error())
	}
	opts.Boosts = boosts
	opts.BoostsRaw = c.Query("boosts")

	return opts, nil
}

type invalidInput string

func (e invalidInput) Error() string { return string(e) }

func errInvalid(msg string) error { return invalidInput(msg) }
