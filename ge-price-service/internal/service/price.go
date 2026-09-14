package service

import (
	"time"

	"go.uber.org/zap"

	"github.com/rs3-market/backend/shared/cache"
	"github.com/rs3-market/backend/shared/models"
	"github.com/rs3-market/backend/shared/weirdgloop"
	"github.com/rs3-market/backend/ge-price-service/internal/repository"
)

type Service struct {
	log  *zap.Logger
	repo *repository.Repo
	c    *cache.Cache
	wg   *weirdgloop.Client
}

func New(repo *repository.Repo, log *zap.Logger, c *cache.Cache, wg *weirdgloop.Client) *Service {
	return &Service{repo: repo, log: log, c: c, wg: wg}
}

// Latest returns the latest snapshots for the requested item IDs.
func (s *Service) Latest(ids []int64) ([]models.PriceSnapshot, error) {
	return s.repo.LatestForItems(ids)
}

func (s *Service) History(itemID int64, from, to time.Time) ([]models.PriceSnapshot, error) {
	return s.repo.History(itemID, from, to)
}

// Change24h returns (changePct, current, prev, err).
func (s *Service) Change24h(itemID int64) (float64, *models.PriceSnapshot, *models.PriceSnapshot, error) {
	cur, err := s.repo.LatestForItems([]int64{itemID})
	if err != nil || len(cur) == 0 {
		return 0, nil, nil, err
	}
	prev, err := s.repo.Price24hAgo(itemID, time.Now().UTC())
	if err != nil || prev == nil {
		return 0, &cur[0], nil, nil
	}
	if prev.BuyPrice == 0 {
		return 0, &cur[0], prev, nil
	}
	pct := float64(cur[0].BuyPrice-prev.BuyPrice) / float64(prev.BuyPrice) * 100.0
	return pct, &cur[0], prev, nil
}
