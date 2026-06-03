package redisutil

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestJitteredTTL(t *testing.T) {
	base := 60 * time.Second
	maxJitter := 10 * time.Second

	for i := 0; i < 100; i++ {
		ttl := JitteredTTL(base, maxJitter)
		assert.True(t, ttl >= base && ttl < base+maxJitter,
			"expected TTL in [%v, %v), got %v", base, base+maxJitter, ttl)
	}
}

func TestJitteredTTL_NoJitter(t *testing.T) {
	base := 60 * time.Second
	assert.Equal(t, base, JitteredTTL(base, 0))
}

func TestDefaultOpts(t *testing.T) {
	opts := DefaultOpts("localhost:6379", "pass", 1)
	assert.Equal(t, "localhost:6379", opts.Addr)
	assert.Equal(t, "pass", opts.Password)
	assert.Equal(t, 1, opts.DB)
	assert.Equal(t, 20, opts.PoolSize)
	assert.Equal(t, 5, opts.MinIdleConns)
}
