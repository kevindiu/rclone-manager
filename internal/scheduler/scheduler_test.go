package scheduler

import (
	"testing"
)

func TestPreviewAndValidateCron(t *testing.T) {
	if err := ValidateCron("0 2 * * *"); err != nil {
		t.Fatalf("expected valid cron: %v", err)
	}
	if err := ValidateCron("not a cron"); err == nil {
		t.Fatal("expected error for invalid cron expression")
	}

	times, err := PreviewCron("*/15 * * * *", "Asia/Tokyo", 5)
	if err != nil {
		t.Fatalf("PreviewCron failed: %v", err)
	}
	if len(times) != 5 {
		t.Fatalf("expected 5 times, got %d", len(times))
	}
	for i := 1; i < len(times); i++ {
		if !times[i].After(times[i-1]) {
			t.Fatalf("times not strictly increasing: %v", times)
		}
		if times[i].Sub(times[i-1]).Minutes() != 15 {
			t.Fatalf("expected 15m interval, got %v", times[i].Sub(times[i-1]))
		}
	}
}
