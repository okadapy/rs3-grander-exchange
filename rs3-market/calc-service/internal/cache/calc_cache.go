package cache

import (
	"encoding/json"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/rs3-market/backend/shared/cache"
)

type CalcCache struct {
	Cache *cache.Cache
	TTL   time.Duration
}

func New(c *cache.Cache) *CalcCache {
	return &CalcCache{Cache: c, TTL: 10 * time.Minute}
}

func (cc *CalcCache) Get(key string, v interface{}) bool {
	raw, err := cc.Cache.Get(key)
	if err != nil {
		return false
	}
	return json.Unmarshal([]byte(raw), v) == nil
}

func (cc *CalcCache) Set(key string, v interface{}) {
	body, err := json.Marshal(v)
	if err != nil {
		return
	}
	_ = cc.Cache.Set(key, string(body), cc.TTL)
}

// Expose the underlying redis for rate-limiting in chat (used by realtime).
func (cc *CalcCache) Raw() *goredis.Client { return cc.Cache.RDB }
