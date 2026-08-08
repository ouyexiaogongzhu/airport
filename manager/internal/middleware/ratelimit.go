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
	globalLimiter.windows.Range(func(key, value any) bool {
		globalLimiter.windows.Delete(key)
		return true
	})
}

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
	RateGroupUser   = RateGroup{Name: "user", Rate: 100}
)

// slidingWindow stores request timestamps for a single IP within a 1-second
// sliding window using a fixed-capacity ring buffer (cap = group rate, at most
// 100). Expired entries are skipped from the head on each call, so trimming is
// O(k) only for k expired entries (k ≤ rate, bounded) and inserts are O(1) --
// the previous slice implementation re-sliced the whole front on every request.
type slidingWindow struct {
	mu    sync.Mutex
	ts    []int64 // unix-millisecond timestamps, ring buffer
	head  int     // index of the oldest valid entry
	count int     // number of valid entries
}

func newSlidingWindow(cap int) *slidingWindow {
	if cap < 1 {
		cap = 1
	}
	return &slidingWindow{ts: make([]int64, cap)}
}

// allow records nowMs if the window has room, first skipping entries that have
// fallen out of the 1-second window. Callers must hold sw.mu.
func (sw *slidingWindow) allow(nowMs int64) bool {
	for sw.count > 0 && nowMs-sw.ts[sw.head] >= 1000 {
		sw.head = (sw.head + 1) % len(sw.ts)
		sw.count--
	}
	if sw.count >= len(sw.ts) {
		return false
	}
	sw.ts[(sw.head+sw.count)%len(sw.ts)] = nowMs
	sw.count++
	return true
}

// empty reports whether the window holds no valid entries. Callers must hold
// sw.mu.
func (sw *slidingWindow) empty() bool {
	return sw.count == 0
}

type rateLimiter struct {
	windows sync.Map // key: "group:ip" -> *slidingWindow
	groups  map[string]RateGroup
	stopCh  chan struct{}
}

var globalLimiter *rateLimiter

func init() {
	globalLimiter = &rateLimiter{
		groups: map[string]RateGroup{
			RateGroupPublic.Name: RateGroupPublic,
			RateGroupAuth.Name:   RateGroupAuth,
			RateGroupAPI.Name:    RateGroupAPI,
			RateGroupUser.Name:   RateGroupUser,
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

// prune drops windows with no entries valid within the 1-second window. The
// sync.Map lookup is single-lock and per-window locks keep different IPs
// independent.
func (rl *rateLimiter) prune() {
	nowMs := time.Now().UnixMilli()
	rl.windows.Range(func(key, value any) bool {
		sw := value.(*slidingWindow)
		sw.mu.Lock()
		for sw.count > 0 && nowMs-sw.ts[sw.head] >= 1000 {
			sw.head = (sw.head + 1) % len(sw.ts)
			sw.count--
		}
		if sw.count == 0 {
			rl.windows.Delete(key)
		}
		sw.mu.Unlock()
		return true
	})
}

func (rl *rateLimiter) allow(groupName string, ip string) bool {
	group, ok := rl.groups[groupName]
	if !ok {
		return true // unknown group, allow by default
	}

	key := groupName + ":" + ip
	nowMs := time.Now().UnixMilli()

	swAny, _ := rl.windows.LoadOrStore(key, newSlidingWindow(group.Rate))
	sw := swAny.(*slidingWindow)

	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.allow(nowMs)
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
