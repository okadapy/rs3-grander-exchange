package chat

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/rs3-market/backend/shared/cache"
	"github.com/rs3-market/backend/shared/models"
)

type Service struct {
	DB              *gorm.DB
	c               *cache.Cache
	log             *zap.Logger
	RateLimitWindow time.Duration
	HistoryLimit    int
}

func New(db *gorm.DB, c *cache.Cache, log *zap.Logger, rateLimit time.Duration, history int) *Service {
	return &Service{DB: db, c: c, log: log, RateLimitWindow: rateLimit, HistoryLimit: history}
}

// Allow enforces a 1-message-per-window rate limit per user using Redis INCR.
func (s *Service) Allow(userID uint) bool {
	key := fmt.Sprintf("chat:rl:%d", userID)
	n, err := s.c.RDB.Incr(s.c.Ctx, key).Result()
	if err != nil {
		return true // fail open
	}
	if n == 1 {
		s.c.RDB.Expire(s.c.Ctx, key, s.RateLimitWindow)
	}
	return n <= 1
}

func (s *Service) Save(userID uint, username, body string) (*models.ChatMessage, error) {
	m := &models.ChatMessage{
		UserID:    userID,
		Username:  username,
		Body:      body,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.DB.Create(m).Error; err != nil {
		return nil, err
	}
	return m, nil
}

func (s *Service) History(before time.Time, limit int) ([]models.ChatMessage, error) {
	if limit <= 0 {
		limit = s.HistoryLimit
	}
	q := s.DB.Order("created_at DESC").Limit(limit)
	if !before.IsZero() {
		q = q.Where("created_at < ?", before)
	}
	var out []models.ChatMessage
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	// reverse to chronological order
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// Subscriber fans out price:pubsub into the hub.
func (s *Service) RunPriceSubscriber(ctx context.Context, onMessage func(channel, payload string)) {
	sub := s.c.PSubscribe("prices:*")
	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			_ = sub.Close()
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			onMessage(msg.Channel, msg.Payload)
		}
	}
}
