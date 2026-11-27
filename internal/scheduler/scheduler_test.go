package scheduler

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/kevindiu/rclone-manager/internal/domain"
)

type mockStore struct {
	settings domain.GlobalSettings
	jobs     []domain.Job
}

func (m *mockStore) GetSettings(_ context.Context, _ domain.GlobalSettings) (domain.GlobalSettings, error) {
	return m.settings, nil
}
func (m *mockStore) ListJobs(_ context.Context) ([]domain.Job, error) {
	return m.jobs, nil
}
func (m *mockStore) GetJob(_ context.Context, id string) (*domain.Job, error) {
	for i := range m.jobs {
		if m.jobs[i].ID == id {
			j := m.jobs[i]
			return &j, nil
		}
	}
	return nil, context.Canceled
}
func (m *mockStore) PruneOldRuns(_ context.Context, _ int) (int64, error)      { return 0, nil }
func (m *mockStore) PruneExpiredSessions(_ context.Context) (int64, error) { return 0, nil }

type mockStarter struct{}

func (m *mockStarter) StartJob(job domain.Job, trigger string, dryRun bool) (*domain.JobRun, error) {
	return &domain.JobRun{ID: "run-1", JobID: job.ID, Trigger: trigger, DryRun: dryRun}, nil
}

func TestPreviewAndValidateCron(t *testing.T) {
	if err := ValidateCron("0 2 * * *"); err != nil {
		t.Fatalf("expected valid cron: %v", err)
	}
	if err := ValidateCron(""); err != nil {
		t.Fatalf("expected empty cron to be valid (manual job): %v", err)
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

func TestSchedulerStartReloadInvalidCronAndTimezoneFallback(t *testing.T) {
	st := &mockStore{
		settings: domain.GlobalSettings{Timezone: "Mars/Phobos"},
		jobs: []domain.Job{
			{ID: "j1", Name: "Valid Enabled", Enabled: true, Schedule: "0 2 * * *"},
			{ID: "j2", Name: "Valid Disabled", Enabled: false, Schedule: "0 3 * * *"},
			{ID: "j3", Name: "Bad Cron", Enabled: true, Schedule: "not valid cron"},
		},
	}
	sched := New(st, &mockStarter{}, domain.GlobalSettings{Timezone: "UTC"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := sched.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if sched.loc != time.UTC {
		t.Fatalf("expected UTC fallback for invalid timezone, got %v", sched.loc)
	}

	// Only 1 valid enabled job + 1 daily maintenance entry = 2 cron entries.
	if got := len(sched.cron.Entries()); got != 2 {
		t.Fatalf("expected 2 scheduled entries (1 job + 1 maintenance), got %d", got)
	}

	// Reload with valid timezone swaps location and keeps running cleanly.
	st.settings.Timezone = "Asia/Tokyo"
	st.jobs = append(st.jobs, domain.Job{ID: "j4", Name: "Second Enabled", Enabled: true, Schedule: "30 4 * * *"})
	if err := sched.Reload(); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}
	if sched.loc.String() != "Asia/Tokyo" {
		t.Fatalf("expected Asia/Tokyo location after reload, got %v", sched.loc)
	}
	if got := len(sched.cron.Entries()); got != 3 {
		t.Fatalf("expected 3 scheduled entries after reload, got %d", got)
	}

	sched.Stop()
}

func TestNextRunHelper(t *testing.T) {
	if got := NextRun("", "UTC"); got != nil {
		t.Fatalf("expected nil for empty schedule, got %v", got)
	}
	if got := NextRun("invalid", "UTC"); got != nil {
		t.Fatalf("expected nil for invalid cron, got %v", got)
	}
	got := NextRun("0 * * * *", "UTC")
	if got == nil || !got.After(time.Now().Add(-time.Second)) {
		t.Fatalf("expected valid future next run, got %v", got)
	}
}
