package auth

import (
	"sync"
	"time"
)

const (
	defaultMaxFailures = 10
	defaultWindow      = 5 * time.Minute
)

// LoginRateLimiter tracks recent failed login timestamps per client IP/key
// inside a sliding time window to mitigate brute-force guessing.
type LoginRateLimiter struct {
	mu          sync.Mutex
	maxFailures int
	window      time.Duration
	entries     map[string][]time.Time
	now         func() time.Time
}

// NewLoginRateLimiter constructs a sliding-window limiter with sane defaults when <= 0.
func NewLoginRateLimiter(maxFailures int, window time.Duration) *LoginRateLimiter {
	if maxFailures <= 0 {
		maxFailures = defaultMaxFailures
	}
	if window <= 0 {
		window = defaultWindow
	}
	return &LoginRateLimiter{
		maxFailures: maxFailures,
		window:      window,
		entries:     make(map[string][]time.Time),
		now:         func() time.Time { return time.Now().UTC() },
	}
}

// Allow checks whether key may attempt a login. When blocked, retryAfter indicates
// how long until the oldest failure in the current window expires.
func (l *LoginRateLimiter) Allow(key string) (allowed bool, retryAfter time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	ts := l.pruneKeyLocked(key, now)
	if len(ts) >= l.maxFailures {
		wait := l.window - now.Sub(ts[0])
		if wait < time.Second {
			wait = time.Second
		}
		return false, wait
	}
	return true, 0
}

// RecordFailure records a failed authentication attempt for key.
func (l *LoginRateLimiter) RecordFailure(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	ts := l.pruneKeyLocked(key, now)
	l.entries[key] = append(ts, now)

	// Opportunistically bound map growth under sustained distributed probes.
	if len(l.entries) > 1000 {
		for k := range l.entries {
			_ = l.pruneKeyLocked(k, now)
		}
	}
}

// Reset clears recorded failures for key (e.g. after a successful login).
func (l *LoginRateLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, key)
}

func (l *LoginRateLimiter) pruneKeyLocked(key string, now time.Time) []time.Time {
	ts := l.entries[key]
	if len(ts) == 0 {
		return nil
	}
	cutoff := now.Add(-l.window)
	idx := 0
	for idx < len(ts) && !ts[idx].After(cutoff) {
		idx++
	}
	if idx == len(ts) {
		delete(l.entries, key)
		return nil
	}
	if idx > 0 {
		ts = append([]time.Time(nil), ts[idx:]...)
		l.entries[key] = ts
	}
	return ts
}
