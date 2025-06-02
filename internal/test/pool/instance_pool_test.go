//go:build pool

package pool

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yourorg/shellbox/internal/infra"
	"github.com/yourorg/shellbox/internal/test"
)

func TestInstancePoolScalingUp(t *testing.T) {
	if !test.ShouldRunCategory(test.CategoryPool) {
		t.Skip("Skipping pool tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	// Setup test environment
	clients, vmConfig, cleanup := test.SetupTestEnvironment(t, "pool-inst-scale-up")
	defer cleanup()

	// Create mock golden snapshot for testing
	goldenSnapshot := &infra.GoldenSnapshotInfo{
		ResourceID:      "test-golden-snapshot-id",
		SnapshotName:    "test-golden-snapshot",
		CreationTime:    time.Now(),
		DiskSizeGB:      100,
		ResourceGroupRG: "test-golden-rg",
	}

	// Use development pool config for faster testing
	poolConfig := infra.NewDevPoolConfig()
	poolConfig.MinFreeInstances = 2
	poolConfig.MaxFreeInstances = 4
	poolConfig.CheckInterval = 5 * time.Second
	poolConfig.ScaleDownCooldown = 10 * time.Second

	// Create pool
	pool := infra.NewBoxPool(clients, vmConfig, poolConfig, goldenSnapshot)

	t.Run("ScaleUpFromEmpty", func(t *testing.T) {
		// Setup resource queries to check counts
		resourceQueries := infra.NewResourceGraphQueries(
			clients.ResourceGraphClient,
			clients.SubscriptionID,
			clients.ResourceGroupName,
		)

		// Initially, there should be no instances
		counts, err := resourceQueries.CountInstancesByStatus(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, counts.Free, "Should start with no free instances")

		// Trigger scale up by running maintenance once
		// Since maintainInstancePool is private, we'll run a short maintenance cycle
		maintCtx, maintCancel := context.WithTimeout(ctx, 15*time.Second)
		defer maintCancel()
		go pool.MaintainPool(maintCtx)

		// Wait for async operations to complete
		time.Sleep(45 * time.Second)

		// Check that instances were created
		countsAfter, err := resourceQueries.CountInstancesByStatus(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, countsAfter.Free, poolConfig.MinFreeInstances,
			"Should have created at least MinFreeInstances")
	})

	t.Run("ScaleUpFromBelowMinimum", func(t *testing.T) {
		// Simulate having 1 free instance (below minimum of 2)
		initialCount := 1

		// Setup resource queries
		resourceQueries := infra.NewResourceGraphQueries(
			clients.ResourceGraphClient,
			clients.SubscriptionID,
			clients.ResourceGroupName,
		)

		// Get current counts
		counts, err := resourceQueries.CountInstancesByStatus(ctx)
		require.NoError(t, err)

		// If we already have enough instances, remove some to get below minimum
		if counts.Free >= poolConfig.MinFreeInstances {
			instancesToRemove := counts.Free - initialCount
			oldestInstances, err := resourceQueries.GetOldestFreeInstances(ctx, instancesToRemove)
			require.NoError(t, err)

			for _, instance := range oldestInstances {
				err := infra.DeleteInstance(ctx, clients, clients.ResourceGroupName, instance.Name)
				require.NoError(t, err)
			}

			// Wait for deletion to complete
			time.Sleep(10 * time.Second)
		}

		// Verify we're below minimum
		countsBefore, err := resourceQueries.CountInstancesByStatus(ctx)
		require.NoError(t, err)
		require.Less(t, countsBefore.Free, poolConfig.MinFreeInstances,
			"Should have fewer than MinFreeInstances before test")

		// Trigger scale up
		maintCtx, maintCancel := context.WithTimeout(ctx, 15*time.Second)
		defer maintCancel()
		go pool.MaintainPool(maintCtx)
		time.Sleep(45 * time.Second)

		// Verify scale up occurred
		countsAfter, err := resourceQueries.CountInstancesByStatus(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, countsAfter.Free, poolConfig.MinFreeInstances,
			"Should have scaled up to MinFreeInstances")
	})

	t.Run("NoScaleWhenWithinRange", func(t *testing.T) {
		// Ensure we have instances within the acceptable range
		counts, err := pool.CountInstancesByStatus(ctx)
		require.NoError(t, err)

		if counts.Free < poolConfig.MinFreeInstances || counts.Free > poolConfig.MaxFreeInstances {
			// Adjust to be within range (3 instances for our config)
			targetCount := (poolConfig.MinFreeInstances + poolConfig.MaxFreeInstances) / 2

			if counts.Free < targetCount {
				// Need to create more
				pool.MaintainInstancePool(ctx)
				time.Sleep(30 * time.Second)
			} else if counts.Free > targetCount {
				// Need to remove some
				instancesToRemove := counts.Free - targetCount
				oldestInstances, err := pool.GetOldestFreeInstances(ctx, instancesToRemove)
				require.NoError(t, err)

				for _, instance := range oldestInstances {
					err := infra.DeleteInstance(ctx, clients, clients.ResourceGroupName, instance.Name)
					require.NoError(t, err)
				}
				time.Sleep(10 * time.Second)
			}
		}

		// Record counts before maintenance
		countsBefore, err := pool.CountInstancesByStatus(ctx)
		require.NoError(t, err)
		require.GreaterOrEqual(t, countsBefore.Free, poolConfig.MinFreeInstances)
		require.LessOrEqual(t, countsBefore.Free, poolConfig.MaxFreeInstances)

		// Run maintenance - should not change counts
		pool.MaintainInstancePool(ctx)
		time.Sleep(10 * time.Second)

		// Verify no changes
		countsAfter, err := pool.CountInstancesByStatus(ctx)
		require.NoError(t, err)
		assert.Equal(t, countsBefore.Free, countsAfter.Free,
			"Should not scale when within acceptable range")
	})
}

func TestInstancePoolScalingDown(t *testing.T) {
	if !test.ShouldRunCategory(test.CategoryPool) {
		t.Skip("Skipping pool tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	// Setup test environment
	clients, vmConfig, cleanup := test.SetupTestEnvironment(t, "pool-inst-scale-down")
	defer cleanup()

	// Create mock golden snapshot
	goldenSnapshot := &infra.GoldenSnapshotInfo{
		ResourceID:      "test-golden-snapshot-id",
		SnapshotName:    "test-golden-snapshot",
		CreationTime:    time.Now(),
		DiskSizeGB:      100,
		ResourceGroupRG: "test-golden-rg",
	}

	// Use development pool config for faster testing
	poolConfig := infra.NewDevPoolConfig()
	poolConfig.MinFreeInstances = 1
	poolConfig.MaxFreeInstances = 3
	poolConfig.CheckInterval = 5 * time.Second
	poolConfig.ScaleDownCooldown = 5 * time.Second // Short cooldown for testing

	// Create pool
	pool := infra.NewBoxPool(clients, vmConfig, poolConfig, goldenSnapshot)

	t.Run("ScaleDownFromAboveMaximum", func(t *testing.T) {
		// Create instances above maximum (5 instances vs max of 3)
		targetInstances := poolConfig.MaxFreeInstances + 2

		// First ensure we have enough instances
		for i := 0; i < targetInstances; i++ {
			_, err := infra.CreateInstance(ctx, clients, vmConfig)
			require.NoError(t, err)
		}

		// Wait for creation to complete
		time.Sleep(30 * time.Second)

		// Verify we have more than maximum
		countsBefore, err := pool.CountInstancesByStatus(ctx)
		require.NoError(t, err)
		require.Greater(t, countsBefore.Free, poolConfig.MaxFreeInstances,
			"Should have more than MaxFreeInstances before test")

		// Trigger scale down
		pool.MaintainInstancePool(ctx)
		time.Sleep(30 * time.Second)

		// Verify scale down occurred
		countsAfter, err := pool.CountInstancesByStatus(ctx)
		require.NoError(t, err)
		assert.LessOrEqual(t, countsAfter.Free, poolConfig.MaxFreeInstances,
			"Should have scaled down to MaxFreeInstances")
		assert.Less(t, countsAfter.Free, countsBefore.Free,
			"Should have removed some instances")
	})

	t.Run("ScaleDownCooldownPrevention", func(t *testing.T) {
		// Create instances above maximum
		targetInstances := poolConfig.MaxFreeInstances + 2

		for i := 0; i < targetInstances; i++ {
			_, err := infra.CreateInstance(ctx, clients, vmConfig)
			require.NoError(t, err)
		}
		time.Sleep(30 * time.Second)

		// First scale down
		pool.MaintainInstancePool(ctx)
		time.Sleep(20 * time.Second)

		countsAfterFirst, err := pool.CountInstancesByStatus(ctx)
		require.NoError(t, err)

		// Immediate second scale down attempt (should be prevented by cooldown)
		pool.MaintainInstancePool(ctx)
		time.Sleep(5 * time.Second)

		countsAfterSecond, err := pool.CountInstancesByStatus(ctx)
		require.NoError(t, err)
		assert.Equal(t, countsAfterFirst.Free, countsAfterSecond.Free,
			"Cooldown should prevent immediate second scale down")

		// Wait for cooldown to expire and try again
		time.Sleep(poolConfig.ScaleDownCooldown + time.Second)

		// Add more instances to trigger another scale down
		for i := 0; i < 2; i++ {
			_, err := infra.CreateInstance(ctx, clients, vmConfig)
			require.NoError(t, err)
		}
		time.Sleep(20 * time.Second)

		pool.MaintainInstancePool(ctx)
		time.Sleep(20 * time.Second)

		countsAfterCooldown, err := pool.CountInstancesByStatus(ctx)
		require.NoError(t, err)
		assert.Less(t, countsAfterCooldown.Free, countsAfterSecond.Free+2,
			"Should scale down after cooldown expires")
	})
}

func TestInstancePoolResourceAllocation(t *testing.T) {
	if !test.ShouldRunCategory(test.CategoryPool) {
		t.Skip("Skipping pool tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()

	// Setup test environment
	clients, vmConfig, cleanup := test.SetupTestEnvironment(t, "pool-inst-alloc")
	defer cleanup()

	// Create mock golden snapshot
	goldenSnapshot := &infra.GoldenSnapshotInfo{
		ResourceID:      "test-golden-snapshot-id",
		SnapshotName:    "test-golden-snapshot",
		CreationTime:    time.Now(),
		DiskSizeGB:      100,
		ResourceGroupRG: "test-golden-rg",
	}

	poolConfig := infra.NewDevPoolConfig()
	poolConfig.MinFreeInstances = 2
	poolConfig.MaxFreeInstances = 4
	poolConfig.MaxTotalInstances = 6

	pool := infra.NewBoxPool(clients, vmConfig, poolConfig, goldenSnapshot)

	t.Run("RespectMaxTotalInstances", func(t *testing.T) {
		// Create instances up to the maximum total limit
		for i := 0; i < poolConfig.MaxTotalInstances; i++ {
			_, err := infra.CreateInstance(ctx, clients, vmConfig)
			require.NoError(t, err)
		}
		time.Sleep(30 * time.Second)

		// Try to trigger scale up (should fail due to max total limit)
		// First, mark some instances as connected to reduce free count
		instances, err := pool.GetInstancesByStatus(ctx, infra.ResourceStatusFree)
		require.NoError(t, err)
		require.Greater(t, len(instances), 0, "Should have some free instances")

		// Simulate marking half as connected
		for i := 0; i < len(instances)/2 && i < 3; i++ {
			// Update instance status to connected (this would normally be done by the SSH server)
			tags := infra.InstanceTags{
				Role:       infra.ResourceRoleInstance,
				Status:     infra.ResourceStatusConnected,
				CreatedAt:  time.Now().Format(time.RFC3339),
				LastUsed:   time.Now().Format(time.RFC3339),
				InstanceID: instances[i].ResourceID,
			}

			err := infra.UpdateInstanceTags(ctx, clients, clients.ResourceGroupName,
				instances[i].Name, tags)
			require.NoError(t, err)
		}
		time.Sleep(10 * time.Second)

		// Check counts
		counts, err := pool.CountInstancesByStatus(ctx)
		require.NoError(t, err)
		require.Equal(t, poolConfig.MaxTotalInstances, counts.Total,
			"Should be at max total instances")
		require.Less(t, counts.Free, poolConfig.MinFreeInstances,
			"Should have fewer free instances than minimum")

		// Try to scale up - should not exceed max total
		pool.MaintainInstancePool(ctx)
		time.Sleep(30 * time.Second)

		countsAfter, err := pool.CountInstancesByStatus(ctx)
		require.NoError(t, err)
		assert.LessOrEqual(t, countsAfter.Total, poolConfig.MaxTotalInstances,
			"Should not exceed MaxTotalInstances even when below MinFreeInstances")
	})

	t.Run("CorrectInstanceTagging", func(t *testing.T) {
		// Create a single instance and verify its tags
		instanceID, err := infra.CreateInstance(ctx, clients, vmConfig)
		require.NoError(t, err)
		time.Sleep(20 * time.Second)

		// Get the instance details
		instances, err := pool.GetAllInstances(ctx)
		require.NoError(t, err)

		var testInstance *infra.ResourceInfo
		for _, instance := range instances {
			if instance.ResourceID == instanceID {
				testInstance = &instance
				break
			}
		}
		require.NotNil(t, testInstance, "Should find the created instance")

		// Verify tags
		assert.Equal(t, infra.ResourceRoleInstance, testInstance.Role,
			"Instance should have correct role tag")
		assert.Equal(t, infra.ResourceStatusFree, testInstance.Status,
			"New instance should have free status")
		assert.NotNil(t, testInstance.CreatedAt, "Instance should have creation time")
		assert.NotNil(t, testInstance.LastUsed, "Instance should have last used time")
		assert.Equal(t, instanceID, testInstance.ResourceID,
			"Instance should have correct instance ID")
	})
}

func TestInstancePoolConcurrentOperations(t *testing.T) {
	if !test.ShouldRunCategory(test.CategoryPool) {
		t.Skip("Skipping pool tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()

	// Setup test environment
	clients, vmConfig, cleanup := test.SetupTestEnvironment(t, "pool-inst-concurrent")
	defer cleanup()

	// Create mock golden snapshot
	goldenSnapshot := &infra.GoldenSnapshotInfo{
		ResourceID:      "test-golden-snapshot-id",
		SnapshotName:    "test-golden-snapshot",
		CreationTime:    time.Now(),
		DiskSizeGB:      100,
		ResourceGroupRG: "test-golden-rg",
	}

	poolConfig := infra.NewDevPoolConfig()
	poolConfig.MinFreeInstances = 3
	poolConfig.MaxFreeInstances = 6
	poolConfig.CheckInterval = 2 * time.Second

	pool := infra.NewBoxPool(clients, vmConfig, poolConfig, goldenSnapshot)

	t.Run("ConcurrentScaleOperations", func(t *testing.T) {
		// Start concurrent maintenance operations
		maintCtx, maintCancel := context.WithTimeout(ctx, 2*time.Minute)
		defer maintCancel()

		// Run maintenance in background
		go pool.MaintainPool(maintCtx)

		// Simultaneously create and delete instances manually to create fluctuations
		go func() {
			for i := 0; i < 3; i++ {
				time.Sleep(15 * time.Second)
				_, err := infra.CreateInstance(ctx, clients, vmConfig)
				if err == nil {
					t.Logf("Manually created instance %d", i)
				}
			}
		}()

		go func() {
			time.Sleep(30 * time.Second)
			instances, err := pool.GetInstancesByStatus(ctx, infra.ResourceStatusFree)
			if err == nil && len(instances) > 0 {
				err := infra.DeleteInstance(ctx, clients, clients.ResourceGroupName, instances[0].Name)
				if err == nil {
					t.Logf("Manually deleted instance %s", instances[0].Name)
				}
			}
		}()

		// Let it run for a while
		time.Sleep(90 * time.Second)
		maintCancel()

		// Check final state - should be stable within limits
		finalCounts, err := pool.CountInstancesByStatus(ctx)
		require.NoError(t, err)

		t.Logf("Final counts: Free=%d, Connected=%d, Total=%d",
			finalCounts.Free, finalCounts.Connected, finalCounts.Total)

		// The pool should maintain reasonable bounds despite concurrent operations
		assert.LessOrEqual(t, finalCounts.Total, poolConfig.MaxTotalInstances,
			"Should not exceed max total instances")
		// Note: We can't guarantee minimum due to concurrent deletions, but it should be reasonable
		assert.LessOrEqual(t, finalCounts.Free, poolConfig.MaxFreeInstances*2,
			"Should not have excessive free instances")
	})
}

func TestInstancePoolErrorHandling(t *testing.T) {
	if !test.ShouldRunCategory(test.CategoryPool) {
		t.Skip("Skipping pool tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	// Setup test environment
	clients, vmConfig, cleanup := test.SetupTestEnvironment(t, "pool-inst-errors")
	defer cleanup()

	t.Run("GracefulDegradationWithAzureErrors", func(t *testing.T) {
		// Create pool with invalid golden snapshot to simulate errors
		invalidGoldenSnapshot := &infra.GoldenSnapshotInfo{
			ResourceID:      "invalid-snapshot-id",
			SnapshotName:    "invalid-snapshot",
			CreationTime:    time.Now(),
			DiskSizeGB:      100,
			ResourceGroupRG: "invalid-rg",
		}

		poolConfig := infra.NewDevPoolConfig()
		poolConfig.MinFreeInstances = 1
		poolConfig.MaxFreeInstances = 2

		pool := infra.NewBoxPool(clients, vmConfig, poolConfig, invalidGoldenSnapshot)

		// This should handle errors gracefully without panicking
		assert.NotPanics(t, func() {
			pool.MaintainInstancePool(ctx)
		}, "Pool maintenance should handle Azure errors gracefully")

		// The pool should still be able to get current counts
		counts, err := pool.CountInstancesByStatus(ctx)
		require.NoError(t, err)
		t.Logf("Counts with error conditions: Free=%d, Total=%d", counts.Free, counts.Total)
	})
}
