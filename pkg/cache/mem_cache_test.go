package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSetAndGetFound(t *testing.T) {
	c := NewMemoryCache[string, string](0)
	c.Set("hello", "Hello", 0)
	hello, found := c.Get("hello")
	assert.True(t, found)
	assert.Equal(t, "Hello", hello)
}

func TestSetAndGetNotFound(t *testing.T) {
	c := NewMemoryCache[string, string](0)
	_, found := c.Get("does not exist")
	assert.False(t, found)
}

func TestManualExpiration(t *testing.T) {
	c := NewMemoryCache[string, string](time.Minute)
	c.Set("short", "expiration", time.Nanosecond)
	c.ExpireNow()

	_, found := c.Get("short")
	assert.False(t, found)
}

func TestExpiration(t *testing.T) {
	c := NewMemoryCache[string, string](10 * time.Millisecond)
	c.Set("short", "expiration", time.Nanosecond)
	defer c.Stop()

	// hope for the best
	time.Sleep(100 * time.Millisecond)

	_, found := c.Get("short")
	assert.False(t, found)
}

func BenchmarkNew(b *testing.B) {
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			NewMemoryCache[string, string](5 * time.Second).Stop()
		}
	})
}

func BenchmarkGet(b *testing.B) {
	c := NewMemoryCache[string, string](5 * time.Second)
	defer c.Stop()
	c.Set("Hello", "World", 0)

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.Get("Hello")
		}
	})
}

func BenchmarkSet(b *testing.B) {
	c := NewMemoryCache[string, string](5 * time.Second)
	defer c.Stop()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.Set("Hello", "World", 0)
		}
	})
}
