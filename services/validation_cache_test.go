package services

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestValidationCache_GetEmpty(t *testing.T) {
	c := newValidationCache(5 * time.Minute)
	result, ok := c.Get()
	assert.Nil(t, result)
	assert.False(t, ok)
}

func TestValidationCache_SetAndGet(t *testing.T) {
	c := newValidationCache(5 * time.Minute)
	expected := &ValidationResult{Valid: true}
	c.Set(expected)
	result, ok := c.Get()
	assert.True(t, ok)
	assert.Equal(t, expected, result)
}

func TestValidationCache_Expire(t *testing.T) {
	c := newValidationCache(10 * time.Millisecond)
	c.Set(&ValidationResult{Valid: true})

	_, ok := c.Get()
	assert.True(t, ok)

	time.Sleep(20 * time.Millisecond)

	_, ok = c.Get()
	assert.False(t, ok)
}

func TestValidationCache_Invalidate(t *testing.T) {
	c := newValidationCache(5 * time.Minute)
	c.Set(&ValidationResult{Valid: true})
	c.Invalidate()
	_, ok := c.Get()
	assert.False(t, ok)
}

func TestValidationCache_Concurrent(t *testing.T) {
	c := newValidationCache(5 * time.Minute)
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Set(&ValidationResult{Valid: true})
			c.Get()
			c.Invalidate()
		}()
	}
	wg.Wait()
}
