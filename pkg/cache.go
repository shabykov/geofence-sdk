package pkg

import (
	"container/list"
	"math"
	"sync"
)

// lruCache caches Lookup results keyed by quantized coordinates.
// Points are snapped to a ~11m grid (4 decimal places).
type lruCache struct {
	mu       sync.Mutex
	capacity int
	items    map[cacheKey]*list.Element
	order    *list.List
}

type cacheKey struct {
	lat, lng int64
}

type cacheEntry struct {
	key    cacheKey
	result *Result
}

const precision = 1e4 // 4 decimal places ≈ 11m

func quantize(v float64) int64 {
	return int64(math.Round(v * precision))
}

func newLRUCache(capacity int) *lruCache {
	return &lruCache{
		capacity: capacity,
		items:    make(map[cacheKey]*list.Element, capacity),
		order:    list.New(),
	}
}

func (c *lruCache) get(lat, lng float64) (*Result, bool) {
	key := cacheKey{quantize(lat), quantize(lng)}

	c.mu.Lock()
	defer c.mu.Unlock()

	if el, ok := c.items[key]; ok {
		c.order.MoveToFront(el)
		return el.Value.(*cacheEntry).result, true
	}
	return nil, false
}

func (c *lruCache) put(lat, lng float64, result *Result) {
	key := cacheKey{quantize(lat), quantize(lng)}

	c.mu.Lock()
	defer c.mu.Unlock()

	if el, ok := c.items[key]; ok {
		c.order.MoveToFront(el)
		el.Value.(*cacheEntry).result = result
		return
	}

	if c.order.Len() >= c.capacity {
		oldest := c.order.Back()
		if oldest != nil {
			c.order.Remove(oldest)
			delete(c.items, oldest.Value.(*cacheEntry).key)
		}
	}

	entry := &cacheEntry{key: key, result: result}
	el := c.order.PushFront(entry)
	c.items[key] = el
}

func (c *lruCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[cacheKey]*list.Element, c.capacity)
	c.order.Init()
}
