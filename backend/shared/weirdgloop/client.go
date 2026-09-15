package weirdgloop

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs3-market/backend/shared/config"
)

// Client talks to the Weirdgloop exchange-history API.
//
// /latest supports ID filtering via a pipe-separated list:
//
//	GET /latest?id=4151|4152|2
//
// We build the query string manually rather than via url.Values,
// because net/url percent-encodes "|" to "%7C" and Weirdgloop
// rejects the escaped form.
//
// Response shape (one row per requested item):
//
//	{
//	  "14622": { "id": "14622", "timestamp": "2026-09-15T07:15:41.000Z",
//	             "price": 234, "volume": 0 },
//	  ...
//	}
//
// `timestamp` is an ISO8601 string in the wild, but historic and
// mirror responses have been seen with Unix ints, so LatestItem
// accepts both via a custom unmarshaller.
type Client struct {
	baseURL   string
	userAgent string
	http      *http.Client

	mu       sync.Mutex
	lastCall time.Time
	minGap   time.Duration
}

func New(cfg config.WeirdGloopConf) *Client {
	to := cfg.HTTPTimeout
	if to == 0 {
		to = 30 * time.Second
	}
	return &Client{
		baseURL:   strings.TrimRight(cfg.BaseURL, "/"),
		userAgent: cfg.UserAgent,
		http:      &http.Client{Timeout: to},
		minGap:    cfg.MinGap,
	}
}

// LatestItem is one row from /latest.
type LatestItem struct {
	Price     int64
	Volume    int64
	Timestamp time.Time
}

// UnmarshalJSON accepts both timestamp encodings:
//   - string ISO8601:  "2026-09-15T07:15:41.000Z"
//   - number unix secs: 1726380941
//
// Also tolerates price/volume coming in as strings (historic quirk
// in some mirrors).
func (l *LatestItem) UnmarshalJSON(b []byte) error {
	var raw struct {
		Price     json.RawMessage `json:"price"`
		Volume    json.RawMessage `json:"volume"`
		Timestamp json.RawMessage `json:"timestamp"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}

	p, err := parseFlexInt(raw.Price)
	if err != nil {
		return fmt.Errorf("price: %w", err)
	}
	v, err := parseFlexInt(raw.Volume)
	if err != nil {
		return fmt.Errorf("volume: %w", err)
	}
	ts, err := parseFlexTime(raw.Timestamp)
	if err != nil {
		return fmt.Errorf("timestamp: %w", err)
	}

	l.Price = p
	l.Volume = v
	l.Timestamp = ts
	return nil
}

func parseFlexInt(raw json.RawMessage) (int64, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, nil
	}
	// Number form.
	var n int64
	if err := json.Unmarshal(raw, &n); err == nil {
		return n, nil
	}
	// String form.
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return 0, fmt.Errorf("not int or string: %s", string(raw))
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	return strconv.ParseInt(s, 10, 64)
}

func parseFlexTime(raw json.RawMessage) (time.Time, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return time.Time{}, nil
	}
	// Unix seconds as a number.
	var n int64
	if err := json.Unmarshal(raw, &n); err == nil {
		return time.Unix(n, 0).UTC(), nil
	}
	// ISO8601 as a string.
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return time.Time{}, fmt.Errorf("not time or string: %s", string(raw))
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	// Try the common layouts.
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unparseable time %q", s)
}

// LatestBatch fetches price+volume for the given IDs in one request.
func (c *Client) LatestBatch(ids []int64) (map[int64]LatestItem, error) {
	if len(ids) == 0 {
		return map[int64]LatestItem{}, nil
	}
	c.throttle()

	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatInt(id, 10)
	}
	endpoint := c.baseURL + "/latest?id=" + strings.Join(parts, "|")

	body, err := c.doGET(endpoint)
	if err != nil {
		return nil, err
	}
	return decodeLatest(body)
}

// LatestAll fetches the entire price table with no filter.
func (c *Client) LatestAll() (map[int64]LatestItem, error) {
	c.throttle()
	body, err := c.doGET(c.baseURL + "/latest")
	if err != nil {
		return nil, err
	}
	return decodeLatest(body)
}

// History fetches the time-series for a single item.
func (c *Client) History(itemID int64) ([]LatestItem, error) {
	c.throttle()
	endpoint := fmt.Sprintf("%s/%d", c.baseURL, itemID)
	body, err := c.doGET(endpoint)
	if err != nil {
		return nil, err
	}

	var asMap map[string]int64
	if err := json.Unmarshal(body, &asMap); err == nil && len(asMap) > 0 {
		out := make([]LatestItem, 0, len(asMap))
		for k, price := range asMap {
			ts, _ := strconv.ParseInt(k, 10, 64)
			out = append(out, LatestItem{
				Price:     price,
				Timestamp: time.Unix(ts, 0).UTC(),
			})
		}
		return out, nil
	}

	var asArr []LatestItem
	if err := json.Unmarshal(body, &asArr); err == nil {
		return asArr, nil
	}
	return nil, fmt.Errorf("decode history: unknown shape: %s", truncate(body, 200))
}

func decodeLatest(body []byte) (map[int64]LatestItem, error) {
	// Explicit failure envelope.
	var envelope struct {
		Success *bool                 `json:"success"`
		Error   string                `json:"error"`
		Items   map[string]LatestItem `json:"items"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil {
		if envelope.Success != nil && !*envelope.Success {
			// "No results returned" means none of the requested IDs
			// are tradeable. This is a normal, expected outcome for
			// batches that contain only bound or untradeable items —
			// not a failure. Return an empty result so the caller
			// treats it as "nothing to insert" instead of an error.
			if isNoResultsError(envelope.Error) {
				return map[int64]LatestItem{}, nil
			}
			return nil, fmt.Errorf("weirdgloop error: %s", envelope.Error)
		}
		if len(envelope.Items) > 0 {
			return convertKeys(envelope.Items), nil
		}
	}

	// Direct map: { "4151": {...}, ... }
	var direct map[string]LatestItem
	if err := json.Unmarshal(body, &direct); err == nil && len(direct) > 0 {
		return convertKeys(direct), nil
	}

	return nil, fmt.Errorf("unrecognized response shape: %s", truncate(body, 300))
}

func convertKeys(m map[string]LatestItem) map[int64]LatestItem {
	out := make(map[int64]LatestItem, len(m))
	for k, v := range m {
		id, err := strconv.ParseInt(k, 10, 64)
		if err != nil {
			continue
		}
		out[id] = v
	}
	return out
}

func (c *Client) doGET(endpoint string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("weirdgloop %d: %s",
			resp.StatusCode, truncate(b, 200))
	}
	return io.ReadAll(resp.Body)
}

func (c *Client) throttle() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if gap := time.Since(c.lastCall); gap < c.minGap {
		time.Sleep(c.minGap - gap)
	}
	c.lastCall = time.Now()
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}

// isNoResultsError reports whether the given error string is
// Weirdgloop's way of saying "none of these IDs are tradeable".
// Treated as an empty result rather than a failure.
func isNoResultsError(msg string) bool {
	m := strings.ToLower(strings.TrimSpace(msg))
	return m == "no results returned" ||
		m == "no results" ||
		m == "empty result"
}
