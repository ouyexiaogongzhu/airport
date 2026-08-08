package ratelimit

import (
	"sync"
	"testing"
	"time"
)

// TestNewUserRateLimiter verifies that a new limiter is created with the correct
// rate and burst values.
func TestNewUserRateLimiter(t *testing.T) {
	r := 10.0
	b := 5
	limiter := NewUserRateLimiter(r, b)

	if limiter == nil {
		t.Fatal("NewUserRateLimiter returned nil")
	}
	if limiter.rate != 10.0 {
		t.Errorf("expected rate 10.0, got %f", limiter.rate)
	}
	if limiter.burst != 5 {
		t.Errorf("expected burst 5, got %d", limiter.burst)
	}
	if len(limiter.users) != 0 {
		t.Errorf("expected empty users map, got %d entries", len(limiter.users))
	}
}

// TestAllow_UnderLimit verifies that Allow returns true when we stay within
// the configured rate but do not exceed burst.
func TestAllow_UnderLimit(t *testing.T) {
	// Rate = 1000/s (generous), burst = 3 → 3 immediate tokens
	limiter := NewUserRateLimiter(1000, 3)
	uid := "user-1"

	// All three burst tokens should succeed
	for i := 0; i < 3; i++ {
		if !limiter.Allow(uid) {
			t.Errorf("iteration %d: expected Allow to succeed (within burst)", i)
		}
	}
}

// TestAllow_ExceedsRate verifies that calling Allow() more than the configured
// rate per second eventually returns false when burst is fully consumed.
func TestAllow_ExceedsRate(t *testing.T) {
	// Rate = 10/s, burst = 1 → only 1 immediate allow
	limiter := NewUserRateLimiter(10, 1)
	uid := "test-user"

	// First allow should succeed
	if !limiter.Allow(uid) {
		t.Error("expected first Allow to succeed")
	}

	// Second immediate allow should fail (burst=1)
	if limiter.Allow(uid) {
		t.Error("expected second Allow to fail (burst exhausted)")
	}

	// Wait for rate to replenish
	time.Sleep(100 * time.Millisecond) // gives ~1 token at 10/s

	if !limiter.Allow(uid) {
		t.Error("expected Allow to succeed after waiting")
	}
}

// TestAllow_Burst verifies that burst capacity is respected: calling Allow()
// burst times returns true, but one more call returns false immediately.
func TestAllow_Burst(t *testing.T) {
	limiter := NewUserRateLimiter(1, 4) // 1/s rate, burst 4
	uid := "burst-user"

	// Consume all 4 burst tokens immediately
	for i := 0; i < 4; i++ {
		if !limiter.Allow(uid) {
			t.Errorf("burst iteration %d: expected Allow to succeed", i)
		}
	}

	// Fifth call should fail — burst exhausted, rate is only 1/s
	if limiter.Allow(uid) {
		t.Error("expected Allow to fail after burst exhausted")
	}
}

// TestTokens verifies that Tokens() returns the correct number of available tokens.
func TestTokens(t *testing.T) {
	limiter := NewUserRateLimiter(100, 5) // high rate, burst 5
	uid := "token-user"

	// Fresh user should have full burst worth of tokens
	tokens := limiter.Tokens(uid)
	if tokens != 5.0 {
		t.Errorf("expected 5.0 tokens for new user, got %f", tokens)
	}

	// After consuming 2 tokens, we should have ~3 left
	limiter.Allow(uid)
	limiter.Allow(uid)

	tokens = limiter.Tokens(uid)
	if tokens < 2.5 || tokens > 3.5 {
		t.Errorf("expected ~3 tokens after consuming 2, got %f", tokens)
	}
}

// TestSetRate verifies that dynamically changing the rate takes effect.
func TestSetRate(t *testing.T) {
	limiter := NewUserRateLimiter(1, 5) // 1/s, burst 5
	uid := "rate-user"

	// Consume all 5 burst tokens
	for i := 0; i < 5; i++ {
		limiter.Allow(uid)
	}

	// Sixth call should fail (no tokens left)
	if limiter.Allow(uid) {
		t.Error("expected Allow to fail before set rate")
	}

	// Increase rate to 1000/s
	limiter.SetRate(1000)

	// Wait briefly for tokens to accumulate
	time.Sleep(10 * time.Millisecond)

	// Now Allow should succeed because the high rate replenishes quickly
	if !limiter.Allow(uid) {
		t.Error("expected Allow to succeed after increasing rate")
	}
}

