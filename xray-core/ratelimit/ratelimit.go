package ratelimit

import (
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

// idleEvictAfter is how long a limiter must be unused before Cleanup may drop
// it when the map is already at or below the cap. Idle entries are recreated
// lazily on the next access.
const idleEvictAfter = 10 * time.Minute

// userEntry holds a user's rate limiter plus the time of its last access.
// lastAccess stores unix nanoseconds so it can be updated atomically on the
// read path without taking the map's write lock for every Allow call.
type userEntry struct {
	limiter    *rate.Limiter
	lastAccess atomic.Int64
}

// UserRateLimiter provides per-user token-bucket rate limiting.
// Each user gets their own rate.Limiter created on first access.
type UserRateLimiter struct {
	mu    sync.RWMutex
	users map[string]*userEntry
	rate  rate.Limit
	burst int
}

// NewUserRateLimiter creates a new per-user rate limiter.
// r: allowed events per second (rate.Limit type is float64).
// b: maximum burst size (bucket capacity).
func NewUserRateLimiter(r float64, b int) *UserRateLimiter {
	return &UserRateLimiter{
		users: make(map[string]*userEntry),
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
	for _, entry := range ul.users {
		entry.limiter.SetLimit(ul.rate)
	}
}

// SetBurst dynamically updates the per-user burst.
func (ul *UserRateLimiter) SetBurst(b int) {
	ul.mu.Lock()
	defer ul.mu.Unlock()
	ul.burst = b
	for _, entry := range ul.users {
		entry.limiter.SetBurst(b)
	}
}

// Cleanup removes limiters that have been idle for long periods.
// maxUsers caps the map size to prevent unbounded memory growth. When the map
// exceeds maxUsers, the least-recently-used entries are evicted first; below
// the cap, entries idle beyond idleEvictAfter are dropped.
func (ul *UserRateLimiter) Cleanup(maxUsers int) {
	ul.mu.Lock()
	defer ul.mu.Unlock()

	if len(ul.users) > maxUsers {
		ul.evictOldest(len(ul.users) - maxUsers)
		return
	}

	// Under the cap: drop entries idle beyond the threshold. They are
	// recreated lazily on the next access.
	idleCutoff := time.Now().Add(-idleEvictAfter).UnixNano()
	for userID, entry := range ul.users {
		if entry.lastAccess.Load() < idleCutoff {
			delete(ul.users, userID)
		}
	}
}

// evictOldest removes the n least-recently-used entries. Each pass locates
// the oldest entry with a single linear scan; n is small in practice because
// Cleanup is invoked periodically, so no full sort is needed.
func (ul *UserRateLimiter) evictOldest(n int) {
	for i := 0; i < n && len(ul.users) > 0; i++ {
		var oldestID string
		var oldestTS int64
		first := true
		for userID, entry := range ul.users {
			ts := entry.lastAccess.Load()
			if first || ts < oldestTS {
				oldestTS = ts
				oldestID = userID
				first = false
			}
		}
		delete(ul.users, oldestID)
	}
}

// getLimiter returns (or creates) the rate.Limiter for the given userID,
// stamping the entry as recently used.
func (ul *UserRateLimiter) getLimiter(userID string) *rate.Limiter {
	now := time.Now().UnixNano()

	ul.mu.RLock()
	entry, exists := ul.users[userID]
	ul.mu.RUnlock()
	if exists {
		entry.lastAccess.Store(now)
		return entry.limiter
	}

	ul.mu.Lock()
	defer ul.mu.Unlock()

	// Double-check after acquiring write lock
	if entry, exists = ul.users[userID]; exists {
		entry.lastAccess.Store(now)
		return entry.limiter
	}

	limiter := rate.NewLimiter(ul.rate, ul.burst)
	entry = &userEntry{limiter: limiter}
	entry.lastAccess.Store(now)
	ul.users[userID] = entry
	return limiter
}
