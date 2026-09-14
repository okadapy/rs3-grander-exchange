package poller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"go.uber.org/zap"

	"github.com/rs3-market/backend/shared/cache"
	"github.com/rs3-market/backend/shared/config"
	"github.com/rs3-market/backend/shared/models"
	"github.com/rs3-market/backend/shared/weirdgloop"
	"github.com/rs3-market/backend/ge-price-service/internal/repository"
)

type Poller struct {
	cfg          *config.Config
	log          *zap.Logger
	repo         *repository.Repo
	c            *cache.Cache
	wg           *weirdgloop.Client
	httpClient   *http.Client
}

func New(cfg *config.Config, repo *repository.Repo, c *cache.Cache, wg *weirdgloop.Client, log *zap.Logger) *Poller {
	return &Poller{
		cfg: cfg, repo: repo, c: c, wg: wg, log: log,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (p *Poller) Run(ctx context.Context) {
	interval := p.cfg.Poller.PollInterval
	if interval == 0 {
		interval = time.Hour
	}
	tick := time.NewTicker(interval)
	defer tick.Stop()

	// initial delay to let the recipe-service populate
	time.Sleep(20 * time.Second)

	for {
		p.pollOnce(ctx)
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

func (p *Poller) pollOnce(ctx context.Context) {
	// Distributed lock so we don't duplicate work if scaled
	ok, err := p.c.TryLock("ge-poller", 30*time.Minute)
	if err != nil || !ok {
		p.log.Debug("poller lock not acquired, skipping")
		return
	}
	defer p.c.Unlock("ge-poller")

	ids, err := p.fetchAllItemIDs(ctx)
	if err != nil || len(ids) == 0 {
		p.log.Warn("no item ids", zap.Error(err))
		return
	}

	batchSize := p.cfg.WeirdGloop.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}

	now := time.Now().UTC()
	all := make([]models.PriceSnapshot, 0, len(ids))

	for start := 0; start < len(ids); start += batchSize {
		end := start + batchSize
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[start:end]

		latest, err := p.wg.LatestBatch(batch)
		if err != nil {
			p.log.Warn("batch fetch failed",
				zap.Int("size", len(batch)), zap.Error(err))
			continue
		}
		for id, item := range latest {
			snap := models.PriceSnapshot{
				ItemID:    id,
				Timestamp: now,
				BuyPrice:  item.Price,
				SellPrice: item.Price,
				Volume:    item.Volume,
			}
			all = append(all, snap)

			// publish per-item event for realtime-service
			payload, _ := json.Marshal(snap)
			_ = p.c.Publish(fmt.Sprintf("prices:%d", id), string(payload))
		}
	}

	if err := p.repo.InsertSnapshots(all); err != nil {
		p.log.Error("insert snapshots", zap.Error(err))
		return
	}
	p.log.Info("poller tick complete",
		zap.Int("items", len(all)),
		zap.Int("batches", (len(ids)+batchSize-1)/batchSize),
	)
}

func (p *Poller) fetchAllItemIDs(ctx context.Context) ([]int64, error) {
	u := p.cfg.Services.RecipeURL + "/internal/all-item-ids"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var payload struct {
		ItemIDs []int64 `json:"item_ids"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode ids: %w body=%s", err, string(body[:min(len(body), 200)]))
	}
	return payload.ItemIDs, nil
}

func min(a, b int) int { if a < b { return a }; return b }

// silence unused import warning
var _ = strconv.Itoa
