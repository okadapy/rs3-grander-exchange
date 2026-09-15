package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/rs3-market/backend/recipe-service/internal/service"
)

const (
	defaultLimit = 50
	maxLimit     = 500
)

type Handler struct {
	svc *service.Service
	log *zap.Logger
}

func New(svc *service.Service, log *zap.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

func (h *Handler) Register(r *gin.Engine) {
	legacySearchRedirects(r)
	r.GET("/health", h.health)

	// Static routes MUST come before :itemID so Gin doesn't try to
	// match "ids"/"search" as an item ID. Gin v1.10 prefers static
	// segments, but explicit ordering documents intent.
	// /recipes/* static + param routes. Gin v1.10 tolerates the
	// static-then-param ordering, but keep them adjacent so the
	// routing tree stays easy to reason about.
	r.GET("/recipes", h.list)
	r.GET("/recipes/ids", h.ids)
	r.GET("/recipes/:itemID", h.tree)
	r.GET("/recipes/:itemID/tree", h.tree)

	// Search lives under its own prefix. This keeps it clear of the
	// /recipes/:itemID wildcard and lets us add more search scopes
	// later (e.g. /search/skills) without reshuffling.
	r.GET("/search/recipes", h.searchRecipes)
	r.GET("/search/items", h.searchItems)

	// Grand Exchange 4-hour buy limits. Public because the frontend
	// needs them to show what a strategy can actually absorb, and
	// ge-price-service reads them from here too.
	r.GET("/items/limits", h.buyLimits)

	// Internal: consumed by ge-price-service.
	r.GET("/internal/all-item-ids", h.allIDs)
	r.GET("/internal/input-id-stats", h.inputIDStats)
	r.POST("/internal/backfill-inputs", h.backfillInputs)
}

func (h *Handler) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ---------- listings ----------

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

func (h *Handler) ids(c *gin.Context) {
	skill := strings.TrimSpace(c.Query("skill"))
	minLevel, _ := strconv.Atoi(c.Query("min_level"))
	maxLevel, _ := strconv.Atoi(c.Query("level"))

	if minLevel > 0 && maxLevel > 0 && minLevel > maxLevel {
		c.JSON(http.StatusBadRequest, gin.H{"error": "min_level cannot exceed level"})
		return
	}

	ids, err := h.svc.ListOutputItemIDs(skill, minLevel, maxLevel)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"skill":     skill,
		"min_level": minLevel,
		"max_level": maxLevel,
		"count":     len(ids),
		"item_ids":  ids,
	})
}

// ---------- search ----------

// searchRecipes: GET /recipes/search?q=&skill=&min_level=&level=&limit=&offset=
func (h *Handler) searchRecipes(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	skill := strings.TrimSpace(c.Query("skill"))
	minLevel, _ := strconv.Atoi(c.Query("min_level"))
	maxLevel, _ := strconv.Atoi(c.Query("level"))
	limit, offset := pagination(c)

	if minLevel > 0 && maxLevel > 0 && minLevel > maxLevel {
		c.JSON(http.StatusBadRequest, gin.H{"error": "min_level cannot exceed level"})
		return
	}

	recipes, total, err := h.svc.SearchRecipes(q, skill, minLevel, maxLevel, limit, offset)
	if err != nil {
		h.log.Warn("search recipes", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"query":   q,
		"count":   len(recipes),
		"total":   total,
		"limit":   limit,
		"offset":  offset,
		"recipes": recipes,
	})
}

// searchItems: GET /items/search?q=&source=output|input|all&limit=&offset=
func (h *Handler) searchItems(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	source := strings.TrimSpace(c.Query("source"))
	if source == "" {
		source = "all"
	}
	limit, offset := pagination(c)

	items, total, err := h.svc.SearchItems(q, source, limit, offset)
	if err != nil {
		h.log.Warn("search items", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"query":  q,
		"source": source,
		"count":  len(items),
		"total":  total,
		"limit":  limit,
		"offset": offset,
		"items":  items,
	})
}

// ---------- buy limits ----------

// buyLimits: GET /items/limits?ids=1,2,3
//
// Items with no known limit are omitted rather than returned as 0, so a
// caller can tell "unlimited/unknown" from "limited to nothing".
func (h *Handler) buyLimits(c *gin.Context) {
	ids, err := parseIDs(c.Query("ids"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	limits, err := h.svc.BuyLimitsFor(ids)
	if err != nil {
		h.log.Warn("buy limits", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make(map[string]int, len(limits))
	for id, lim := range limits {
		out[strconv.FormatInt(id, 10)] = lim
	}
	c.JSON(http.StatusOK, gin.H{"count": len(out), "limits": out})
}

// ---------- tree ----------

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

// ---------- internal ----------

func (h *Handler) allIDs(c *gin.Context) {
	ids, err := h.svc.AllRelevantItemIDs(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item_ids": ids})
}

func (h *Handler) inputIDStats(c *gin.Context) {
	total, resolved, err := h.svc.InputIDStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	pct := 0.0
	if total > 0 {
		pct = float64(resolved) / float64(total) * 100
	}
	c.JSON(http.StatusOK, gin.H{
		"total_inputs":   total,
		"resolved_ids":   resolved,
		"unresolved_ids": total - resolved,
		"coverage_pct":   pct,
	})
}

func (h *Handler) backfillInputs(c *gin.Context) {
	n, err := h.svc.BackfillInputItemIDs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"rows_updated": n})
}

// ---------- helpers ----------

// maxIDsPerRequest bounds comma-separated ID lists.
const maxIDsPerRequest = 500

// parseIDs rejects malformed entries rather than skipping them: silently
// dropping a bad ID makes a client typo look like missing data.
func parseIDs(raw string) ([]int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("ids required")
	}
	parts := strings.Split(raw, ",")
	if len(parts) > maxIDsPerRequest {
		return nil, fmt.Errorf("too many ids (max %d)", maxIDsPerRequest)
	}
	ids := make([]int64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		id, err := strconv.ParseInt(p, 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("invalid item id %q", p)
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, errors.New("ids required")
	}
	return ids, nil
}

func pagination(c *gin.Context) (limit, offset int) {
	limit, _ = strconv.Atoi(c.Query("limit"))
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	offset, _ = strconv.Atoi(c.Query("offset"))
	if offset < 0 {
		offset = 0
	}
	return
}

// legacySearchRedirects keeps the old /recipes/search and /items/search
// paths working after the move to /search/*. Each handler issues a 301
// to the new canonical path with the query string preserved.
func legacySearchRedirects(r *gin.Engine) {
	r.GET("/recipes/search", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/search/recipes?"+c.Request.URL.RawQuery)
	})
	r.GET("/items/search", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/search/items?"+c.Request.URL.RawQuery)
	})
}
