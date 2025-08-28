package feature_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dmitrymomot/foundation/pkg/feature"
)

func TestHashConsistency(t *testing.T) {
	t.Run("Different flags have independent percentage rollouts", func(t *testing.T) {
		// Create provider with two flags using the same percentage strategy
		strategy := feature.NewTargetedStrategy(feature.TargetCriteria{
			Percentage: ptr(30), // 30% rollout
		})

		provider, err := feature.NewMemoryProvider(
			&feature.Flag{
				Name:     "feature-a",
				Enabled:  true,
				Strategy: strategy,
			},
			&feature.Flag{
				Name:     "feature-b",
				Enabled:  true,
				Strategy: strategy,
			},
		)
		require.NoError(t, err)
		defer provider.Close()

		// Test with multiple users to verify independence
		// With proper hashing, approximately 30% of users should get each feature
		// but the sets shouldn't be identical
		usersWithFeatureA := 0
		usersWithFeatureB := 0
		usersWithBoth := 0
		totalUsers := 1000

		for i := 0; i < totalUsers; i++ {
			userID := fmt.Sprintf("user-%d", i)
			ctx := feature.WithUserID(context.Background(), userID)

			hasA, err := provider.IsEnabled(ctx, "feature-a")
			require.NoError(t, err)

			hasB, err := provider.IsEnabled(ctx, "feature-b")
			require.NoError(t, err)

			if hasA {
				usersWithFeatureA++
			}
			if hasB {
				usersWithFeatureB++
			}
			if hasA && hasB {
				usersWithBoth++
			}
		}

		// Debug output
		t.Logf("Users with feature A: %d (%.1f%%)", usersWithFeatureA, float64(usersWithFeatureA)/float64(totalUsers)*100)
		t.Logf("Users with feature B: %d (%.1f%%)", usersWithFeatureB, float64(usersWithFeatureB)/float64(totalUsers)*100)
		t.Logf("Users with both: %d (%.1f%%)", usersWithBoth, float64(usersWithBoth)/float64(totalUsers)*100)

		// Both features should have approximately 30% of users (±10% tolerance)
		assert.InDelta(t, 300, usersWithFeatureA, 100, "Feature A should have ~30%% of users")
		assert.InDelta(t, 300, usersWithFeatureB, 100, "Feature B should have ~30%% of users")

		// Users with both features should be approximately 9% (30% * 30% = 9%)
		// This verifies independence - if they were dependent, it would be 30%
		// Allow wider tolerance as this is probabilistic
		expectedBoth := float64(totalUsers) * 0.3 * 0.3 // 9% of users
		assert.InDelta(t, expectedBoth, usersWithBoth, expectedBoth*0.7, "Users with both features should be ~9%% (independent)")

		// Most importantly, verify they're not identical (which would give 30% overlap)
		assert.Less(t, usersWithBoth, usersWithFeatureA/2, "Overlap should be much less than 50% of users with feature A")
	})

	t.Run("Same flag gives consistent results for same user", func(t *testing.T) {
		strategy := feature.NewTargetedStrategy(feature.TargetCriteria{
			Percentage: ptr(50),
		})

		provider, err := feature.NewMemoryProvider(
			&feature.Flag{
				Name:     "consistent-feature",
				Enabled:  true,
				Strategy: strategy,
			},
		)
		require.NoError(t, err)
		defer provider.Close()

		// Test the same user multiple times
		ctx := feature.WithUserID(context.Background(), "test-user-123")

		// Get initial result
		firstResult, err := provider.IsEnabled(ctx, "consistent-feature")
		require.NoError(t, err)

		// Verify consistency across multiple calls
		for i := 0; i < 100; i++ {
			result, err := provider.IsEnabled(ctx, "consistent-feature")
			require.NoError(t, err)
			assert.Equal(t, firstResult, result, "Same user should always get same result")
		}
	})
}

// Helper function for percentage pointer
func ptr(i int) *int {
	return &i
}
