package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RecipeClient reads the wiki-derived reference data that recipe-service
// owns. ge-price-service needs Grand Exchange buy limits to report how
// much of an item can actually be acquired, but the scraper — and so the
// canonical copy of that data — lives on the other side.
type RecipeClient struct {
	Base string
	HTTP *http.Client
}

func NewRecipeClient(base string) *RecipeClient {
	return &RecipeClient{
		Base: strings.TrimRight(base, "/"),
		HTTP: &http.Client{Timeout: 15 * time.Second},
	}
}

// BuyLimits returns itemID -> 4-hour buy limit for the requested IDs.
// Items with no known limit are simply absent from the map.
func (c *RecipeClient) BuyLimits(ctx context.Context, ids []int64) (map[int64]int, error) {
	if len(ids) == 0 {
		return map[int64]int{}, nil
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatInt(id, 10)
	}
	u := c.Base + "/items/limits?ids=" + strings.Join(parts, ",")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", u, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("recipe-service %d: %s", resp.StatusCode, truncate(b, 200))
	}

	var payload struct {
		Limits map[string]int `json:"limits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode buy limits: %w", err)
	}

	out := make(map[int64]int, len(payload.Limits))
	for k, v := range payload.Limits {
		id, err := strconv.ParseInt(k, 10, 64)
		if err != nil {
			continue
		}
		out[id] = v
	}
	return out, nil
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
