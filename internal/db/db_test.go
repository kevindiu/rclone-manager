package db

import (
	"context"
	"errors"
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
	defer func() { _ = s2.Close() }()
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

	runs, total, err := store.ListRuns(ctx, "job-1", "", "", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(runs) != 1 || runs[0].ID != "run-running" {
		t.Fatalf("unexpected remaining runs: total=%d runs=%+v", total, runs)
	}
}

func TestRunsSubsecondOrderingAndTriggerFilter(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	base := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	run0 := domain.JobRun{
		ID:        "run-000",
		JobID:     "job-sub",
		JobName:   "Subsecond Job",
		Trigger:   "schedule",
		Status:    "success",
		StartedAt: base,
	}
	run1 := domain.JobRun{
		ID:        "run-100",
		JobID:     "job-sub",
		JobName:   "Subsecond Job",
		Trigger:   "manual",
		Status:    "success",
		StartedAt: base.Add(100 * time.Millisecond),
	}
	run9 := domain.JobRun{
		ID:        "run-900",
		JobID:     "job-sub",
		JobName:   "Subsecond Job",
		Trigger:   "restore",
		Status:    "success",
		StartedAt: base.Add(900 * time.Millisecond),
	}
	for _, r := range []domain.JobRun{run0, run1, run9} {
		if err := store.CreateRun(ctx, r); err != nil {
			t.Fatalf("CreateRun %s: %v", r.ID, err)
		}
	}

	runs, total, err := store.ListRuns(ctx, "job-sub", "", "", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || len(runs) != 3 {
		t.Fatalf("expected 3 runs, got total=%d len=%d", total, len(runs))
	}
	if runs[0].ID != "run-900" || runs[1].ID != "run-100" || runs[2].ID != "run-000" {
		t.Fatalf("unexpected chronological DESC order: %s, %s, %s", runs[0].ID, runs[1].ID, runs[2].ID)
	}

	lastMap, err := store.GetLastRunsByJobIDs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if lastMap["job-sub"].ID != "run-900" {
		t.Fatalf("expected latest run-900 from GetLastRunsByJobIDs, got %s", lastMap["job-sub"].ID)
	}

	restoreRuns, restoreTotal, err := store.ListRuns(ctx, "", "", "restore", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if restoreTotal != 1 || len(restoreRuns) != 1 || restoreRuns[0].ID != "run-900" {
		t.Fatalf("expected 1 restore-triggered run, got total=%d runs=%+v", restoreTotal, restoreRuns)
	}
}

func TestUsersAndSessionsLifecycle(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	count, err := store.CountUsers(ctx)
	if err != nil {
		t.Fatalf("CountUsers: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 users initially, got %d", count)
	}

	user, err := store.CreateInitialUser(ctx, "Admin", "hash-v1")
	if err != nil {
		t.Fatalf("CreateInitialUser: %v", err)
	}
	if user.ID == "" || user.Username != "Admin" {
		t.Fatalf("unexpected user: %+v", user)
	}

	// Second initial user attempt must fail atomically with ErrAlreadyInitialized.
	if _, err := store.CreateInitialUser(ctx, "other", "hash-v2"); !errors.Is(err, ErrAlreadyInitialized) {
		t.Fatalf("expected ErrAlreadyInitialized, got %v", err)
	}

	// Case-insensitive lookup.
	byName, err := store.GetUserByUsername(ctx, "admin")
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}
	if byName.ID != user.ID {
		t.Fatalf("expected ID %q, got %q", user.ID, byName.ID)
	}

	// Update password.
	if err := store.UpdateUserPassword(ctx, user.ID, "hash-v2"); err != nil {
		t.Fatalf("UpdateUserPassword: %v", err)
	}
	byID, err := store.GetUserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if byID.PasswordHash != "hash-v2" {
		t.Fatalf("expected hash-v2, got %q", byID.PasswordHash)
	}

	now := time.Now().UTC()
	if err := store.CreateSession(ctx, domain.Session{
		TokenHash: "tok-active-1",
		UserID:    user.ID,
		ExpiresAt: now.Add(time.Hour),
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("CreateSession active-1: %v", err)
	}
	if err := store.CreateSession(ctx, domain.Session{
		TokenHash: "tok-active-2",
		UserID:    user.ID,
		ExpiresAt: now.Add(time.Hour),
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("CreateSession active-2: %v", err)
	}
	if err := store.CreateSession(ctx, domain.Session{
		TokenHash: "tok-expired",
		UserID:    user.ID,
		ExpiresAt: now.Add(-time.Minute),
		CreatedAt: now.Add(-2 * time.Hour),
	}); err != nil {
		t.Fatalf("CreateSession expired: %v", err)
	}

	sess, sessUser, err := store.GetSessionWithUser(ctx, "tok-active-1")
	if err != nil {
		t.Fatalf("GetSessionWithUser: %v", err)
	}
	if sess.TokenHash != "tok-active-1" || sessUser.ID != user.ID {
		t.Fatalf("unexpected session/user: %+v %+v", sess, sessUser)
	}

	// Prune expired keeps valid sessions.
	pruned, err := store.PruneExpiredSessions(ctx)
	if err != nil {
		t.Fatalf("PruneExpiredSessions: %v", err)
	}
	if pruned != 1 {
		t.Fatalf("expected 1 pruned session, got %d", pruned)
	}
	if _, _, err := store.GetSessionWithUser(ctx, "tok-expired"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected expired session deleted, got %v", err)
	}

	// Revoke other sessions keeping tok-active-1.
	if err := store.DeleteUserSessionsExcept(ctx, user.ID, "tok-active-1"); err != nil {
		t.Fatalf("DeleteUserSessionsExcept: %v", err)
	}
	if _, _, err := store.GetSessionWithUser(ctx, "tok-active-2"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected tok-active-2 deleted, got %v", err)
	}
	if _, _, err := store.GetSessionWithUser(ctx, "tok-active-1"); err != nil {
		t.Fatalf("expected tok-active-1 preserved, got %v", err)
	}

	// List active user sessions excludes expired (tok-active-1 + tok-list-1 remain).
	_ = store.CreateSession(ctx, domain.Session{
		TokenHash: "tok-list-1",
		UserID:    user.ID,
		ExpiresAt: now.Add(time.Hour),
		CreatedAt: now,
	})
	_ = store.CreateSession(ctx, domain.Session{
		TokenHash: "tok-list-expired",
		UserID:    user.ID,
		ExpiresAt: now.Add(-time.Minute),
		CreatedAt: now.Add(-2 * time.Hour),
	})
	activeSessions, err := store.ListUserSessions(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListUserSessions: %v", err)
	}
	if len(activeSessions) != 2 {
		t.Fatalf("expected 2 active sessions (excluding expired), got %d: %+v", len(activeSessions), activeSessions)
	}
	for _, s := range activeSessions {
		if s.TokenHash == "tok-list-expired" {
			t.Fatalf("expected expired session excluded, got %+v", activeSessions)
		}
	}

	if err := store.DeleteSession(ctx, "tok-active-1"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if _, _, err := store.GetSessionWithUser(ctx, "tok-active-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after DeleteSession, got %v", err)
	}
}

func TestDailyRunStatsAndDeleteFinishedRuns(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	tmpDir := t.TempDir()

	doneLog := filepath.Join(tmpDir, "done.log")
	if err := os.WriteFile(doneLog, []byte("done"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for _, r := range []domain.JobRun{
		{ID: "s1", JobID: "j1", JobName: "J1", Trigger: "manual", Status: "success", StartedAt: now, BytesTransferred: 1000, LogFile: doneLog},
		{ID: "f1", JobID: "j1", JobName: "J1", Trigger: "schedule", Status: "failed", StartedAt: now, BytesTransferred: 500},
		{ID: "a1", JobID: "j1", JobName: "J1", Trigger: "manual", Status: "running", StartedAt: now, BytesTransferred: 9999},
	} {
		if err := store.CreateRun(ctx, r); err != nil {
			t.Fatalf("CreateRun %s: %v", r.ID, err)
		}
	}

	stats, err := store.GetDailyRunStats(ctx, 14)
	if err != nil {
		t.Fatalf("GetDailyRunStats: %v", err)
	}
	if len(stats) != 14 {
		t.Fatalf("expected 14 dense stat rows, got %d", len(stats))
	}
	today := stats[len(stats)-1]
	if today.SuccessCount != 1 || today.FailedCount != 1 || today.BytesTransferred != 1500 {
		t.Fatalf("unexpected today stats (should exclude running): %+v", today)
	}

	deleted, err := store.DeleteFinishedRuns(ctx, "j1", "success", "")
	if err != nil {
		t.Fatalf("DeleteFinishedRuns: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 deleted run, got %d", deleted)
	}
	if _, err := os.Stat(doneLog); !os.IsNotExist(err) {
		t.Fatalf("expected done log file removed, stat err=%v", err)
	}
	// Running run must still exist.
	if _, err := store.GetRun(ctx, "a1"); err != nil {
		t.Fatalf("expected running run preserved, got %v", err)
	}
}

func TestJobsPreserveAllOperations(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	ops := []string{"copy", "sync", "move", "check", "bisync"}
	for _, op := range ops {
		created, err := store.CreateJob(ctx, domain.Job{
			Name:        "Op-" + op,
			Operation:   op,
			Source:      "/src",
			Destination: "r2:dst",
		})
		if err != nil {
			t.Fatalf("CreateJob(%s): %v", op, err)
		}
		if created.Operation != op {
			t.Fatalf("CreateJob(%s) mutated operation to %q", op, created.Operation)
		}

		got, err := store.GetJob(ctx, created.ID)
		if err != nil {
			t.Fatalf("GetJob(%s): %v", op, err)
		}
		if got.Operation != op {
			t.Fatalf("GetJob(%s) returned operation %q", op, got.Operation)
		}

		got.Description = "updated"
		updated, err := store.UpdateJob(ctx, got.ID, *got)
		if err != nil {
			t.Fatalf("UpdateJob(%s): %v", op, err)
		}
		if updated.Operation != op {
			t.Fatalf("UpdateJob(%s) mutated operation to %q", op, updated.Operation)
		}
	}

	bundle := domain.ExportBundle{
		Version:  1,
		Settings: defaultTestSettings(),
		Jobs: []domain.Job{
			{ID: "imp-move", Name: "ImpMove", Operation: "move", Source: "/a", Destination: "/b"},
			{ID: "imp-check", Name: "ImpCheck", Operation: "check", Source: "/a", Destination: "/b"},
			{ID: "imp-bisync", Name: "ImpBisync", Operation: "bisync", Source: "/a", Destination: "/b"},
		},
	}
	if err := store.ImportBundle(ctx, bundle, "merge", defaultTestSettings()); err != nil {
		t.Fatalf("ImportBundle: %v", err)
	}
	for _, tc := range bundle.Jobs {
		got, err := store.GetJob(ctx, tc.ID)
		if err != nil {
			t.Fatalf("GetJob(%s) after import: %v", tc.ID, err)
		}
		if got.Operation != tc.Operation {
			t.Fatalf("ImportBundle(%s) mutated operation to %q", tc.Operation, got.Operation)
		}
	}
}
