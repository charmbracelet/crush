package cache

import "sync"

// Cache is a plain string-keyed in-memory cache.
type Cache struct {
	mu    sync.Mutex
	items map[string]string
}

// New returns an empty Cache.
func New() *Cache {
	return &Cache{items: make(map[string]string)}
}

// Get returns the value for key, or "" and false when absent.
func (c *Cache) Get(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.items[key]
	return v, ok
}

// Set stores value under key.
func (c *Cache) Set(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = value
}

// Delete removes key. Missing keys are not an error.
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

// Len reports how many entries are held.
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.items)
}
