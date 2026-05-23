package ratelimit

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestAllow_RollingWindow(t *testing.T) {
	l := New()

	for i := 0; i < 5; i++ {
		if !l.Allow("u1") {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}

	if l.Allow("u1") {
		t.Fatal("6th request should be rejected")
	}
}

func TestAllow_IndependentUsers(t *testing.T) {
	l := New()

	for i := 0; i < 5; i++ {
		l.Allow("a")
	}

	// Different user should still be allowed
	if !l.Allow("b") {
		t.Fatal("different user should not be affected")
	}
}

// TestAllow_ConcurrentSafety fires 100 goroutines at the same user_id
// and verifies that exactly 5 get accepted — never more, never less.
func TestAllow_ConcurrentSafety(t *testing.T) {
	l := New()

	var accepted atomic.Int64
	var rejected atomic.Int64
	var wg sync.WaitGroup

	n := 100
	wg.Add(n)

	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			if l.Allow("contended") {
				accepted.Add(1)
			} else {
				rejected.Add(1)
			}
		}()
	}

	wg.Wait()

	if got := accepted.Load(); got != 5 {
		t.Errorf("expected exactly 5 accepted, got %d", got)
	}
	if got := rejected.Load(); got != 95 {
		t.Errorf("expected exactly 95 rejected, got %d", got)
	}
}

func TestStats_ReflectsCurrentWindow(t *testing.T) {
	l := New()

	l.Allow("x")
	l.Allow("x")
	l.Allow("x")

	stats := l.Stats()
	us, ok := stats.Users["x"]
	if !ok {
		t.Fatal("user x should appear in stats")
	}
	if us.Accepted != 3 {
		t.Errorf("expected 3 accepted, got %d", us.Accepted)
	}
	if us.Rejected != 0 {
		t.Errorf("expected 0 rejected, got %d", us.Rejected)
	}
}
