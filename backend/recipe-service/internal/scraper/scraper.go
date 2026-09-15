package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/rs3-market/backend/recipe-service/internal/repository"
	"github.com/rs3-market/backend/shared/models"
)

// Scraper pulls recipes from the RuneScape Wiki MediaWiki API.
//
// Discovery uses `list=embeddedin` against the recipe infobox template
// rather than wiki categories — categories on RS3 are inconsistent
// (some skills have none, others are incomplete), whereas a page with
// the recipe infobox is unambiguously a recipe.
//
// Wikitext is fetched in batches of up to 50 pages per request. Each
// request is retried up to maxRetries times with exponential backoff.
// Actions-per-hour comes from the wiki when the infobox has it, or
// falls back to a per-skill default. Users can override at calc time.
type Scraper struct {
	wikiBase  string
	userAgent string
	log       *zap.Logger
	repo      *repository.Repo
	http      *http.Client
}

const (
	// defaultActionsPerHour is the last-resort throughput for a skill
	// with no per-skill default of its own.
	defaultActionsPerHour = 600

	batchSize      = 50
	maxRetries     = 3
	initialBackoff = 1 * time.Second
	requestTimeout = 60 * time.Second
)

// Candidate template names — RS3 wiki has used several over the years.
// We try each until one returns pages.
var recipeTemplates = []string{
	"Template:Infobox Recipe",
	"Template:Recipe",
	"Template:Infobox recipe",
	"Template:Crafting recipe",
}

var fallbackActionsPerHour = map[string]int{
	"Crafting":     1000,
	"Smithing":     100,
	"Fletching":    1200,
	"Herblore":     1200,
	"Construction": 300,
	"Runecrafting": 800,
	"Firemaking":   900,
	"Cooking":      1200,
	"Farming":      100,
	"Divination":   700,
	"Prayer":       600,
}

func New(repo *repository.Repo, log *zap.Logger) *Scraper {
	return &Scraper{
		wikiBase:  "https://runescape.wiki/api.php",
		userAgent: "RS3-Market-Backend/1.0 (contact: admin@example.com)",
		log:       log,
		repo:      repo,
		http:      &http.Client{Timeout: requestTimeout},
	}
}

// ScrapeAll discovers every recipe page and upserts it. Returns the
// number of successfully upserted recipes.
func (s *Scraper) ScrapeAll() (int, error) {
	titles, sourceTemplate, err := s.discoverRecipePages()
	if err != nil {
		return 0, fmt.Errorf("discover: %w", err)
	}
	s.log.Info("discovered recipe pages",
		zap.Int("count", len(titles)),
		zap.String("via_template", sourceTemplate))

	if len(titles) == 0 {
		return 0, fmt.Errorf("no recipe pages found via any of %v", recipeTemplates)
	}

	total := 0
	for i := 0; i < len(titles); i += batchSize {
		end := i + batchSize
		if end > len(titles) {
			end = len(titles)
		}
		batch := titles[i:end]

		pages, err := s.fetchBatchWikitext(batch)
		if err != nil {
			s.log.Warn("batch fetch failed",
				zap.Int("offset", i), zap.Error(err))
			continue
		}

		inserted := 0
		for title, wt := range pages {
			rec := parseWikitext(title, "", wt)
			if rec == nil {
				continue
			}
			if rec.Skill == "" {
				continue
			}
			if rec.ActionsPerHour <= 0 {
				// The wiki almost never publishes a real
				// actions-per-hour, so most recipes land here. Marking
				// the source lets the API say so instead of presenting
				// a house default as a measured rate.
				rec.ActionsPerHour = fallbackActionsPerHour[rec.Skill]
				if rec.ActionsPerHour == 0 {
					rec.ActionsPerHour = defaultActionsPerHour
				}
				rec.APHSource = models.APHSourceDefault
			}
			if err := s.repo.UpsertRecipe(rec); err != nil {
				s.log.Debug("upsert failed",
					zap.String("title", title), zap.Error(err))
				continue
			}
			inserted++
		}
		total += inserted

		s.log.Debug("batch done",
			zap.Int("requested", len(batch)),
			zap.Int("returned", len(pages)),
			zap.Int("upserted", inserted))

		time.Sleep(300 * time.Millisecond)
	}

	s.log.Info("scrape complete", zap.Int("upserted", total))
	return total, nil
}

