package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kevindiu/rclone-manager/internal/domain"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func defaultTestSettings() domain.GlobalSettings {
	return domain.GlobalSettings{
		Transfers:         4,
		Checkers:          8,
		Retries:           3,
		LowLevelRetries:   10,
		Timeout:           "5m",
		ConTimeout:        "1m",
		LogRetentionDays:  30,
		Timezone:          "UTC",
		MaxConcurrentJobs: 2,
	}
}

func TestMigrationsIdempotentAndVersioned(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "migrated.db")
	s1, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	var ver int
	if err := s1.db.QueryRow(`PRAGMA user_version`).Scan(&ver); err != nil {
		t.Fatal(err)
	}
	if ver != len(migrations) {
		t.Fatalf("expected user_version %d, got %d", len(migrations), ver)
	}
	_ = s1.Close()

	// Reopening should be a clean no-op
	s2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("reopen failed: %v", err)
	}
	defer s2.Close()
	if err := s2.db.QueryRow(`PRAGMA user_version`).Scan(&ver); err != nil {
		t.Fatal(err)
	}
	if ver != len(migrations) {
		t.Fatalf("expected user_version %d after reopen, got %d", len(migrations), ver)
	}
}

func TestSettingsSeedAndSave(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	defaults := defaultTestSettings()

	got, err := store.GetSettings(ctx, defaults)
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if got.Transfers != 4 || got.MaxConcurrentJobs != 2 {
		t.Fatalf("unexpected seeded settings: %+v", got)
	}

	got.Transfers = 12
	got.BwLimit = "08:00,5M 23:00,off"
	got.Timezone = "Asia/Tokyo"
	if err := store.SaveSettings(ctx, got); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	reloaded, err := store.GetSettings(ctx, defaults)
	if err != nil {
		t.Fatalf("GetSettings after save: %v", err)
	}
	if reloaded.Transfers != 12 || reloaded.BwLimit != "08:00,5M 23:00,off" || reloaded.Timezone != "Asia/Tokyo" {
		t.Fatalf("unexpected reloaded settings: %+v", reloaded)
	}
}

func TestJobsCRUDAndImport(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	maxDel := 50

	created, err := store.CreateJob(ctx, domain.Job{
		Name:                 "Photos Backup",
		Enabled:              true,
		Operation:            "sync",
		Source:               "/photos",
		Destination:          "b2:photos",
		Schedule:             "0 2 * * *",
		VersioningEnabled:    true,
		VersionRetentionDays: 30,
		AbortIfSourceEmpty:   true,
		MaxDelete:            &maxDel,
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected generated job ID")
	}

	got, err := store.GetJob(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if got.Name != "Photos Backup" || got.MaxDelete == nil || *got.MaxDelete != 50 {
		t.Fatalf("unexpected fetched job: %+v", got)
	}

	got.Name = "Photos Sync"
	updated, err := store.UpdateJob(ctx, created.ID, *got)
	if err != nil {
		t.Fatalf("UpdateJob: %v", err)
	}
	if updated.Name != "Photos Sync" {
		t.Fatalf("expected updated name, got %q", updated.Name)
	}

	// Import merge should upsert and keep existing
	bundle := domain.ExportBundle{
		Version:  1,
		Settings: defaultTestSettings(),
		Jobs: []domain.Job{
			{
				ID:          "imported-1",
				Name:        "Docs Copy",
				Enabled:     true,
				Operation:   "copy",
				Source:      "/docs",
				Destination: "gdrive:docs",
			},
		},
	}
	bundle.Settings.Transfers = 9
	if err := store.ImportBundle(ctx, bundle, "merge", defaultTestSettings()); err != nil {
		t.Fatalf("ImportBundle merge: %v", err)
	}
	jobs, err := store.ListJobs(ctx)
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs after merge, got %d", len(jobs))
	}

	// Import replace should remove previous jobs
	if err := store.ImportBundle(ctx, bundle, "replace", defaultTestSettings()); err != nil {
		t.Fatalf("ImportBundle replace: %v", err)
	}
	jobs, err = store.ListJobs(ctx)
	if err != nil {
		t.Fatalf("ListJobs after replace: %v", err)
	}
	if len(jobs) != 1 || jobs[0].ID != "imported-1" {
		t.Fatalf("expected 1 replaced job, got %+v", jobs)
	}

	if err := store.DeleteJob(ctx, "imported-1"); err != nil {
		t.Fatalf("DeleteJob: %v", err)
	}
	if _, err := store.GetJob(ctx, "imported-1"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestRunsLifecycleRecoveryAndPruning(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	tmpDir := t.TempDir()

	oldLog := filepath.Join(tmpDir, "old.log")
	if err := os.WriteFile(oldLog, []byte("old log"), 0o644); err != nil {
		t.Fatal(err)
	}
	newLog := filepath.Join(tmpDir, "new.log")
	if err := os.WriteFile(newLog, []byte("new log"), 0o644); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	oldRun := domain.JobRun{
		ID:        "run-old",
		JobID:     "job-1",
		JobName:   "Job 1",
		Trigger:   "schedule",
		Status:    "success",
		StartedAt: now.AddDate(0, 0, -45),
		LogFile:   oldLog,
	}
	runningRun := domain.JobRun{
		ID:        "run-running",
		JobID:     "job-1",
		JobName:   "Job 1",
		Trigger:   "manual",
		Status:    "running",
		StartedAt: now.Add(-1 * time.Minute),
		LogFile:   newLog,
	}
	if err := store.CreateRun(ctx, oldRun); err != nil {
		t.Fatalf("CreateRun old: %v", err)
	}
	if err := store.CreateRun(ctx, runningRun); err != nil {
		t.Fatalf("CreateRun running: %v", err)
	}

	recovered, err := store.MarkInterruptedRunsFailed(ctx)
	if err != nil {
		t.Fatalf("MarkInterruptedRunsFailed: %v", err)
	}
	if recovered != 1 {
		t.Fatalf("expected 1 recovered run, got %d", recovered)
	}
	gotRunning, err := store.GetRun(ctx, "run-running")
	if err != nil {
		t.Fatal(err)
	}
	if gotRunning.Status != "failed" {
		t.Fatalf("expected failed status after recovery, got %q", gotRunning.Status)
	}

	pruned, err := store.PruneOldRuns(ctx, 30)
	if err != nil {
		t.Fatalf("PruneOldRuns: %v", err)
	}
	if pruned != 1 {
		t.Fatalf("expected 1 pruned run, got %d", pruned)
	}
	if _, err := os.Stat(oldLog); !os.IsNotExist(err) {
		t.Fatalf("expected old log file to be deleted, stat err=%v", err)
	}
	if _, err := os.Stat(newLog); err != nil {
		t.Fatalf("expected recent log file to remain, stat err=%v", err)
	}

	runs, total, err := store.ListRuns(ctx, "job-1", "", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(runs) != 1 || runs[0].ID != "run-running" {
		t.Fatalf("unexpected remaining runs: total=%d runs=%+v", total, runs)
	}
}
