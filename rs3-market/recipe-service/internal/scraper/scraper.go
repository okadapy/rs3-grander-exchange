package scraper

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/rs3-market/backend/shared/models"
	"github.com/rs3-market/backend/recipe-service/internal/repository"
)

// Scraper pulls recipes from the RuneScape Wiki MediaWiki API.
//
// We scrape the wiki rather than the official game cache for two reasons:
//   1. Wiki already curates recipe inputs / XP / level requirements
//   2. Actions-per-hour (aph) values live in training-guide tables,
//      not in the game cache
//
// aph is loaded from the guide tables when available. If the wiki has no
// aph row, the per-skill fallback below is used. Users may override
// aph at query time via calc-service ?aph=...
type Scraper struct {
	wikiBase string
	log      *zap.Logger
	repo     *repository.Repo
	http     *http.Client
}

var fallbackActionsPerHour = map[string]int{
	"Crafting":     1000,
	"Smithing":      100,
	"Fletching":    1200,
	"Herblore":     1200,
	"Construction":  300,
	"Runecrafting":  800,
	"Firemaking":    900,
	"Cooking":      1200,
	"Farming":       100,
	"Divination":    700,
	"Prayer":        600,
}

// Skills we cover. Everything craftable.
var craftSkills = []string{
	"Crafting", "Smithing", "Fletching", "Herblore",
	"Construction", "Runecrafting", "Firemaking", "Cooking",
	"Farming", "Divination", "Prayer",
}

func New(repo *repository.Repo, log *zap.Logger) *Scraper {
	return &Scraper{
		wikiBase: "https://runescape.wiki/api.php",
		log:      log,
		repo:     repo,
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

// ScrapeAll iterates the wiki's category pages per skill and upserts recipes.
func (s *Scraper) ScrapeAll() error {
	for _, skill := range craftSkills {
		s.log.Info("scraping skill", zap.String("skill", skill))
		if err := s.scrapeSkill(skill); err != nil {
			s.log.Warn("skill scrape failed", zap.String("skill", skill), zap.Error(err))
		}
	}
	return nil
}

// scrapeSkill pulls the "<Skill> items" category members.
func (s *Scraper) scrapeSkill(skill string) error {
	cat := url.QueryEscape(skill + " items")
	endpoint := fmt.Sprintf("%s?action=query&list=categorymembers&cmtitle=Category:%s&cmlimit=500&format=json", s.wikiBase, cat)

	body, err := s.get(endpoint)
	if err != nil {
		return err
	}

	var resp struct {
		Query struct {
			CategoryMembers []struct {
				Title string `json:"title"`
			} `json:"categorymembers"`
		} `json:"query"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return err
	}

	for _, m := range resp.Query.CategoryMembers {
		if strings.HasPrefix(m.Title, "Category:") {
			continue
		}
		if err := s.scrapeItem(m.Title, skill); err != nil {
			s.log.Debug("item skip", zap.String("title", m.Title), zap.Error(err))
		}
		// be polite to the wiki
		time.Sleep(200 * time.Millisecond)
	}
	return nil
}

// scrapeItem parses a single item page for recipe parameters.
// We use the parser API + text extraction. For robustness we prefer
// the infobox module output (`action=parse&prop=text&section=0`).
func (s *Scraper) scrapeItem(title, skill string) error {
	q := url.Values{}
	q.Set("action", "parse")
	q.Set("page", title)
	q.Set("prop", "wikitext")
	q.Set("format", "json")
	q.Set("formatversion", "2")

	body, err := s.get(s.wikiBase + "?" + q.Encode())
	if err != nil {
		return err
	}

	var resp struct {
		Parse struct {
			Wikitext string `json:"wikitext"`
		} `json:"parse"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return err
	}
	wt := resp.Parse.Wikitext
	if wt == "" {
		return fmt.Errorf("no wikitext")
	}

	rec := parseWikitext(title, skill, wt)
	if rec == nil {
		return nil
	}

	// Fallback aph if we couldn't parse one
	if rec.ActionsPerHour <= 0 {
		rec.ActionsPerHour = fallbackActionsPerHour[skill]
		if rec.ActionsPerHour == 0 {
			rec.ActionsPerHour = 600
		}
	}

	return s.repo.UpsertRecipe(rec)
}

func (s *Scraper) get(u string) ([]byte, error) {
	req, _ := http.NewRequest(http.MethodGet, u, nil)
	req.Header.Set("User-Agent", "RS3-Market-Backend/1.0 (contact@example.com)")
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("wiki status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// helper used by parser_test and by scraper — exported for tests
func ParseIntOr(s string, d int) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return d
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return d
	}
	return n
}
