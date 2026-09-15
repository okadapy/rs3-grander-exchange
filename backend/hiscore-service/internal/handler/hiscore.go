package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/rs3-market/backend/hiscore-service/internal/service"
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
	r.GET("/hiscore/:name", h.get)
}

func (h *Handler) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) get(c *gin.Context) {
	name := c.Param("name")
	mode := c.DefaultQuery("mode", "normal")

	p, err := h.svc.GetOrFetch(name, mode)
	if err != nil {
		h.log.Warn("hiscore lookup failed",
			zap.String("name", name), zap.String("mode", mode), zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, p)
}
