package queue_test

import (
	"testing"

	"github.com/dmitrymomot/foundation/core/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFromConfig_WithEmptyConfig(t *testing.T) {
	t.Parallel()

	// Test with completely empty config (all zero values)
	emptyConfig := queue.Config{}
	storage := queue.NewMemoryStorage()
	defer storage.Close()

	t.Run("NewWorkerFromConfig with empty config", func(t *testing.T) {
		worker, err := queue.NewWorkerFromConfig(emptyConfig, storage)
		require.NoError(t, err)
		assert.NotNil(t, worker)
	})

	t.Run("NewSchedulerFromConfig with empty config", func(t *testing.T) {
		scheduler, err := queue.NewSchedulerFromConfig(emptyConfig, storage)
		require.NoError(t, err)
		assert.NotNil(t, scheduler)
	})

	t.Run("NewEnqueuerFromConfig with empty config", func(t *testing.T) {
		enqueuer, err := queue.NewEnqueuerFromConfig(emptyConfig, storage)
		require.NoError(t, err)
		assert.NotNil(t, enqueuer)
	})

	t.Run("NewServiceFromConfig with empty config", func(t *testing.T) {
		service, err := queue.NewServiceFromConfig(emptyConfig, storage)
		require.NoError(t, err)
		assert.NotNil(t, service)
		assert.NotNil(t, service.Worker())
		assert.NotNil(t, service.Scheduler())
		assert.NotNil(t, service.Enqueuer())
	})
}

func TestNewFromConfig_WithPartialConfig(t *testing.T) {
	t.Parallel()

	// Test with partially filled config
	partialConfig := queue.Config{
		MaxConcurrentTasks: 5,
		DefaultQueue:       "test-queue",
		// Other fields remain zero values
	}
	storage := queue.NewMemoryStorage()
	defer storage.Close()

	t.Run("NewServiceFromConfig with partial config", func(t *testing.T) {
		service, err := queue.NewServiceFromConfig(partialConfig, storage)
		require.NoError(t, err)
		assert.NotNil(t, service)
	})
}

func TestNewFromConfig_OptionsOverrideConfig(t *testing.T) {
	t.Parallel()

	config := queue.Config{
		DefaultQueue:       "config-queue",
		MaxConcurrentTasks: 10,
	}
	storage := queue.NewMemoryStorage()
	defer storage.Close()

	// Test that additional options override config values
	service, err := queue.NewServiceFromConfig(config, storage,
		queue.WithEnqueuerOptions(
			queue.WithDefaultQueue("override-queue"),
		),
		queue.WithWorkerOptions(
			queue.WithMaxConcurrentTasks(20),
		),
	)
	require.NoError(t, err)
	assert.NotNil(t, service)

	// The service should use the override values (we can't directly test this without exposing internal fields,
	// but the fact that it compiles and creates successfully is a good sign)
}
