package handler

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/rs3-market/backend/shared/config"
)

type upstream struct {
	Name   string
	URL    string
	Client *http.Client
}

// Health aggregates /health from every upstream route target.
type Health struct {
	log       *zap.Logger
	upstreams []upstream
}

func NewHealth(cfg *config.Config, log *zap.Logger) *Health {
	seen := map[string]bool{}
	var ups []upstream
	for _, r := range cfg.Routes {
		if seen[r.Target] {
			continue
		}
		seen[r.Target] = true
		ups = append(ups, upstream{
			Name:   r.Target,
			URL:    r.Target + "/health",
			Client: &http.Client{Timeout: 3 * time.Second},
		})
	}
	return &Health{log: log, upstreams: ups}
}

func (h *Health) Register(r *gin.Engine) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "gateway"})
	})
	r.GET("/health/all", h.all)
}

type result struct {
	Target string `json:"target"`
	OK     bool   `json:"ok"`
	Status int    `json:"status,omitempty"`
	Error  string `json:"error,omitempty"`
	Ms     int64  `json:"ms"`
}

func (h *Health) all(c *gin.Context) {
	results := make([]result, len(h.upstreams))
	var wg sync.WaitGroup
	for i, u := range h.upstreams {
		wg.Add(1)
		go func(i int, u upstream) {
			defer wg.Done()
			start := time.Now()
			resp, err := u.Client.Get(u.URL)
			elapsed := time.Since(start).Milliseconds()
			if err != nil {
				results[i] = result{Target: u.Name, OK: false, Error: err.Error(), Ms: elapsed}
				return
			}
			defer resp.Body.Close()
			results[i] = result{Target: u.Name, OK: resp.StatusCode == 200,
				Status: resp.StatusCode, Ms: elapsed}
		}(i, u)
	}
	wg.Wait()

	allOK := true
	for _, r := range results {
		if !r.OK {
			allOK = false
			break
		}
	}
	code := http.StatusOK
	if !allOK {
		code = http.StatusServiceUnavailable
	}
	c.JSON(code, gin.H{"gateway": "ok", "all_upstreams_ok": allOK, "upstreams": results})
}
