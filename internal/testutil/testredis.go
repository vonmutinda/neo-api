package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

// MustStartRedis spins up an ephemeral Redis 7 container and returns its URL.
func MustStartRedis(t *testing.T) string {
	t.Helper()
	ctx := context.Background()

	rc, err := redis.Run(ctx,
		"redis:7-alpine",
		testcontainers.WithWaitStrategy(
			wait.ForLog("Ready to accept connections").
				WithStartupTimeout(15*time.Second),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rc.Terminate(ctx) })

	url, err := rc.ConnectionString(ctx)
	require.NoError(t, err)
	return url
}
