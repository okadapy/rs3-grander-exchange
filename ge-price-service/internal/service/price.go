package service

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/rs3-market/backend/ge-price-service/internal/client"
	"github.com/rs3-market/backend/ge-price-service/internal/liquidity"
	"github.com/rs3-market/backend/ge-price-service/internal/repository"
	"github.com/rs3-market/backend/shared/cache"
	"github.com/rs3-market/backend/shared/models"
)

type Service struct {
	log    *zap.Logger
	repo   *repository.Repo
	c      *cache.Cache
	recipe *client.RecipeClient
}

func New(repo *repository.Repo, log *zap.Logger, c *cache.Cache, recipe *client.RecipeClient) *Service {
	return &Service{repo: repo, log: log, c: c, recipe: recipe}
}

// Latest returns the latest snapshot for each requested item ID.
func (s *Service) Latest(ids []int64) ([]models.PriceSnapshot, error) {
	return s.repo.LatestForItems(ids)
}

func (s *Service) History(itemID int64, from, to time.Time) ([]models.PriceSnapshot, error) {
	return s.repo.History(itemID, from, to)
}

// Change is the price movement over `window`, expressed both in GP and
// as a percentage.
type Change struct {
	ItemID      int64                 `json:"item_id"`
	WindowHours float64               `json:"window_hours"`
	ChangePct   float64               `json:"change_pct"`
	ChangeGP    int64                 `json:"change_gp"`
	Current     *models.PriceSnapshot `json:"current"`
	Previous    *models.PriceSnapshot `json:"previous"`
	HasBaseline bool                  `json:"has_baseline"`
}

// ChangeOver compares the newest price against the closest snapshot at
// or before now-window.
//
// HasBaseline distinguishes "the price did not move" from "we have no
// history that far back" — two very different things to a trader, and
// indistinguishable if we only returned 0.
func (s *Service) ChangeOver(itemID int64, window time.Duration, now time.Time) (*Change, error) {
	cur, err := s.repo.LatestForItems([]int64{itemID})
	if err != nil {
		return nil, err
	}
	out := &Change{ItemID: itemID, WindowHours: window.Hours()}
	if len(cur) == 0 {
		return out, nil
	}
	out.Current = &cur[0]

	prev, err := s.repo.PriceAt(itemID, now.Add(-window))
	if err != nil || prev == nil {
		return out, nil
	}
	out.Previous = prev
	out.HasBaseline = true
	out.ChangeGP = cur[0].Price - prev.Price
	if prev.Price != 0 {
		out.ChangePct = float64(out.ChangeGP) / float64(prev.Price) * 100.0
	}
	return out, nil
}

// Stats returns the tradeability summary for one item over `window`.
// A failure to reach recipe-service degrades to a stats block with no
// buy limit rather than failing the request — the price-derived signals
// are still useful without it.
func (s *Service) Stats(ctx context.Context, itemID int64, window time.Duration, now time.Time) (liquidity.Stats, error) {
	snaps, err := s.repo.History(itemID, now.Add(-window), now)
	if err != nil {
		return liquidity.Stats{}, err
	}

	limit := 0
	if s.recipe != nil {
		if limits, err := s.recipe.BuyLimits(ctx, []int64{itemID}); err != nil {
			s.log.Debug("buy limit lookup failed", zap.Int64("item_id", itemID))
		} else {
			limit = limits[itemID]
		}
	}
	return liquidity.Compute(itemID, snaps, limit, now), nil
}
