package pkg

import (
	"math"
	"time"

	lru "github.com/hashicorp/golang-lru/v2/expirable"
)

type cacheKey struct {
	lat, lng int64
}

const (
	precision = 1e4       // 4 decimal places ≈ 11m
	ttl       = time.Hour // expire after 1 hour
)

func quantize(v float64) int64 {
	return int64(math.Round(v * precision))
}

type lruCache struct {
	c *lru.LRU[cacheKey, *Result]
}

func newLRUCache(capacity int) *lruCache {
	c := lru.NewLRU[cacheKey, *Result](capacity, nil, ttl)
	return &lruCache{c: c}
}

func (c *lruCache) get(lat, lng float64) (*Result, bool) {
	return c.c.Get(cacheKey{quantize(lat), quantize(lng)})
}

func (c *lruCache) put(lat, lng float64, result *Result) {
	c.c.Add(cacheKey{quantize(lat), quantize(lng)}, result)
}

func (c *lruCache) clear() {
	c.c.Purge()
}
