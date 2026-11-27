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
