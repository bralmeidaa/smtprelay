package relay

import (
	"sync"
	"time"
)

// RateLimiter is a simple fixed-window counter per remote IP, kept entirely
// in memory. That's intentionally proportional to the scale of this project
// (~4 users, very low volume) rather than pulling in a token-bucket
// dependency for a problem this small.
type RateLimiter struct {
	mu sync.Mutex

	connWindow time.Duration
	connLimit  int
	conns      map[string][]time.Time

	msgWindow time.Duration
	msgLimit  int
	msgs      map[string][]time.Time
}

func NewRateLimiter(connPerMinute, msgsPerHour int) *RateLimiter {
	return &RateLimiter{
		connWindow: time.Minute,
		connLimit:  connPerMinute,
		conns:      make(map[string][]time.Time),
		msgWindow:  time.Hour,
		msgLimit:   msgsPerHour,
		msgs:       make(map[string][]time.Time),
	}
}

// AllowConn reports whether ip may open another connection right now.
func (r *RateLimiter) AllowConn(ip string) bool {
	return r.allow(r.conns, ip, r.connWindow, r.connLimit)
}

// AllowMessage reports whether ip may relay another message right now.
func (r *RateLimiter) AllowMessage(ip string) bool {
	return r.allow(r.msgs, ip, r.msgWindow, r.msgLimit)
}

func (r *RateLimiter) allow(bucket map[string][]time.Time, key string, window time.Duration, limit int) bool {
	if limit <= 0 {
		return true
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-window)

	kept := bucket[key][:0]
	for _, t := range bucket[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}

	if len(kept) >= limit {
		bucket[key] = kept
		return false
	}

	bucket[key] = append(kept, now)
	return true
}