// discoverRecipePages tries each candidate template until one yields
// pages. Returns the titles and the template that worked.
func (s *Scraper) discoverRecipePages() ([]string, string, error) {
	for _, tmpl := range recipeTemplates {
		titles, err := s.embeddedIn(tmpl)
		if err != nil {
			s.log.Debug("embeddedin failed",
				zap.String("template", tmpl), zap.Error(err))
			continue
		}
		if len(titles) > 0 {
			return titles, tmpl, nil
		}
	}
	return nil, "", fmt.Errorf("no template yielded results")
}

// embeddedIn returns all main-namespace pages that transclude tmpl.
// Follows continuation so we get the full set, not just the first 500.
func (s *Scraper) embeddedIn(tmpl string) ([]string, error) {
	var titles []string
	cont := ""

	for {
		q := url.Values{}
		q.Set("action", "query")
		q.Set("list", "embeddedin")
		q.Set("eititle", tmpl)
		q.Set("eilimit", "500")
		q.Set("einamespace", "0")
		q.Set("format", "json")
		q.Set("formatversion", "2")
		if cont != "" {
			q.Set("eicontinue", cont)
		}

		body, err := s.getWithRetry(s.wikiBase + "?" + q.Encode())
		if err != nil {
			return titles, err
		}

		var resp struct {
			Continue struct {
				EIContinue string `json:"eicontinue"`
			} `json:"continue"`
			Query struct {
				EmbeddedIn []struct {
					Title string `json:"title"`
				} `json:"embeddedin"`
			} `json:"query"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return titles, fmt.Errorf("decode embeddedin: %w", err)
		}
		for _, p := range resp.Query.EmbeddedIn {
			titles = append(titles, p.Title)
		}

		cont = resp.Continue.EIContinue
		if cont == "" {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	return titles, nil
}

// fetchBatchWikitext fetches up to batchSize pages' wikitext in one
// call via prop=revisions. Redirects are resolved so the returned
// title may differ from what was requested.
func (s *Scraper) fetchBatchWikitext(titles []string) (map[string]string, error) {
	if len(titles) == 0 {
		return map[string]string{}, nil
	}

	q := url.Values{}
	q.Set("action", "query")
	q.Set("prop", "revisions")
	q.Set("rvprop", "content")
	q.Set("rvslots", "main")
	q.Set("titles", strings.Join(titles, "|"))
	q.Set("redirects", "1")
	q.Set("format", "json")
	q.Set("formatversion", "2")

	body, err := s.getWithRetry(s.wikiBase + "?" + q.Encode())
	if err != nil {
		return nil, err
	}

	var resp struct {
		Query struct {
			Pages []struct {
				Title     string `json:"title"`
				Missing   bool   `json:"missing"`
				Revisions []struct {
					Slots struct {
						Main struct {
							Content string `json:"content"`
						} `json:"main"`
					} `json:"slots"`
				} `json:"revisions"`
			} `json:"pages"`
		} `json:"query"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode revisions: %w", err)
	}

	out := make(map[string]string, len(resp.Query.Pages))
	for _, p := range resp.Query.Pages {
		if p.Missing || len(p.Revisions) == 0 {
			continue
		}
		content := p.Revisions[0].Slots.Main.Content
		if content == "" {
			continue
		}
		out[p.Title] = content
	}
	return out, nil
}

// getWithRetry wraps get() with exponential backoff. Retryable
// failures are 429/503 and network timeouts; other 4xx are returned
// immediately.
func (s *Scraper) getWithRetry(u string) ([]byte, error) {
	var lastErr error
	backoff := initialBackoff

	for attempt := 1; attempt <= maxRetries; attempt++ {
		body, err := s.get(u)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !isRetryable(err) {
			return nil, err
		}
		if attempt < maxRetries {
			s.log.Debug("wiki request failed, retrying",
				zap.Int("attempt", attempt),
				zap.Duration("backoff", backoff),
				zap.Error(err))
			time.Sleep(backoff)
			backoff *= 2
		}
	}
	return nil, fmt.Errorf("after %d attempts: %w", maxRetries, lastErr)
}

func (s *Scraper) get(u string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 || resp.StatusCode == 503 {
		return nil, &retryableError{status: resp.StatusCode}
	}
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("wiki status %d: %s", resp.StatusCode, truncate(b, 200))
	}
	return io.ReadAll(resp.Body)
}

type retryableError struct{ status int }

func (e *retryableError) Error() string {
	return fmt.Sprintf("retryable status %d", e.status)
}

func isRetryable(err error) bool {
	if _, ok := err.(*retryableError); ok {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "context deadline exceeded") ||
		strings.Contains(s, "connection reset") ||
		strings.Contains(s, "EOF") ||
		strings.Contains(s, "timeout")
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
