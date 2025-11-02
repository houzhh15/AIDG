// pkg/similarity/cache.go
package similarity

import (
	"container/list"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// QueryVectorCache LRU缓存查询向量（容量100条）
type QueryVectorCache struct {
	mu       sync.RWMutex
	capacity int
	cache    map[string]*list.Element
	lru      *list.List
}

type cacheEntry struct {
	key    string
	vector []float64
}

// NewQueryVectorCache 创建查询向量缓存
func NewQueryVectorCache(capacity int) *QueryVectorCache {
	return &QueryVectorCache{
		capacity: capacity,
		cache:    make(map[string]*list.Element),
		lru:      list.New(),
	}
}

// Get 获取缓存的查询向量
func (c *QueryVectorCache) Get(queryText string) ([]float64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := hashKey(queryText)
	if elem, ok := c.cache[key]; ok {
		// 移到链表头部（最近使用）
		c.lru.MoveToFront(elem)
		entry := elem.Value.(*cacheEntry)
		return entry.vector, true
	}
	return nil, false
}

// Put 缓存查询向量
func (c *QueryVectorCache) Put(queryText string, vector []float64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := hashKey(queryText)

	// 如果已存在，更新并移到头部
	if elem, ok := c.cache[key]; ok {
		c.lru.MoveToFront(elem)
		entry := elem.Value.(*cacheEntry)
		entry.vector = vector
		return
	}

	// 检查容量，超出则删除最久未使用的
	if c.lru.Len() >= c.capacity {
		oldest := c.lru.Back()
		if oldest != nil {
			c.lru.Remove(oldest)
			oldEntry := oldest.Value.(*cacheEntry)
			delete(c.cache, oldEntry.key)
		}
	}

	// 添加新条目到头部
	entry := &cacheEntry{key: key, vector: vector}
	elem := c.lru.PushFront(entry)
	c.cache[key] = elem
}

// RecommendationResultCache 推荐结果缓存（带TTL）
type RecommendationResultCache struct {
	mu    sync.RWMutex
	cache map[string]*resultCacheEntry
	ttl   time.Duration
}

type resultCacheEntry struct {
	results   []RecommendationResult
	expiresAt time.Time
}

// NewRecommendationResultCache 创建推荐结果缓存
func NewRecommendationResultCache(ttl time.Duration) *RecommendationResultCache {
	cache := &RecommendationResultCache{
		cache: make(map[string]*resultCacheEntry),
		ttl:   ttl,
	}

	// 启动定期清理过期条目的goroutine
	go cache.cleanupExpired()

	return cache
}

// Get 获取缓存的推荐结果
func (c *RecommendationResultCache) Get(projectID, queryText, docType string) ([]RecommendationResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := cacheKey(projectID, queryText, docType)
	if entry, ok := c.cache[key]; ok {
		// 检查是否过期
		if time.Now().Before(entry.expiresAt) {
			return entry.results, true
		}
	}
	return nil, false
}

// Put 缓存推荐结果
func (c *RecommendationResultCache) Put(projectID, queryText, docType string, results []RecommendationResult) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := cacheKey(projectID, queryText, docType)
	c.cache[key] = &resultCacheEntry{
		results:   results,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// cleanupExpired 定期清理过期条目（每分钟）
func (c *RecommendationResultCache) cleanupExpired() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, entry := range c.cache {
			if now.After(entry.expiresAt) {
				delete(c.cache, key)
			}
		}
		c.mu.Unlock()
	}
}

// hashKey 生成查询文本的哈希键
func hashKey(text string) string {
	hash := sha256.Sum256([]byte(text))
	return hex.EncodeToString(hash[:])
}

// cacheKey 生成推荐结果缓存键
func cacheKey(projectID, queryText, docType string) string {
	return hashKey(projectID + ":" + queryText + ":" + docType)
}
