package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rs3-market/backend/shared/models"
)

type RecipeClient struct {
	Base string
	HTTP *http.Client
}

func NewRecipeClient(base string) *RecipeClient {
	return &RecipeClient{Base: strings.TrimRight(base, "/"), HTTP: &http.Client{Timeout: 20 * time.Second}}
}

func (c *RecipeClient) Tree(ctx context.Context, itemID int64) (*TreeResp, error) {
	u := fmt.Sprintf("%s/recipes/%d/tree", c.Base, itemID)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("recipe svc %d: %s", resp.StatusCode, string(b))
	}
	var t TreeResp
	if err := json.NewDecoder(resp.Body).Decode(&t); err != nil {
		return nil, err
	}
	return &t, nil
}

// BuyLimits returns itemID -> 4-hour Grand Exchange buy limit. Items
// with no known limit are absent from the map.
func (c *RecipeClient) BuyLimits(ctx context.Context, ids []int64) (map[int64]int, error) {
	if len(ids) == 0 {
		return map[int64]int{}, nil
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatInt(id, 10)
	}
	u := fmt.Sprintf("%s/items/limits?ids=%s", c.Base, strings.Join(parts, ","))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("recipe svc %d: %s", resp.StatusCode, string(b))
	}

	var payload struct {
		Limits map[string]int `json:"limits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
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

type TreeResp struct {
	Root *Node `json:"root"`
}

type Node struct {
	Recipe   models.Recipe `json:"recipe"`
	Children []*Node       `json:"children"`
	Depth    int           `json:"depth"`
}

// ---------------- price client ----------------

type PriceClient struct {
	Base string
	HTTP *http.Client
}

func NewPriceClient(base string) *PriceClient {
	return &PriceClient{Base: strings.TrimRight(base, "/"), HTTP: &http.Client{Timeout: 20 * time.Second}}
}

func (c *PriceClient) Latest(ctx context.Context, ids []int64) (map[int64]models.PriceSnapshot, error) {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf("%d", id)
	}
	u := fmt.Sprintf("%s/prices/latest?ids=%s", c.Base, strings.Join(parts, ","))
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var payload struct {
		Prices []models.PriceSnapshot `json:"prices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	out := make(map[int64]models.PriceSnapshot, len(payload.Prices))
	for _, p := range payload.Prices {
		out[p.ItemID] = p
	}
	return out, nil
}

// ---------------- hiscore client ----------------

type HiscoreClient struct {
	Base string
	HTTP *http.Client
}

func NewHiscoreClient(base string) *HiscoreClient {
	return &HiscoreClient{Base: strings.TrimRight(base, "/"), HTTP: &http.Client{Timeout: 20 * time.Second}}
}

func (c *HiscoreClient) Get(ctx context.Context, name, mode string) (*models.Player, error) {
	u := fmt.Sprintf("%s/hiscore/%s?mode=%s", c.Base, url.PathEscape(name), url.QueryEscape(mode))
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("hiscore svc %d", resp.StatusCode)
	}
	var p models.Player
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return nil, err
	}
	return &p, nil
}