// TestSetBurst verifies that dynamically changing the burst takes effect.
func TestSetBurst(t *testing.T) {
	limiter := NewUserRateLimiter(1000, 2) // high rate, burst 2
	uid := "burst-set-user"

	// Consume initial burst
	limiter.Allow(uid)
	limiter.Allow(uid)

	// Should fail now
	if limiter.Allow(uid) {
		t.Error("expected Allow to fail before set burst")
	}

	// Increase burst to 5
	limiter.SetBurst(5)

	// Give a tiny amount of time for the high rate (1000/s) to replenish a token
	time.Sleep(5 * time.Millisecond)

	// Now Allow should succeed (burst was expanded and token replenished)
	if !limiter.Allow(uid) {
		t.Error("expected Allow to succeed after increasing burst")
	}
}

// TestCleanup verifies that Cleanup removes excess users when the map grows
// beyond the specified limit.
func TestCleanup(t *testing.T) {
	limiter := NewUserRateLimiter(10, 1)

	// Add 10 users
	for i := 0; i < 10; i++ {
		uid := "cleanup-user-" + string(rune('0'+i))
		limiter.Allow(uid)
	}

	if len(limiter.users) != 10 {
		t.Fatalf("expected 10 users, got %d", len(limiter.users))
	}

	// Cleanup down to 3 users — should remove 7
	limiter.Cleanup(3)

	if len(limiter.users) > 3 {
		t.Errorf("expected at most 3 users after cleanup, got %d", len(limiter.users))
	}

	// Cleanup when already under limit should be a no-op
	limiter.Cleanup(5)
	if len(limiter.users) > 3 {
		t.Errorf("expected at most 3 users after no-op cleanup, got %d", len(limiter.users))
	}
}

// TestCleanup_EvictsLeastRecentlyUsed verifies that the least-recently-used
// entries are evicted first, rather than an arbitrary subset.
func TestCleanup_EvictsLeastRecentlyUsed(t *testing.T) {
	limiter := NewUserRateLimiter(10, 1)

	// Access user-A first, then user-B, then user-C. lastAccess stamps differ
	// by at least a couple of milliseconds.
	limiter.Allow("user-A")
	time.Sleep(2 * time.Millisecond)
	limiter.Allow("user-B")
	time.Sleep(2 * time.Millisecond)
	limiter.Allow("user-C")

	// maxUsers=2 evicts exactly one entry: the least recently used (user-A).
	limiter.Cleanup(2)

	if len(limiter.users) != 2 {
		t.Fatalf("expected 2 users after cleanup, got %d", len(limiter.users))
	}
	if _, ok := limiter.users["user-A"]; ok {
		t.Error("expected least-recently-used user-A to be evicted")
	}
	if _, ok := limiter.users["user-B"]; !ok {
		t.Error("expected user-B to be kept")
	}
	if _, ok := limiter.users["user-C"]; !ok {
		t.Error("expected user-C to be kept")
	}
}

// TestCleanup_ReaccessRefreshesLRU verifies that reusing an entry updates its
// recency, protecting it from eviction.
func TestCleanup_ReaccessRefreshesLRU(t *testing.T) {
	limiter := NewUserRateLimiter(10, 1)

	limiter.Allow("user-A")
	time.Sleep(2 * time.Millisecond)
	limiter.Allow("user-B")
	time.Sleep(2 * time.Millisecond)
	// Re-access user-A: it becomes the most recently used.
	limiter.Allow("user-A")
	time.Sleep(2 * time.Millisecond)
	limiter.Allow("user-C")

	limiter.Cleanup(2)

	if _, ok := limiter.users["user-A"]; !ok {
		t.Error("expected re-accessed user-A to be kept")
	}
	if _, ok := limiter.users["user-B"]; ok {
		t.Error("expected idle user-B to be evicted")
	}
	if _, ok := limiter.users["user-C"]; !ok {
		t.Error("expected user-C to be kept")
	}
}

// TestConcurrentAccess verifies that the rate limiter is goroutine-safe:
// multiple goroutines calling Allow() simultaneously does not cause panics
// or data races.
func TestConcurrentAccess(t *testing.T) {
	limiter := NewUserRateLimiter(100, 5)
	var wg sync.WaitGroup

	// Spawn 20 goroutines hammering the limiter concurrently
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				uid := "concurrent-user-" + string(rune('0'+id%5))
				limiter.Allow(uid)
				limiter.Tokens(uid)
			}
		}(i)
	}

	wg.Wait()
	// If we get here with no panics, the test passes
}
