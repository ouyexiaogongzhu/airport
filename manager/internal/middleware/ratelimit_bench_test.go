package middleware

import (
	"testing"
	"time"
)

// Benchmarks model a sustained 100 req/s flow (10ms spacing), the max allowed
// under the "user" group (rate=100). At that spacing one entry expires from
// the 1-second window per request, so every request is allowed.

func BenchmarkRateLimitAllow_RingBuffer(b *testing.B) {
	sw := newSlidingWindow(RateGroupUser.Rate)
	base := time.Now().Add(-time.Hour).UnixMilli()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !sw.allow(base + int64(i)*10) {
			b.Fatal("unexpected rate limit hit")
		}
	}
}

// BenchmarkRateLimitAllow_LegacySlice replicates the pre-optimisation
// slidingWindow: a []time.Time trimmed from the front on every request.
func BenchmarkRateLimitAllow_LegacySlice(b *testing.B) {
	ts := make([]time.Time, 0, RateGroupUser.Rate)
	base := time.Now().Add(-time.Hour)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		t := base.Add(time.Duration(i) * 10 * time.Millisecond)
		cutoff := t.Add(-time.Second)
		keep := 0
		for _, e := range ts {
			if e.After(cutoff) {
				break
			}
			keep++
		}
		if keep > 0 {
			ts = ts[keep:]
		}
		if len(ts) >= RateGroupUser.Rate {
			b.Fatal("unexpected rate limit hit")
		}
		ts = append(ts, t)
	}
}
