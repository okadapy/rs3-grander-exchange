package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/rs3-market/backend/shared/config"
)

type Cache struct {
	RDB *redis.Client
	Ctx context.Context
}

func New(cfg config.RedisConfig) *Cache {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})
	return &Cache{RDB: rdb, Ctx: context.Background()}
}

func (c *Cache) Ping() error {
	return c.RDB.Ping(c.Ctx).Err()
}

func (c *Cache) Get(key string) (string, error) {
	return c.RDB.Get(c.Ctx, key).Result()
}

func (c *Cache) Set(key, val string, ttl time.Duration) error {
	return c.RDB.Set(c.Ctx, key, val, ttl).Err()
}

func (c *Cache) Del(keys ...string) error {
	return c.RDB.Del(c.Ctx, keys...).Err()
}

func (c *Cache) Publish(channel, payload string) error {
	return c.RDB.Publish(c.Ctx, channel, payload).Err()
}

func (c *Cache) Subscribe(channels ...string) *redis.PubSub {
	return c.RDB.Subscribe(c.Ctx, channels...)
}

func (c *Cache) PSubscribe(pattern string) *redis.PubSub {
	return c.RDB.PSubscribe(c.Ctx, pattern)
}

// TryLock returns true if the lock was acquired. Used by the poller to
// prevent duplicate polling when the service is scaled horizontally.
func (c *Cache) TryLock(key string, ttl time.Duration) (bool, error) {
	return c.RDB.SetNX(c.Ctx, "lock:"+key, "1", ttl).Result()
}

func (c *Cache) Unlock(key string) {
	c.RDB.Del(c.Ctx, "lock:"+key)
}
