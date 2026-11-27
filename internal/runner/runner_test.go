package runner

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/kevindiu/rclone-manager/internal/db"
	"github.com/kevindiu/rclone-manager/internal/domain"
	"github.com/kevindiu/rclone-manager/internal/events"
	"github.com/kevindiu/rclone-manager/internal/rclone"
)

func setupTestRunner(t *testing.T) (*Runner, *db.Store, string) {
	t.Helper()
	dir := t.TempDir()
	store, err := db.Open(filepath.Join(dir, "runner.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	confPath := filepath.Join(dir, "rclone.conf")
	logDir := filepath.Join(dir, "logs")
	client := rclone.NewClient("rclone", confPath)
	hub := events.NewHub()
	defaults := domain.GlobalSettings{
		Transfers:         2,
		Checkers:          4,
		Retries:           1,
		LowLevelRetries:   2,
		Timeout:           "1m",
		ConTimeout:        "30s",
		LogRetentionDays:  30,
		Timezone:          "UTC",
		MaxConcurrentJobs: 2,
	}
	r := NewRunner(store, client, hub, logDir, defaults, slog.Default())
	return r, store, dir
}

func waitForRunDone(t *testing.T, store *db.Store, runID string) domain.JobRun {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		run, err := store.GetRun(context.Background(), runID)
		if err == nil && (run.Status == "success" || run.Status == "failed" || run.Status == "cancelled") {
			return *run
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for run %s", runID)
	return domain.JobRun{}
}

func TestRunnerSyncVersioningAndPruning(t *testing.T) {
	ctx := context.Background()
	r, store, dir := setupTestRunner(t)
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "doc.txt"), []byte("version-1"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create an old expired version folder and a non-date folder to verify pruning safety.
	expiredDir := filepath.Join(dst, "_versions", "2020-01-01")
	keepNamedDir := filepath.Join(dst, "_versions", "manual-snapshot")
	if err := os.MkdirAll(expiredDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(keepNamedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(expiredDir, "stale.txt"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	job, err := store.CreateJob(ctx, domain.Job{
		Name:                 "Sync With Versions",
		Enabled:              true,
		Operation:            "sync",
		Source:               src,
		Destination:          dst,
		VersioningEnabled:    true,
		VersionRetentionDays: 7,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Initial run copies doc.txt and prunes 2020-01-01
	run1, err := r.StartJob(*job, "manual", false)
	if err != nil {
		t.Fatalf("StartJob 1: %v", err)
	}
	res1 := waitForRunDone(t, store, run1.ID)
	if res1.Status != "success" {
		t.Fatalf("run1 failed: %+v", res1)
	}
	if _, err := os.Stat(expiredDir); !os.IsNotExist(err) {
		t.Fatalf("expected expired version dir to be pruned, err=%v", err)
	}
	if _, err := os.Stat(keepNamedDir); err != nil {
		t.Fatalf("expected non-dated dir to remain, err=%v", err)
	}

	// Modify doc.txt and re-sync -> old version should move into today's _versions/YYYY-MM-DD/
	if err := os.WriteFile(filepath.Join(src, "doc.txt"), []byte("version-2-updated"), 0o644); err != nil {
		t.Fatal(err)
	}
	run2, err := r.StartJob(*job, "manual", false)
	if err != nil {
		t.Fatalf("StartJob 2: %v", err)
	}
	res2 := waitForRunDone(t, store, run2.ID)
	if res2.Status != "success" {
		t.Fatalf("run2 failed: %+v", res2)
	}

	todayVersionFile := filepath.Join(dst, "_versions", time.Now().UTC().Format("2006-01-02"), "doc.txt")
	b, err := os.ReadFile(todayVersionFile)
	if err != nil {
		t.Fatalf("expected previous version in %s: %v", todayVersionFile, err)
	}
	if string(b) != "version-1" {
		t.Fatalf("unexpected backed-up file contents: %q", string(b))
	}
}

func TestRunnerPreflightGuards(t *testing.T) {
	ctx := context.Background()
	r, store, dir := setupTestRunner(t)
	emptySrc := filepath.Join(dir, "empty-src")
	dst := filepath.Join(dir, "dst")
	_ = os.MkdirAll(emptySrc, 0o755)
	_ = os.MkdirAll(dst, 0o755)

	jobEmpty, err := store.CreateJob(ctx, domain.Job{
		Name:               "Guard Empty",
		Enabled:            true,
		Operation:          "sync",
		Source:             emptySrc,
		Destination:        dst,
		AbortIfSourceEmpty: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	runEmpty, err := r.StartJob(*jobEmpty, "manual", false)
	if err != nil {
		t.Fatal(err)
	}
	resEmpty := waitForRunDone(t, store, runEmpty.ID)
	if resEmpty.Status != "failed" {
		t.Fatalf("expected failed status for empty source guard, got %+v", resEmpty)
	}

	// Non-empty source missing marker file
	srcWithFile := filepath.Join(dir, "src-marker")
	_ = os.MkdirAll(srcWithFile, 0o755)
	_ = os.WriteFile(filepath.Join(srcWithFile, "data.txt"), []byte("hello"), 0o644)

	jobMarker, err := store.CreateJob(ctx, domain.Job{
		Name:               "Guard Marker",
		Enabled:            true,
		Operation:          "copy",
		Source:             srcWithFile,
		Destination:        dst,
		RequiredMarkerFile: ".backup-ready",
	})
	if err != nil {
		t.Fatal(err)
	}
	runNoMarker, err := r.StartJob(*jobMarker, "manual", false)
	if err != nil {
		t.Fatal(err)
	}
	resNoMarker := waitForRunDone(t, store, runNoMarker.ID)
	if resNoMarker.Status != "failed" {
		t.Fatalf("expected failed status when marker missing, got %+v", resNoMarker)
	}

	// Add marker file -> should succeed
	_ = os.WriteFile(filepath.Join(srcWithFile, ".backup-ready"), []byte("ok"), 0o644)
	runWithMarker, err := r.StartJob(*jobMarker, "manual", false)
	if err != nil {
		t.Fatal(err)
	}
	resWithMarker := waitForRunDone(t, store, runWithMarker.ID)
	if resWithMarker.Status != "success" {
		t.Fatalf("expected success when marker present, got %+v", resWithMarker)
	}
}

func TestRunnerDryRunAndOverlapProtection(t *testing.T) {
	ctx := context.Background()
	r, store, dir := setupTestRunner(t)
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	_ = os.MkdirAll(src, 0o755)
	_ = os.MkdirAll(dst, 0o755)
	_ = os.WriteFile(filepath.Join(src, "dry.txt"), []byte("dry"), 0o644)

	job, err := store.CreateJob(ctx, domain.Job{
		Name:        "Dry Run Test",
		Enabled:     true,
		Operation:   "copy",
		Source:      src,
		Destination: dst,
	})
	if err != nil {
		t.Fatal(err)
	}

	runDry, err := r.StartJob(*job, "dry_run", true)
	if err != nil {
		t.Fatal(err)
	}
	resDry := waitForRunDone(t, store, runDry.ID)
	if resDry.Status != "success" || !resDry.DryRun {
		t.Fatalf("unexpected dry run result: %+v", resDry)
	}
	if _, err := os.Stat(filepath.Join(dst, "dry.txt")); !os.IsNotExist(err) {
		t.Fatalf("dry run should not copy files to destination")
	}

	// Simulate an active run to verify scheduled overlap skip
	r.mu.Lock()
	r.active[job.ID] = &activeRun{
		snapshot: domain.ActiveRunSnapshot{
			RunID:   "synthetic",
			JobID:   job.ID,
			JobName: job.Name,
			Status:  "running",
		},
	}
	r.mu.Unlock()

	if _, err := r.StartJob(*job, "schedule", false); err != ErrJobAlreadyRunning {
		t.Fatalf("expected ErrJobAlreadyRunning, got %v", err)
	}
	r.mu.Lock()
	delete(r.active, job.ID)
	r.mu.Unlock()

	runs, _, err := store.ListRuns(ctx, job.ID, "skipped", "", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 skipped run recorded, got %d", len(runs))
	}
}

func TestRunnerStopQueuedJobCancelsCleanly(t *testing.T) {
	ctx := context.Background()
	r, store, dir := setupTestRunner(t)
	src := filepath.Join(dir, "qsrc")
	dst := filepath.Join(dir, "qdst")
	_ = os.MkdirAll(src, 0o755)
	_ = os.MkdirAll(dst, 0o755)
	_ = os.WriteFile(filepath.Join(src, "q.txt"), []byte("queued"), 0o644)

	job, err := store.CreateJob(ctx, domain.Job{
		Name:        "Queued Cancel Test",
		Enabled:     true,
		Operation:   "copy",
		Source:      src,
		Destination: dst,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Fill concurrency gate so the job stays queued until StopJob cancels it.
	r.UpdateMaxConcurrent(1)
	if err := r.gate.acquire(ctx); err != nil {
		t.Fatal(err)
	}
	gateReleased := false
	defer func() {
		if !gateReleased {
			r.gate.release()
		}
	}()

	run, err := r.StartJob(*job, "manual", false)
	if err != nil {
		t.Fatalf("StartJob failed: %v", err)
	}
	if _, active := r.ActiveRunByJob(job.ID); !active {
		t.Fatal("expected queued job to be active before StopJob")
	}

	if err := r.StopJob(job.ID); err != nil {
		t.Fatalf("StopJob failed: %v", err)
	}
	r.gate.release()
	gateReleased = true

	res := waitForRunDone(t, store, run.ID)
	if res.Status != "cancelled" {
		t.Fatalf("expected cancelled status, got %+v", res)
	}
	r.Wait()
	if _, active := r.ActiveRunByJob(job.ID); active {
		t.Fatal("expected job removed from active map after completion")
	}
}

type mockNotifier struct {
	mu    sync.Mutex
	calls []domain.JobRun
	done  chan struct{}
}

func (m *mockNotifier) NotifyRun(_ context.Context, _ domain.GlobalSettings, run domain.JobRun) {
	m.mu.Lock()
	m.calls = append(m.calls, run)
	m.mu.Unlock()
	if m.done != nil {
		select {
		case m.done <- struct{}{}:
		default:
		}
	}
}

func TestRunnerTelegramNotificationAndSkippedAndShutdown(t *testing.T) {
	ctx := context.Background()
	r, store, dir := setupTestRunner(t)

	if err := store.SaveSettings(ctx, domain.GlobalSettings{
		Transfers:         2,
		Checkers:          4,
		Timezone:          "UTC",
		MaxConcurrentJobs: 2,
		WebhookType:       "telegram",
		WebhookOn:         "always",
		TelegramBotToken:  "123456:BOTSECRET",
		TelegramChatID:    "-100123",
	}); err != nil {
		t.Fatal(err)
	}

	notif := &mockNotifier{done: make(chan struct{}, 4)}
	r.SetNotifier(notif)

	src := filepath.Join(dir, "tgsrc")
	dst := filepath.Join(dir, "tgdst")
	_ = os.MkdirAll(src, 0o755)
	_ = os.MkdirAll(dst, 0o755)
	_ = os.WriteFile(filepath.Join(src, "f.txt"), []byte("hi"), 0o644)

	job, err := store.CreateJob(ctx, domain.Job{
		Name:        "Telegram Job",
		Enabled:     true,
		Operation:   "copy",
		Source:      src,
		Destination: dst,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Simulate an active run to trigger a skipped scheduled run notification.
	r.mu.Lock()
	r.active[job.ID] = &activeRun{snapshot: domain.ActiveRunSnapshot{RunID: "busy", JobID: job.ID}}
	r.mu.Unlock()
	if _, err := r.StartJob(*job, "schedule", false); !errors.Is(err, ErrJobAlreadyRunning) {
		t.Fatalf("expected ErrJobAlreadyRunning, got %v", err)
	}
	r.mu.Lock()
	delete(r.active, job.ID)
	r.mu.Unlock()

	select {
	case <-notif.done:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for skipped run notification")
	}

	// Now run normally and verify Telegram notification fires even with empty WebhookURL.
	run, err := r.StartJob(*job, "manual", false)
	if err != nil {
		t.Fatalf("StartJob failed: %v", err)
	}
	res := waitForRunDone(t, store, run.ID)
	if res.Status != "success" {
		t.Fatalf("expected success, got %+v", res)
	}

	select {
	case <-notif.done:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for telegram notification on finished run")
	}

	r.Shutdown()
	notif.mu.Lock()
	count := len(notif.calls)
	notif.mu.Unlock()
	if count != 2 {
		t.Fatalf("expected 2 notifications (skipped + success), got %d", count)
	}

	if _, err := r.StartJob(*job, "manual", false); !errors.Is(err, ErrShuttingDown) {
		t.Fatalf("expected ErrShuttingDown after Shutdown, got %v", err)
	}
}
