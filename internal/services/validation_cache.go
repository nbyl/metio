package services

import (
	"sync"
	"time"
)

type validationCache struct {
	mu        sync.RWMutex
	result    *ValidationResult
	expiresAt time.Time
	ttl       time.Duration
}

func newValidationCache(ttl time.Duration) *validationCache {
	return &validationCache{ttl: ttl}
}

func (c *validationCache) Get() (*ValidationResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.result != nil && time.Now().Before(c.expiresAt) {
		return c.result, true
	}
	return nil, false
}

func (c *validationCache) Set(r *ValidationResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.result = r
	c.expiresAt = time.Now().Add(c.ttl)
}

func (c *validationCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.result = nil
	c.expiresAt = time.Time{}
}
