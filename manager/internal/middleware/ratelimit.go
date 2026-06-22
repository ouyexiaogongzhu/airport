package middleware

import (
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Test helper: reset rate limiter state between tests
var ResetRateLimiter = func() {
	resetGlobalLimiter()
}

func resetGlobalLimiter() {
	globalLimiter.mu.Lock()
	defer globalLimiter.mu.Unlock()
	globalLimiter.windows = make(map[string]*slidingWindow)
}

var (
	rateVisitors sync.Map
	regVisitors  sync.Map
)

// RateGroup defines the rate limit for a group of endpoints.
type RateGroup struct {
	Name      string
	Rate      int // requests per second
}

// Supported rate groups
var (
	RateGroupPublic = RateGroup{Name: "public", Rate: 10}
	RateGroupAuth   = RateGroup{Name: "auth", Rate: 5}
	RateGroupAPI    = RateGroup{Name: "api", Rate: 30}
)

// slidingWindow stores request timestamps for a single IP.
type slidingWindow struct {
	mu    sync.Mutex
	ts    []time.Time // sorted request timestamps, oldest first
}

type rateLimiter struct {
	mu     sync.Mutex
	windows map[string]*slidingWindow // key: "group:ip"
	groups map[string]RateGroup
	stopCh chan struct{}
}

var globalLimiter *rateLimiter

func init() {
	globalLimiter = &rateLimiter{
		windows: make(map[string]*slidingWindow),
		groups: map[string]RateGroup{
			RateGroupPublic.Name: RateGroupPublic,
			RateGroupAuth.Name:   RateGroupAuth,
			RateGroupAPI.Name:    RateGroupAPI,
		},
		stopCh: make(chan struct{}),
	}
	go globalLimiter.cleanupLoop()
}

// cleanupLoop prunes stale entries every minute.
func (rl *rateLimiter) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.prune()
		case <-rl.stopCh:
			return
		}
	}
}

func (rl *rateLimiter) prune() {
	now := time.Now()
	rl.mu.Lock()
	defer rl.mu.Unlock()
	for key, sw := range rl.windows {
		sw.mu.Lock()
		// Trim entries older than 1 second (sliding window width)
		cutoff := now.Add(-1 * time.Second)
		keep := 0
		for _, t := range sw.ts {
			if t.After(cutoff) {
				break
			}
			keep++
		}
		if keep > 0 {
			sw.ts = sw.ts[keep:]
		}
		// Remove empty windows
		if len(sw.ts) == 0 {
			delete(rl.windows, key)
		}
		sw.mu.Unlock()
	}
}

func (rl *rateLimiter) allow(groupName string, ip string) bool {
	group, ok := rl.groups[groupName]
	if !ok {
		return true // unknown group, allow by default
	}

	key := groupName + ":" + ip
	now := time.Now()
	cutoff := now.Add(-1 * time.Second) // 1-second sliding window

	rl.mu.Lock()
	sw, exists := rl.windows[key]
	if !exists {
		sw = &slidingWindow{}
		rl.windows[key] = sw
	}
	rl.mu.Unlock()

	sw.mu.Lock()
	defer sw.mu.Unlock()

	// Trim entries outside the window
	keep := 0
	for _, t := range sw.ts {
		if t.After(cutoff) {
			break
		}
		keep++
	}
	if keep > 0 {
		sw.ts = sw.ts[keep:]
	}

	// Check rate
	if len(sw.ts) >= group.Rate {
		return false
	}

	sw.ts = append(sw.ts, now)
	return true
}

// RateLimit returns a Fiber middleware that limits requests per IP
// for a given rate group (public / auth / api).
func RateLimit(group RateGroup) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ip := c.IP()
		if !globalLimiter.allow(group.Name, ip) {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "rate limit exceeded",
				"group": group.Name,
			})
		}
		return c.Next()
	}
}
