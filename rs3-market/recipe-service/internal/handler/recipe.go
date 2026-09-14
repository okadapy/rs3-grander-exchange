package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/rs3-market/backend/recipe-service/internal/service"
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
	r.GET("/recipes", h.list)
	r.GET("/recipes/:itemID", h.tree)
	r.GET("/recipes/:itemID/tree", h.tree)
	r.GET("/internal/all-item-ids", h.allIDs)
}

func (h *Handler) health(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) }

func (h *Handler) list(c *gin.Context) {
	skill := c.Query("skill")
	lvl, _ := strconv.Atoi(c.Query("level"))
	list, err := h.svc.ListFiltered(skill, lvl)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": len(list), "recipes": list})
}

func (h *Handler) tree(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("itemID"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item id"})
		return
	}
	t, err := h.svc.BuildTree(id)
	if err != nil {
		h.log.Warn("tree build", zap.Int64("item_id", id), zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, t)
}

// internal endpoint — used by ge-price-service and calc-service.
func (h *Handler) allIDs(c *gin.Context) {
	ids, err := h.svc.AllOutputItemIDs(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item_ids": ids})
}
