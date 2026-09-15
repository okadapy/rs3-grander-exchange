package poller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/rs3-market/backend/ge-price-service/internal/repository"
	"github.com/rs3-market/backend/shared/cache"
	"github.com/rs3-market/backend/shared/config"
	"github.com/rs3-market/backend/shared/models"
	"github.com/rs3-market/backend/shared/weirdgloop"
)

const pollerLockKey = "ge-poller"
const pollerLockTTL = 20 * time.Minute

// Weirdgloop accepts pipe-separated IDs but the request URL must stay
// reasonable. 100 IDs at ~7 chars each is ~700 bytes — well under any
// proxy limit and small enough to keep individual responses fast.
const weirdgloopBatchSize = 100

type Poller struct {
	cfg        *config.Config
	log        *zap.Logger
	repo       *repository.Repo
	c          *cache.Cache
	wg         *weirdgloop.Client
	httpClient *http.Client
}

func New(cfg *config.Config, repo *repository.Repo, c *cache.Cache,
	wg *weirdgloop.Client, log *zap.Logger) *Poller {
	return &Poller{
		cfg: cfg, repo: repo, c: c, wg: wg, log: log,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (p *Poller) Run(ctx context.Context) {
	interval := p.cfg.Poller.PollInterval
	if interval <= 0 {
		interval = time.Hour
	}
	p.log.Info("poller starting", zap.Duration("interval", interval))

	tick := time.NewTicker(interval)
	defer tick.Stop()

	time.Sleep(15 * time.Second)

	for {
		p.PollOnce(ctx)
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

func (p *Poller) PollOnce(ctx context.Context) {
	ok, err := p.c.TryLock(pollerLockKey, pollerLockTTL)
	if err != nil {
		p.log.Warn("poller lock error", zap.Error(err))
		return
	}
	if !ok {
		ttl, _ := p.c.RDB.TTL(p.c.Ctx, "lock:"+pollerLockKey).Result()
		p.log.Info("poller lock held by another instance, skipping",
			zap.Duration("expires_in", ttl))
		return
	}
	defer p.c.Unlock(pollerLockKey)

	p.log.Info("poll cycle starting")

	ids, err := p.fetchWantedItemIDs(ctx)
	if err != nil {
		p.log.Warn("fetch item ids failed", zap.Error(err))
		return
	}
	if len(ids) == 0 {
		p.log.Warn("recipe-service returned zero item ids — is a scrape complete?")
		return
	}
	p.log.Info("wanted item ids", zap.Int("count", len(ids)))

	batchSize := p.cfg.WeirdGloop.BatchSize
	if batchSize <= 0 {
		batchSize = weirdgloopBatchSize
	}

	now := time.Now().UTC()
	all := make([]models.PriceSnapshot, 0, len(ids))
	okBatches, failBatches, missing := 0, 0, 0

	for start := 0; start < len(ids); start += batchSize {
		end := start + batchSize
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[start:end]

		latest, err := p.wg.LatestBatch(batch)
		if err != nil {
			failBatches++
			p.log.Warn("batch fetch failed",
				zap.Int("offset", start),
				zap.Int("size", len(batch)),
				zap.Error(err))
			continue
		}
		okBatches++
		if len(latest) == 0 {
			// Entire batch was untradeable. Not a failure — just
			// nothing to record. Log at debug so it doesn't spam
			// the operator.
			p.log.Debug("batch all-untradeable",
				zap.Int("offset", start),
				zap.Int("size", len(batch)))
		}

		for id, item := range latest {
			if item.Price <= 0 {
				continue
			}
			snap := models.PriceSnapshot{
				ItemID:    id,
				Timestamp: now,
				Price:     item.Price,
				Volume:    item.Volume,
			}
			all = append(all, snap)

			payload, _ := json.Marshal(snap)
			_ = p.c.Publish(fmt.Sprintf("prices:%d", id), string(payload))
		}
		missing += len(batch) - len(latest)
	}

	p.log.Info("batches complete",
		zap.Int("ok", okBatches),
		zap.Int("failed", failBatches),
		zap.Int("missing_from_api", missing))

	if len(all) == 0 {
		p.log.Warn("no price rows to insert")
		return
	}
	if err := p.repo.InsertSnapshots(all); err != nil {
		p.log.Error("insert snapshots", zap.Error(err))
		return
	}
	p.log.Info("poll cycle complete", zap.Int("inserted", len(all)))
}

// ClearLock forcibly removes the distributed lock. For dev use only.
// Unlock is fire-and-forget, so this is void.
func (p *Poller) ClearLock() {
	p.c.Unlock(pollerLockKey)
}

func (p *Poller) fetchWantedItemIDs(ctx context.Context) ([]int64, error) {
	u := p.cfg.Services.RecipeURL + "/internal/all-item-ids"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", u, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("recipe-service returned %d: %s",
			resp.StatusCode, truncate(body, 200))
	}

	var payload struct {
		ItemIDs []int64 `json:"item_ids"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode ids: %w body=%s",
			err, truncate(body, 200))
	}
	return payload.ItemIDs, nil
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
