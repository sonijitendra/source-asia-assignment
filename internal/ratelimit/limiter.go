package ratelimit

import (
	"sync"
	"time"
)

const (
	windowSize  = time.Minute
	maxRequests = 5
)

// Limiter enforces a rolling-window rate limit per user.
// Each user gets maxRequests accepted calls within a sliding windowSize.
// Internally it stores raw timestamps so the window slides naturally.
type Limiter struct {
	mu       sync.Mutex
	windows  map[string][]time.Time // user_id → accepted request timestamps
	rejected map[string]int64       // user_id → cumulative rejected count
}

func New() *Limiter {
	return &Limiter{
		windows:  make(map[string][]time.Time),
		rejected: make(map[string]int64),
	}
}

// Allow atomically checks and records whether a request from userID is
// within the rate limit. The entire check-then-record runs under a single
// lock so concurrent callers for the same user can never exceed the limit.
func (l *Limiter) Allow(userID string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-windowSize)

	// In-place filter: drop timestamps outside the current window.
	// Safe because the write index never overtakes the read index.
	timestamps := l.windows[userID]
	n := 0
	for _, ts := range timestamps {
		if ts.After(cutoff) {
			timestamps[n] = ts
			n++
		}
	}
	timestamps = timestamps[:n]

	if n >= maxRequests {
		l.windows[userID] = timestamps
		l.rejected[userID]++
		return false
	}

	l.windows[userID] = append(timestamps, now)
	return true
}

// --- Stats ------------------------------------------------------------------

type UserStats struct {
	Accepted int   `json:"accepted"`
	Rejected int64 `json:"rejected"`
}

type StatsResponse struct {
	Users         map[string]UserStats `json:"users"`
	TotalAccepted int                  `json:"total_accepted"`
	TotalRejected int64                `json:"total_rejected"`
}

// Stats returns a point-in-time snapshot of per-user rate-limit counters.
// "Accepted" reflects only timestamps inside the current rolling window.
// It also does light cleanup of fully-expired entries so the map doesn't
// grow unbounded for one-off users.
func (l *Limiter) Stats() StatsResponse {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-windowSize)

	resp := StatsResponse{
		Users: make(map[string]UserStats),
	}

	for userID, timestamps := range l.windows {
		// compact while we're here
		n := 0
		for _, ts := range timestamps {
			if ts.After(cutoff) {
				timestamps[n] = ts
				n++
			}
		}
		l.windows[userID] = timestamps[:n]

		rejected := l.rejected[userID]

		if n == 0 && rejected == 0 {
			delete(l.windows, userID)
			delete(l.rejected, userID)
			continue
		}

		resp.Users[userID] = UserStats{
			Accepted: n,
			Rejected: rejected,
		}
		resp.TotalAccepted += n
		resp.TotalRejected += rejected
	}

	return resp
}
