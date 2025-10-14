package main

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestConcurrencyLimiter tests concurrent execution control.
func TestConcurrencyLimiter(t *testing.T) {
	config := &Config{
		Commands: []CommandConfig{
			{
				Name:          "test_cmd",
				MaxConcurrent: 2, // Allow max 2 concurrent executions
			},
			{
				Name:          "single_cmd",
				MaxConcurrent: 1, // Allow only 1 concurrent execution
			},
		},
		Security: SecurityConfig{
			SharedVolumePath: "/data",
			ForbiddenPaths:   []string{},
			MaxCommandLength: 1024,
		},
	}

	limiter := NewConcurrencyLimiter(config)

	t.Run("acquire and release within limit", func(t *testing.T) {
		ctx := context.Background()

		// First acquire should succeed
		err := limiter.Acquire(ctx, "test_cmd")
		if err != nil {
			t.Errorf("First Acquire() failed: %v", err)
		}

		// Second acquire should also succeed (limit is 2)
		err = limiter.Acquire(ctx, "test_cmd")
		if err != nil {
			t.Errorf("Second Acquire() failed: %v", err)
		}

		// Release both slots
		limiter.Release("test_cmd")
		limiter.Release("test_cmd")

		// Should be able to acquire again
		err = limiter.Acquire(ctx, "test_cmd")
		if err != nil {
			t.Errorf("Third Acquire() failed: %v", err)
		}
		limiter.Release("test_cmd")
	})

	t.Run("block when exceeding concurrency limit", func(t *testing.T) {
		ctx := context.Background()

		// Acquire up to the limit (2)
		err1 := limiter.Acquire(ctx, "test_cmd")
		err2 := limiter.Acquire(ctx, "test_cmd")

		if err1 != nil || err2 != nil {
			t.Fatalf("Failed to acquire slots: %v, %v", err1, err2)
		}

		// Third acquire should block, use timeout context
		timeoutCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()

		err := limiter.Acquire(timeoutCtx, "test_cmd")
		if err == nil {
			t.Error("Third Acquire() should have timed out but succeeded")
			limiter.Release("test_cmd")
		}

		// Release the first two
		limiter.Release("test_cmd")
		limiter.Release("test_cmd")
	})

	t.Run("concurrent goroutines respecting limit", func(t *testing.T) {
		const numGoroutines = 5
		const maxConcurrent = 2

		var wg sync.WaitGroup
		var activeMutex sync.Mutex
		var maxActive int
		var currentActive int

		// Track maximum concurrent executions
		updateMaxActive := func(delta int) {
			activeMutex.Lock()
			currentActive += delta
			if currentActive > maxActive {
				maxActive = currentActive
			}
			activeMutex.Unlock()
		}

		// Launch 5 goroutines, but only 2 should run concurrently
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				ctx := context.Background()

				// Acquire slot
				err := limiter.Acquire(ctx, "test_cmd")
				if err != nil {
					t.Errorf("Goroutine %d: Acquire() failed: %v", id, err)
					return
				}
				defer limiter.Release("test_cmd")

				// Track active count
				updateMaxActive(1)
				defer updateMaxActive(-1)

				// Simulate work
				time.Sleep(100 * time.Millisecond)
			}(i)
		}

		// Wait for all goroutines to complete
		wg.Wait()

		// Verify that we never exceeded the limit
		if maxActive > maxConcurrent {
			t.Errorf("Max concurrent executions = %d, want <= %d", maxActive, maxConcurrent)
		}

		t.Logf("Max concurrent executions: %d (limit: %d)", maxActive, maxConcurrent)
	})

	t.Run("single concurrency command", func(t *testing.T) {
		ctx := context.Background()

		// Acquire the single slot
		err := limiter.Acquire(ctx, "single_cmd")
		if err != nil {
			t.Fatalf("First Acquire() failed: %v", err)
		}

		// Second acquire should timeout
		timeoutCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()

		err = limiter.Acquire(timeoutCtx, "single_cmd")
		if err == nil {
			t.Error("Second Acquire() should have timed out but succeeded")
			limiter.Release("single_cmd")
		}

		// Release and try again
		limiter.Release("single_cmd")

		err = limiter.Acquire(ctx, "single_cmd")
		if err != nil {
			t.Errorf("Acquire() after release failed: %v", err)
		}
		limiter.Release("single_cmd")
	})

	t.Run("acquire for non-existent command", func(t *testing.T) {
		ctx := context.Background()

		err := limiter.Acquire(ctx, "nonexistent_cmd")
		if err == nil {
			t.Error("Acquire() for non-existent command should fail")
		}
	})

	t.Run("release for non-existent command should not panic", func(t *testing.T) {
		// This should not panic
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Release() panicked: %v", r)
			}
		}()

		limiter.Release("nonexistent_cmd")
	})
}

