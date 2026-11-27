package runner

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestConcurrencyGateLimitCancelAndDynamicResize(t *testing.T) {
	g := newConcurrencyGate(1)
	ctx := context.Background()

	if err := g.acquire(ctx); err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}

	// Cancelled waiter returns context.Canceled promptly without leaking.
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if err := g.acquire(cancelCtx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	// Second waiter blocks until limit is raised dynamically.
	acquired := make(chan error, 1)
	go func() {
		acquired <- g.acquire(ctx)
	}()

	select {
	case err := <-acquired:
		t.Fatalf("second acquire should block while gate full, got err=%v", err)
	case <-time.After(40 * time.Millisecond):
	}

	// Increasing limit to 2 unblocks waiter without needing release().
	g.setLimit(2)
	select {
	case err := <-acquired:
		if err != nil {
			t.Fatalf("expected blocked waiter to succeed after setLimit(2), got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for dynamic resize to unblock waiter")
	}

	g.release()
	g.release()
}
