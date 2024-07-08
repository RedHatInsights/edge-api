// A simple memory-only thread-safe cache with TTL. Taken from the provisioning package:
// https://github.com/RHEnVision/provisioning-backend/commit/decfb331a2e5642e904bed0dcd3ac41b319eb732
package cache

import (
	"runtime"
	"sync"
	"time"
)

// Cache stores arbitrary data with expiration time.
type Cache[K comparable, V any] struct {
	items   map[K]*item[V]
	mu      sync.Mutex
	done    chan any
	clean   chan bool
	once    sync.Once
	cleanWG sync.WaitGroup
}

// An item represents arbitrary data with expiration time.
type item[V any] struct {
	data    V
	expires int64
}

// New creates a new cache that asynchronously cleans
// expired entries after the given time passes. If cleaningInterval
// is zero, no background cleanup goroutine is scheduled.
func NewMemoryCache[K comparable, V any](cleaningInterval time.Duration) *Cache[K, V] {
	cache := &Cache[K, V]{
		items: make(map[K]*item[V]),
		clean: make(chan bool),
		done:  make(chan any),
	}

	if cleaningInterval != 0 {
		go func() {
			ticker := time.NewTicker(cleaningInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					cache.cleanup()
				case <-cache.clean:
					cache.cleanup()
					cache.cleanWG.Done()
				case <-cache.done:
					return
				}
			}
		}()
	}

	// Shutdown the goroutine when GC wants to clean this up
	runtime.SetFinalizer(cache, func(c *Cache[K, V]) {
		c.Stop()
	})

	return cache
}

// cleanup function is called from the background goroutine
func (cache *Cache[K, V]) cleanup() {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	now := time.Now().UnixNano()
	for key, item := range cache.items {
		if item.expires > 0 && now > item.expires {
			delete(cache.items, key)
		}
	}
}

// Get gets the value for the given key.
func (cache *Cache[K, V]) Get(key K) (V, bool) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	item, exists := cache.items[key]
	if !exists || (item.expires > 0 && time.Now().UnixNano() > item.expires) {
		var nothing V
		return nothing, false
	}

	return item.data, true
}

// Set sets a value for the given key with an expiration duration.
// If the duration is 0 or less, it will be stored forever.
func (cache *Cache[K, V]) Set(key K, value V, duration time.Duration) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	var expires int64
	if duration > 0 {
		expires = time.Now().Add(duration).UnixNano()
	}
	cache.items[key] = &item[V]{
		data:    value,
		expires: expires,
	}
}

// Count contains count of cached items.
func (cache *Cache[K, V]) Count() int {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	return len(cache.items)
}

// ExpireNow schedules immediate expiration cycle. It blocks, until cleanup is completed.
// If cleanup interval is zero, this will block forever.
func (cache *Cache[K, V]) ExpireNow() {
	cache.cleanWG.Add(1)
	cache.clean <- true
	cache.cleanWG.Wait()
}

// Stop frees up resources and stops the cleanup goroutine
func (cache *Cache[K, V]) Stop() {
	cache.once.Do(func() {
		cache.items = make(map[K]*item[V])
		close(cache.done)
	})
}