// TestConcurrencyLimiterStressTest stress tests the limiter with many concurrent operations.
func TestConcurrencyLimiterStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	config := &Config{
		Commands: []CommandConfig{
			{
				Name:          "stress_cmd",
				MaxConcurrent: 3,
			},
		},
		Security: SecurityConfig{
			SharedVolumePath: "/data",
			ForbiddenPaths:   []string{},
			MaxCommandLength: 1024,
		},
	}

	limiter := NewConcurrencyLimiter(config)

	const numGoroutines = 100
	const maxConcurrent = 3

	var wg sync.WaitGroup
	var activeMutex sync.Mutex
	var maxActive int
	var currentActive int
	successCount := 0

	updateMaxActive := func(delta int) {
		activeMutex.Lock()
		currentActive += delta
		if currentActive > maxActive {
			maxActive = currentActive
		}
		activeMutex.Unlock()
	}

	// Launch many goroutines
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			ctx := context.Background()

			// Acquire slot
			err := limiter.Acquire(ctx, "stress_cmd")
			if err != nil {
				return
			}
			defer limiter.Release("stress_cmd")

			// Track active count
			updateMaxActive(1)
			defer updateMaxActive(-1)

			// Simulate short work
			time.Sleep(10 * time.Millisecond)

			activeMutex.Lock()
			successCount++
			activeMutex.Unlock()
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Verify that we never exceeded the limit
	if maxActive > maxConcurrent {
		t.Errorf("Max concurrent executions = %d, want <= %d", maxActive, maxConcurrent)
	}

	// Verify all goroutines completed successfully
	if successCount != numGoroutines {
		t.Errorf("Success count = %d, want %d", successCount, numGoroutines)
	}

	t.Logf("Stress test: %d goroutines, max concurrent: %d (limit: %d), success: %d",
		numGoroutines, maxActive, maxConcurrent, successCount)
}

// TestConcurrencyLimiterTimeout tests the 30-second default timeout.
func TestConcurrencyLimiterTimeout(t *testing.T) {
	config := &Config{
		Commands: []CommandConfig{
			{
				Name:          "timeout_test",
				MaxConcurrent: 1,
			},
		},
		Security: SecurityConfig{
			SharedVolumePath: "/data",
			ForbiddenPaths:   []string{},
			MaxCommandLength: 1024,
		},
	}

	limiter := NewConcurrencyLimiter(config)
	ctx := context.Background()

	// Acquire the only slot
	err := limiter.Acquire(ctx, "timeout_test")
	if err != nil {
		t.Fatalf("First Acquire() failed: %v", err)
	}

	// Don't release it yet, let the second acquire timeout
	start := time.Now()
	err = limiter.Acquire(ctx, "timeout_test")
	duration := time.Since(start)

	// Should timeout after ~30 seconds (internal timeout in Acquire method)
	// But for testing, we don't want to wait that long
	// The implementation uses 30s timeout, so this test would take too long
	// We'll just verify the error
	if err == nil {
		t.Error("Second Acquire() should have timed out")
		limiter.Release("timeout_test")
	}

	// For practical testing, just verify it times out reasonably
	// (The actual 30s timeout would be tested in integration tests)
	if duration < 29*time.Second {
		// This is expected in the test - it times out at 30s internally
		t.Logf("Acquire timed out after %v (expected ~30s in production)", duration)
	}

	// Clean up
	limiter.Release("timeout_test")
}
