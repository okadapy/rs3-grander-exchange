package weirdgloop

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs3-market/backend/shared/config"
)

type Client struct {
	baseURL   string
	userAgent string
	http      *http.Client
	minGap    time.Duration

	mu       sync.Mutex
	lastCall time.Time
}

func New(cfg config.WeirdGloopConf) *Client {
	to := cfg.HTTPTimeout
	if to == 0 {
		to = 15 * time.Second
	}
	return &Client{
		baseURL:   strings.TrimRight(cfg.BaseURL, "/"),
		userAgent: cfg.UserAgent,
		http:      &http.Client{Timeout: to},
		minGap:    cfg.MinGap,
	}
}

// LatestItem is the shape returned by /latest
type LatestItem struct {
	Price     int64 `json:"price"`
	Volume    int64 `json:"volume"`
	Timestamp int64 `json:"timestamp"`
}

// LatestBatch fetches price+volume for many IDs in one request.
// Weirdgloop /latest accepts comma-separated IDs.
func (c *Client) LatestBatch(ids []int64) (map[int64]LatestItem, error) {
	if len(ids) == 0 {
		return map[int64]LatestItem{}, nil
	}
	c.throttle()

	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatInt(id, 10)
	}
	q := url.Values{}
	q.Set("id", strings.Join(parts, ","))

	endpoint := c.baseURL + "/latest?" + q.Encode()
	body, err := c.doGET(endpoint)
	if err != nil {
		return nil, err
	}

	// Response shape: { "item_id": { "price": N, "volume": N, "timestamp": N } }
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode latest: %w body=%s", err, truncate(body, 200))
	}

	out := make(map[int64]LatestItem, len(raw))
	for k, v := range raw {
		id, err := strconv.ParseInt(k, 10, 64)
		if err != nil {
			continue
		}
		var item LatestItem
		if err := json.Unmarshal(v, &item); err != nil {
			continue
		}
		out[id] = item
	}
	return out, nil
}

// History fetches the full time-series for a single item.
func (c *Client) History(itemID int64) ([]LatestItem, error) {
	c.throttle()
	endpoint := fmt.Sprintf("%s/%d", c.baseURL, itemID)
	body, err := c.doGET(endpoint)
	if err != nil {
		return nil, err
	}
	// Weirdgloop history is { "<ts>": <price>, ... } or [{...}]; handle both.
	var asMap map[string]int64
	if err := json.Unmarshal(body, &asMap); err == nil && len(asMap) > 0 {
		out := make([]LatestItem, 0, len(asMap))
		for k, price := range asMap {
			ts, _ := strconv.ParseInt(k, 10, 64)
			out = append(out, LatestItem{Price: price, Timestamp: ts})
		}
		return out, nil
	}
	var asArr []LatestItem
	if err := json.Unmarshal(body, &asArr); err == nil {
		return asArr, nil
	}
	return nil, fmt.Errorf("decode history: unknown shape")
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
		return nil, fmt.Errorf("weirdgloop %d: %s", resp.StatusCode, truncate(b, 200))
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
