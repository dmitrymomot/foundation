package session_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/dmitrymomot/foundation/core/session"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Use the existing MockStore from manager_test.go
type ConfigTestStore[Data any] struct {
	mock.Mock
}

func (m *ConfigTestStore[Data]) Get(ctx context.Context, token string) (session.Session[Data], error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return session.Session[Data]{}, args.Error(1)
	}
	return args.Get(0).(session.Session[Data]), args.Error(1)
}

func (m *ConfigTestStore[Data]) Store(ctx context.Context, sess session.Session[Data]) error {
	args := m.Called(ctx, sess)
	return args.Error(0)
}

func (m *ConfigTestStore[Data]) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// Use the existing MockTransport pattern from manager_test.go
type ConfigTestTransport struct {
	mock.Mock
}

func (m *ConfigTestTransport) Extract(r *http.Request) (string, error) {
	args := m.Called(r)
	return args.String(0), args.Error(1)
}

func (m *ConfigTestTransport) Embed(w http.ResponseWriter, r *http.Request, token string, ttl time.Duration) error {
	args := m.Called(w, r, token, ttl)
	return args.Error(0)
}

func (m *ConfigTestTransport) Revoke(w http.ResponseWriter, r *http.Request) error {
	args := m.Called(w, r)
	return args.Error(0)
}

func TestNewFromConfig(t *testing.T) {
	t.Run("creates manager from config with env defaults", func(t *testing.T) {
		// Use default config
		cfg := session.DefaultConfig()

		// Create mock store and transport
		store := &ConfigTestStore[string]{}
		transport := &ConfigTestTransport{}

		// Create manager from config
		manager, err := session.NewFromConfig(
			cfg,
			session.WithStore[string](store),
			session.WithTransport[string](transport),
		)

		require.NoError(t, err)
		assert.NotNil(t, manager)
	})

	t.Run("allows overriding config values with options", func(t *testing.T) {
		// Custom config
		cfg := session.Config{
			TTL:           12 * time.Hour,
			TouchInterval: 10 * time.Minute,
		}

		// Create mock store and transport
		store := &ConfigTestStore[string]{}
		transport := &ConfigTestTransport{}

		// Create manager from config but override TTL
		// Note: WithConfig can still be used with NewFromConfig to override values
		manager, err := session.NewFromConfig(
			cfg,
			session.WithStore[string](store),
			session.WithTransport[string](transport),
			session.WithConfig[string](session.WithTTL(6*time.Hour)), // Override
		)

		require.NoError(t, err)
		assert.NotNil(t, manager)
	})

	t.Run("fails without required store", func(t *testing.T) {
		cfg := session.DefaultConfig()
		transport := &ConfigTestTransport{}

		manager, err := session.NewFromConfig(
			cfg,
			session.WithTransport[string](transport),
		)

		assert.Error(t, err)
		assert.Nil(t, manager)
		assert.Equal(t, session.ErrNoStore, err)
	})

	t.Run("fails without required transport", func(t *testing.T) {
		cfg := session.DefaultConfig()
		store := &ConfigTestStore[string]{}

		manager, err := session.NewFromConfig(
			cfg,
			session.WithStore[string](store),
		)

		assert.Error(t, err)
		assert.Nil(t, manager)
		assert.Equal(t, session.ErrNoTransport, err)
	})

	t.Run("handles zero values in config", func(t *testing.T) {
		// Config with zero values
		cfg := session.Config{}

		// Create mock store and transport
		store := &ConfigTestStore[string]{}
		transport := &ConfigTestTransport{}

		// Should not apply zero values, using defaults instead
		manager, err := session.NewFromConfig(
			cfg,
			session.WithStore[string](store),
			session.WithTransport[string](transport),
		)

		require.NoError(t, err)
		assert.NotNil(t, manager)
	})
}
