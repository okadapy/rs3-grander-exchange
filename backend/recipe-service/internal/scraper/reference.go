package scraper

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/rs3-market/backend/recipe-service/internal/repository"
	"github.com/rs3-market/backend/shared/models"
)

// The wiki publishes two machine-readable reference tables that the rest
// of this system depends on. Both are plain {"Item name": <int>} JSON
// with a handful of %PERCENT_DELIMITED% metadata keys mixed in.
//
//   - GEIDs   maps item name -> Grand Exchange item ID. Without it, any
//     recipe input that is gathered rather than crafted (ores, logs,
//     herbs) has no ID, so it never gets a price, so it is silently
//     costed at zero.
//   - GELimits maps item name -> 4-hour buy limit. Without it, every
//     GP/h figure is an unreachable upper bound.
const (
	GEIDsURL    = "https://runescape.wiki/w/Module:GEIDs/data.json?action=raw"
	GELimitsURL = "https://runescape.wiki/w/Module:GELimits/data.json?action=raw"

	referenceUserAgent = "RS3-Market-Backend/1.0 (contact: admin@example.com)"
	referenceTimeout   = 90 * time.Second
)

// parseWikiIDMap decodes the wiki's name -> integer tables.
//
// Kept separate from the HTTP fetch so the quirks of the format — the
// metadata keys, string-encoded numbers, zero and negative values — are
// testable without touching the network.
func parseWikiIDMap(body []byte) (map[string]int64, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode: %w body=%s", err, truncate(body, 200))
	}

	out := make(map[string]int64, len(raw))
	for name, value := range raw {
		name = strings.TrimSpace(name)
		if name == "" || strings.HasPrefix(name, "%") {
			continue
		}
		n, ok := flexInt(value)
		if !ok || n <= 0 {
			continue
		}
		out[name] = n
	}
	return out, nil
}

// flexInt accepts both 12345 and "12345" — the wiki modules have carried
// both over the years.
func flexInt(raw json.RawMessage) (int64, bool) {
	var n int64
	if err := json.Unmarshal(raw, &n); err == nil {
		return n, true
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return 0, false
	}
	var parsed int64
	if _, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &parsed); err != nil {
		return 0, false
	}
	return parsed, true
}

func fetchWikiJSON(url string) ([]byte, error) {
	client := &http.Client{Timeout: referenceTimeout}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", referenceUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wiki status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// FetchGEIDs pulls the GE item ID map, stores it, then re-runs every
// ID-resolution pass that depends on it.
func FetchGEIDs(repo *repository.Repo, log *zap.Logger) error {
	log.Info("fetching GEIDs map", zap.String("url", GEIDsURL))

	body, err := fetchWikiJSON(GEIDsURL)
	if err != nil {
		return err
	}
	parsed, err := parseWikiIDMap(body)
	if err != nil {
		return err
	}
	log.Info("parsed GEIDs map", zap.Int("entries", len(parsed)))

	rows := make([]models.GEIDMap, 0, len(parsed))
	for name, id := range parsed {
		rows = append(rows, models.GEIDMap{Name: name, ItemID: id})
	}
	if err := repo.UpsertGEIDs(rows); err != nil {
		return fmt.Errorf("upsert geids: %w", err)
	}
	log.Info("stored GEIDs map", zap.Int("rows", len(rows)))

	// A fresh map can resolve names that were unknown yesterday, so run
	// the full sequence rather than only the two passes that read the
	// map directly — a newly resolved output is a name the
	// inputs-from-recipes pass could not match before.
	if _, err := ResolveIDs(repo, log); err != nil {
		return err
	}
	return nil
}

// FetchBuyLimits pulls the 4-hour Grand Exchange buy limits and joins
// them to item IDs via the GEIDs table.
func FetchBuyLimits(repo *repository.Repo, log *zap.Logger) error {
	log.Info("fetching GE buy limits", zap.String("url", GELimitsURL))

	body, err := fetchWikiJSON(GELimitsURL)
	if err != nil {
		return err
	}
	parsed, err := parseWikiIDMap(body)
	if err != nil {
		return err
	}
	log.Info("parsed buy limits", zap.Int("entries", len(parsed)))

	rows := make([]models.ItemLimit, 0, len(parsed))
	for name, limit := range parsed {
		rows = append(rows, models.ItemLimit{Name: name, Limit4h: int(limit)})
	}
	if err := repo.UpsertItemLimits(rows); err != nil {
		return fmt.Errorf("upsert limits: %w", err)
	}

	n, err := repo.BackfillLimitItemIDs()
	if err != nil {
		return fmt.Errorf("resolve limit ids: %w", err)
	}
	log.Info("stored buy limits",
		zap.Int("rows", len(rows)),
		zap.Int64("id_resolved", n))
	return nil
}
