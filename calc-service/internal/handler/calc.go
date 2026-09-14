package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/rs3-market/backend/calc-service/internal/service"
)

type Handler struct {
	svc *service.Service
	log *zap.Logger
}

func New(svc *service.Service, log *zap.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

func (h *Handler) Register(r *gin.Engine) {
	r.GET("/health", h.health)
	r.GET("/calc/:itemID", h.calc)
	r.GET("/calc/batch", h.batch)
}

func (h *Handler) health(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) }

func (h *Handler) calc(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("itemID"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad item id"})
		return
	}
	opts := parseOpts(c)
	res, err := h.svc.Calculate(c.Request.Context(), id, opts)
	if err != nil {
		h.log.Warn("calc failed", zap.Int64("item_id", id), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) batch(c *gin.Context) {
	raw := c.Query("ids")
	if raw == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ids required"})
		return
	}
	opts := parseOpts(c)
	out := gin.H{}
	for _, p := range strings.Split(raw, ",") {
		id, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
		if err != nil {
			continue
		}
		res, err := h.svc.Calculate(c.Request.Context(), id, opts)
		if err != nil {
			out[p] = gin.H{"error": err.Error()}
			continue
		}
		out[p] = res
	}
	c.JSON(http.StatusOK, out)
}

func parseOpts(c *gin.Context) service.Options {
	aph, _ := strconv.Atoi(c.Query("aph"))
	return service.Options{
		ActionsPerHourOverride: aph,
		Player:                 c.Query("player"),
		Mode:                   c.DefaultQuery("mode", "normal"),
		Normalized:             c.Query("normalized") == "true",
	}
}
