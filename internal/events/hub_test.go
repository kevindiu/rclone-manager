package events

import (
	"sync"
	"testing"
	"time"
)

func TestHubSubscribeBroadcastAndUnsubscribe(t *testing.T) {
	hub := NewHub()
	ch1, cleanup1 := hub.Subscribe()
	ch2, cleanup2 := hub.Subscribe()
	defer cleanup2()

	hub.Broadcast("progress", map[string]int{"pct": 50})

	for _, ch := range []chan Event{ch1, ch2} {
		select {
		case ev := <-ch:
			if ev.Type != "progress" {
				t.Fatalf("expected event type progress, got %q", ev.Type)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for broadcast")
		}
	}

	cleanup1()
	if _, open := <-ch1; open {
		t.Fatal("expected unsubscribed channel to be closed")
	}

	// Broadcasting after cleanup1 should still deliver to ch2 without panicking.
	hub.Broadcast("run_completed", "ok")
	select {
	case ev := <-ch2:
		if ev.Type != "run_completed" {
			t.Fatalf("expected run_completed, got %q", ev.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for second broadcast")
	}
}

func TestHubSlowSubscriberDoesNotBlockAndRaceSafe(t *testing.T) {
	hub := NewHub()
	_, cleanupSlow := hub.Subscribe()
	defer cleanupSlow()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch, unsub := hub.Subscribe()
			defer unsub()
			for j := 0; j < 100; j++ {
				hub.Broadcast("tick", j)
				select {
				case <-ch:
				default:
				}
			}
		}()
	}
	wg.Wait()
}

func TestHubSlowSubscriberEvictedAndHealthyContinues(t *testing.T) {
	hub := NewHub()
	slowCh, slowCleanup := hub.Subscribe()
	defer slowCleanup()

	fastCh, fastCleanup := hub.Subscribe()
	defer fastCleanup()

	// Fill slowCh buffer (64) without draining, then send 1 more to trigger eviction.
	for i := 0; i < 65; i++ {
		hub.Broadcast("progress", i)
		// Drain fastCh so it never fills up.
		select {
		case <-fastCh:
		default:
		}
	}

	// Drain remaining buffered messages in slowCh and verify channel was closed on eviction.
	drained := 0
	for range slowCh {
		drained++
	}
	if drained != 64 {
		t.Fatalf("expected 64 buffered events before eviction close, got %d", drained)
	}

	// Healthy subscriber should still receive subsequent broadcasts.
	hub.Broadcast("run_completed", "done")
	gotDone := false
	timeout := time.After(time.Second)
	for !gotDone {
		select {
		case ev, open := <-fastCh:
			if !open {
				t.Fatal("fast subscriber unexpectedly closed")
			}
			if ev.Type == "run_completed" {
				gotDone = true
			}
		case <-timeout:
			t.Fatal("timed out waiting for run_completed on fast subscriber")
		}
	}
}

func TestHubCloseDisconnectsSubscribersAndIdempotent(t *testing.T) {
	hub := NewHub()
	ch1, cleanup1 := hub.Subscribe()
	defer cleanup1()
	ch2, cleanup2 := hub.Subscribe()
	defer cleanup2()

	hub.Close()
	// Second Close and Broadcast after Close should be safe no-ops.
	hub.Close()
	hub.Broadcast("progress", 1)

	for i, ch := range []chan Event{ch1, ch2} {
		select {
		case _, open := <-ch:
			if open {
				t.Fatalf("subscriber %d channel still open after Close", i)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for subscriber %d channel to close", i)
		}
	}

	lateCh, lateCleanup := hub.Subscribe()
	defer lateCleanup()
	if _, open := <-lateCh; open {
		t.Fatal("expected Subscribe after Close to return a closed channel")
	}
}
