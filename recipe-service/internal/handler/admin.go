package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/rs3-market/backend/recipe-service/internal/scraper"
)

// Admin exposes ops endpoints for local debugging. Not routed through
// the gateway — only reachable on the recipe-service port directly.
type Admin struct {
	scraper  scraper.ScrapeRunner
	resolver scraper.IDResolver
	log      *zap.Logger
	http     *http.Client
}

func NewAdmin(s scraper.ScrapeRunner, r scraper.IDResolver, log *zap.Logger) *Admin {
	return &Admin{
		scraper:  s,
		resolver: r,
		log:      log,
		http:     &http.Client{Timeout: 60 * time.Second},
	}
}

func (a *Admin) Register(r *gin.Engine) {
	r.POST("/internal/scrape", a.triggerScrape)
	r.GET("/internal/dump", a.dumpPage)
}

func (a *Admin) triggerScrape(c *gin.Context) {
	go func() {
		// The same cycle the background loop runs: a scrape on its
		// own would leave every input's item ID cleared.
		n, res, err := scraper.RunScrapeCycle(a.scraper, a.resolver, a.log)
		if err != nil {
			a.log.Error("manual scrape failed", zap.Error(err))
			return
		}
		a.log.Info("manual scrape done",
			zap.Int("upserted", n),
			zap.Int64("inputs_resolved", res.InputsFromRecipes+res.InputsFromGEIDs))
	}()
	c.JSON(http.StatusAccepted, gin.H{"status": "scrape started"})
}

// dumpPage fetches the raw wikitext for a page so you can inspect
// the infobox format when debugging the parser.
//
//	curl "http://localhost:8082/internal/dump?page=Sapphire%20necklace"
//	curl "http://localhost:8082/internal/dump?page=Sapphire%20necklace&raw=1"
func (a *Admin) dumpPage(c *gin.Context) {
	page := c.Query("page")
	if page == "" {
		c.String(http.StatusBadRequest, "?page= required")
		return
	}

	q := url.Values{}
	q.Set("action", "parse")
	q.Set("page", page)
	q.Set("prop", "wikitext")
	q.Set("format", "json")
	q.Set("formatversion", "2")

	req, _ := http.NewRequest(http.MethodGet,
		"https://runescape.wiki/api.php?"+q.Encode(), nil)
	req.Header.Set("User-Agent", "RS3-Market-Backend/1.0 (contact: admin@example.com)")

	resp, err := a.http.Do(req)
	if err != nil {
		c.String(http.StatusBadGateway, "upstream: %v", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if c.Query("raw") == "1" {
		c.Data(resp.StatusCode, "application/json", body)
		return
	}

	var parsed struct {
		Parse struct {
			Wikitext string `json:"wikitext"`
		} `json:"parse"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		c.Data(resp.StatusCode, "application/json", body)
		return
	}
	wt := parsed.Parse.Wikitext
	if wt == "" {
		c.String(http.StatusNotFound, "page %q not found or has no wikitext", page)
		return
	}
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(wt))
}
