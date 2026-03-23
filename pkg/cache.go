package pkg

import (
	"math"

	lru "github.com/hashicorp/golang-lru/v2"
)

type cacheKey struct {
	lat, lng int64
}

const precision = 1e4 // 4 decimal places ≈ 11m

func quantize(v float64) int64 {
	return int64(math.Round(v * precision))
}

type lruCache struct {
	c *lru.Cache[cacheKey, *Result]
}

func newLRUCache(capacity int) *lruCache {
	c, _ := lru.New[cacheKey, *Result](capacity)
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
