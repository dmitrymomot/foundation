package queue_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dmitrymomot/foundation/core/queue"
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
}
