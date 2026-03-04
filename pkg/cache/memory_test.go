package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryCache_SetAndGet(t *testing.T) {
	ctx := context.Background()
	c := NewMemoryCache()
	defer c.Close()

	err := c.Set(ctx, "key1", []byte("value1"), 0)
	require.NoError(t, err)

	val, ok := c.Get(ctx, "key1")
	require.True(t, ok)
	assert.Equal(t, []byte("value1"), val)
}

func TestMemoryCache_GetMissing(t *testing.T) {
	ctx := context.Background()
	c := NewMemoryCache()
	defer c.Close()

	_, ok := c.Get(ctx, "nonexistent")
	assert.False(t, ok)
}

func TestMemoryCache_Delete(t *testing.T) {
	ctx := context.Background()
	c := NewMemoryCache()
	defer c.Close()

	require.NoError(t, c.Set(ctx, "key1", []byte("value1"), 0))
	require.NoError(t, c.Delete(ctx, "key1"))

	_, ok := c.Get(ctx, "key1")
	assert.False(t, ok)
}

func TestMemoryCache_TTLExpiry(t *testing.T) {
	ctx := context.Background()
	c := NewMemoryCache()
	defer c.Close()

	require.NoError(t, c.Set(ctx, "key1", []byte("value1"), 50*time.Millisecond))
	time.Sleep(100 * time.Millisecond)

	_, ok := c.Get(ctx, "key1")
	assert.False(t, ok)
}

func TestMemoryCache_NoTTL(t *testing.T) {
	ctx := context.Background()
	c := NewMemoryCache()
	defer c.Close()

	require.NoError(t, c.Set(ctx, "key1", []byte("value1"), 0))
	time.Sleep(50 * time.Millisecond)

	val, ok := c.Get(ctx, "key1")
	require.True(t, ok)
	assert.Equal(t, []byte("value1"), val)
}

func TestMemoryCache_DeleteByPrefix(t *testing.T) {
	ctx := context.Background()
	c := NewMemoryCache()
	defer c.Close()

	require.NoError(t, c.Set(ctx, "rate:ETB", []byte("57.5"), 0))
	require.NoError(t, c.Set(ctx, "rate:USD", []byte("1.0"), 0))
	require.NoError(t, c.Set(ctx, "other:1", []byte("x"), 0))

	require.NoError(t, c.DeleteByPrefix(ctx, "rate:"))

	_, ok := c.Get(ctx, "rate:ETB")
	assert.False(t, ok)
	_, ok = c.Get(ctx, "rate:USD")
	assert.False(t, ok)
	val, ok := c.Get(ctx, "other:1")
	require.True(t, ok)
	assert.Equal(t, []byte("x"), val)
}

func TestMemoryCache_Increment(t *testing.T) {
	ctx := context.Background()
	c := NewMemoryCache()
	defer c.Close()

	n, err := c.Increment(ctx, "counter", 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	n, err = c.Increment(ctx, "counter", 0)
	require.NoError(t, err)
	assert.Equal(t, int64(2), n)
}

func TestMemoryCache_Ping(t *testing.T) {
	ctx := context.Background()
	c := NewMemoryCache()
	defer c.Close()

	assert.NoError(t, c.Ping(ctx))
}
