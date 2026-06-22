package ratelimit

import (
	"sync"

	"golang.org/x/time/rate"
)

// UserRateLimiter provides per-user token-bucket rate limiting.
// Each user gets their own rate.Limiter created on first access.
type UserRateLimiter struct {
	mu       sync.RWMutex
	users    map[string]*rate.Limiter
	rate     rate.Limit
	burst    int
}

// NewUserRateLimiter creates a new per-user rate limiter.
// r: allowed events per second (rate.Limit type is float64).
// b: maximum burst size (bucket capacity).
func NewUserRateLimiter(r float64, b int) *UserRateLimiter {
	return &UserRateLimiter{
		users: make(map[string]*rate.Limiter),
		rate:  rate.Limit(r),
		burst: b,
	}
}

// Allow reports whether an event for the given userID may happen now.
// It consumes one token from the user's bucket if available.
func (ul *UserRateLimiter) Allow(userID string) bool {
	limiter := ul.getLimiter(userID)
	return limiter.Allow()
}

// Reserve returns a Reservation that indicates how long the caller must wait
// before events happen for the given userID.
func (ul *UserRateLimiter) Reserve(userID string) *rate.Reservation {
	limiter := ul.getLimiter(userID)
	return limiter.Reserve()
}

// Tokens returns the number of available tokens for the given userID.
func (ul *UserRateLimiter) Tokens(userID string) float64 {
	limiter := ul.getLimiter(userID)
	return limiter.Tokens()
}

// SetRate dynamically updates the per-user rate.
func (ul *UserRateLimiter) SetRate(r float64) {
	ul.mu.Lock()
	defer ul.mu.Unlock()
	ul.rate = rate.Limit(r)
	// Update existing limiters
	for _, limiter := range ul.users {
		limiter.SetLimit(ul.rate)
	}
}

// SetBurst dynamically updates the per-user burst.
func (ul *UserRateLimiter) SetBurst(b int) {
	ul.mu.Lock()
	defer ul.mu.Unlock()
	ul.burst = b
	for _, limiter := range ul.users {
		limiter.SetBurst(b)
	}
}

// Cleanup removes limiters that have been idle for long periods.
// maxUsers caps the map size to prevent unbounded memory growth.
func (ul *UserRateLimiter) Cleanup(maxUsers int) {
	ul.mu.Lock()
	defer ul.mu.Unlock()
	if len(ul.users) <= maxUsers {
		return
	}
	// Simple eviction: remove half the entries
	// A production system would use an LRU or similar strategy.
	count := 0
	toRemove := len(ul.users) - maxUsers
	for userID := range ul.users {
		if count >= toRemove {
			break
		}
		delete(ul.users, userID)
		count++
	}
}

// getLimiter returns (or creates) the rate.Limiter for the given userID.
func (ul *UserRateLimiter) getLimiter(userID string) *rate.Limiter {
	ul.mu.RLock()
	limiter, exists := ul.users[userID]
	ul.mu.RUnlock()
	if exists {
		return limiter
	}

	ul.mu.Lock()
	defer ul.mu.Unlock()

	// Double-check after acquiring write lock
	if limiter, exists = ul.users[userID]; exists {
		return limiter
	}

	limiter = rate.NewLimiter(ul.rate, ul.burst)
	ul.users[userID] = limiter
	return limiter
}
