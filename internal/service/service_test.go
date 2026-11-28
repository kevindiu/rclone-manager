package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kevindiu/rclone-manager/internal/db"
	"github.com/kevindiu/rclone-manager/internal/domain"
	"github.com/kevindiu/rclone-manager/internal/rclone"
	"github.com/kevindiu/rclone-manager/internal/runner"
)

type mockStore struct {
	settings  domain.GlobalSettings
	jobs      map[string]domain.Job
	runs      map[string]domain.JobRun
	importErr error
}

func newMockStore() *mockStore {
	return &mockStore{
		jobs: make(map[string]domain.Job),
		runs: make(map[string]domain.JobRun),
	}
}

func (m *mockStore) GetSettings(_ context.Context, defaults domain.GlobalSettings) (domain.GlobalSettings, error) {
	if m.settings.Timezone == "" {
		return defaults, nil
	}
	return m.settings, nil
}
func (m *mockStore) SaveSettings(_ context.Context, settings domain.GlobalSettings) error {
	m.settings = settings
	return nil
}
func (m *mockStore) ListJobs(_ context.Context) ([]domain.Job, error) {
	out := make([]domain.Job, 0, len(m.jobs))
	for _, j := range m.jobs {
		out = append(out, j)
	}
	return out, nil
}
func (m *mockStore) GetJob(_ context.Context, id string) (*domain.Job, error) {
	j, ok := m.jobs[id]
	if !ok {
		return nil, db.ErrNotFound
	}
	return &j, nil
}
func (m *mockStore) CreateJob(_ context.Context, job domain.Job) (*domain.Job, error) {
	if job.ID == "" {
		job.ID = "job-1"
	}
	m.jobs[job.ID] = job
	return &job, nil
}
func (m *mockStore) UpdateJob(_ context.Context, id string, job domain.Job) (*domain.Job, error) {
	if _, ok := m.jobs[id]; !ok {
		return nil, db.ErrNotFound
	}
	job.ID = id
	m.jobs[id] = job
	return &job, nil
}
func (m *mockStore) DeleteJob(_ context.Context, id string) error {
	if _, ok := m.jobs[id]; !ok {
		return db.ErrNotFound
	}
	delete(m.jobs, id)
	return nil
}
func (m *mockStore) ImportBundle(_ context.Context, bundle domain.ExportBundle, _ string, _ domain.GlobalSettings) error {
	if m.importErr != nil {
		return m.importErr
	}
	m.settings = bundle.Settings
	for _, j := range bundle.Jobs {
		m.jobs[j.ID] = j
	}
	return nil
}
func (m *mockStore) ListRuns(_ context.Context, _, _, _ string, _, _ int) ([]domain.JobRun, int, error) {
	return nil, 0, nil
}
func (m *mockStore) GetRun(_ context.Context, id string) (*domain.JobRun, error) {
	r, ok := m.runs[id]
	if !ok {
		return nil, db.ErrNotFound
	}
	return &r, nil
}
func (m *mockStore) GetLastRunsByJobIDs(_ context.Context) (map[string]domain.JobRun, error) {
	return nil, nil
}
func (m *mockStore) DeleteRun(_ context.Context, id string) error {
	delete(m.runs, id)
	return nil
}
func (m *mockStore) DeleteFinishedRuns(_ context.Context, jobID, status, trigger string) (int64, error) {
	var deleted int64
	for id, r := range m.runs {
		if r.Status == "running" || r.Status == "queued" {
			continue
		}
		if (jobID == "" || r.JobID == jobID) && (status == "" || r.Status == status) && (trigger == "" || r.Trigger == trigger) {
			delete(m.runs, id)
			deleted++
		}
	}
	return deleted, nil
}
func (m *mockStore) GetDailyRunStats(_ context.Context, days int) ([]domain.DailyRunStat, error) {
	return []domain.DailyRunStat{{Date: "2026-11-27", SuccessCount: 1, BytesTransferred: 1024}}, nil
}

type mockRunner struct {
	maxConcurrent int
	runningJobID  string
	lastJob       domain.Job
	lastTrigger   string
	lastDryRun    bool
}

func (m *mockRunner) UpdateMaxConcurrent(limit int) { m.maxConcurrent = limit }
func (m *mockRunner) ActiveRunByJob(jobID string) (string, bool) {
	if m.runningJobID == jobID {
		return "run-active", true
	}
	return "", false
}
func (m *mockRunner) StartJob(job domain.Job, trigger string, dryRun bool) (*domain.JobRun, error) {
	m.lastJob = job
	m.lastTrigger = trigger
	m.lastDryRun = dryRun
	return &domain.JobRun{ID: "run-1", JobID: job.ID, Trigger: trigger, DryRun: dryRun}, nil
}
func (m *mockRunner) StopJob(string) error { return nil }
func (m *mockRunner) DryRunDraft(_ context.Context, _ domain.Job, _ domain.GlobalSettings) (runner.DraftDryRunResult, error) {
	return runner.DraftDryRunResult{OK: true, Summary: "Dry run succeeded"}, nil
}

type mockScheduler struct{ reloads int }

func (m *mockScheduler) Reload() error {
	m.reloads++
	return nil
}

type mockHub struct{ events []string }

func (m *mockHub) Broadcast(eventType string, _ any) {
	m.events = append(m.events, eventType)
}

type mockBrowser struct {
	missingRemote  string
	lastExtra      []rclone.VersionRootCandidate
	versions       []rclone.FileVersion
	deletedRemotes []string
	rawConf        string
	importedConf   string
	importedMode   string
	importRemotes  []string
	importErr      error
}

func (m *mockBrowser) EnsureRemoteExists(_ context.Context, name string) error {
	if m.missingRemote != "" && name == m.missingRemote {
		return rclone.ErrRemoteNotFound
	}
	return nil
}

func (m *mockBrowser) DeleteRemote(_ context.Context, name string) error {
	if m.missingRemote != "" && name == m.missingRemote {
		return rclone.ErrRemoteNotFound
	}
	m.deletedRemotes = append(m.deletedRemotes, name)
	return nil
}

func (m *mockBrowser) ReadRawConf(_ context.Context) (string, error) {
	return m.rawConf, nil
}

func (m *mockBrowser) ImportConf(_ context.Context, content, mode string) ([]string, error) {
	if m.importErr != nil {
		return nil, m.importErr
	}
	m.rawConf = content
	m.importedConf = content
	m.importedMode = mode
	if len(m.importRemotes) > 0 {
		return m.importRemotes, nil
	}
	return []string{"b2"}, nil
}

func (m *mockBrowser) ClearConf(_ context.Context) error {
	m.rawConf = ""
	m.importedConf = ""
	m.importedMode = "cleared"
	return nil
}

func (m *mockBrowser) ListVersions(_ context.Context, _, _ string, extra []rclone.VersionRootCandidate) ([]rclone.FileVersion, error) {
	m.lastExtra = extra
	return m.versions, nil
}

func newTestService() (*AppService, *mockStore, *mockRunner, *mockScheduler, *mockHub, *mockBrowser) {
	st := newMockStore()
	rn := &mockRunner{}
	sc := &mockScheduler{}
	hb := &mockHub{}
	br := &mockBrowser{}
	defaults := domain.GlobalSettings{
		Transfers:         4,
		Checkers:          8,
		Timezone:          "UTC",
		MaxConcurrentJobs: 2,
	}
	return New(st, rn, sc, hb, br, defaults, slog.Default()), st, rn, sc, hb, br
}

func TestUpdateSettingsAndImportBundleValidation(t *testing.T) {
	ctx := context.Background()
	svc, _, rn, sc, _, _ := newTestService()

	if _, err := svc.UpdateSettings(ctx, domain.GlobalSettings{Timezone: "Mars/Olympus"}); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation for bad timezone, got %v", err)
	}
	if _, err := svc.UpdateSettings(ctx, domain.GlobalSettings{UploadBatchTransfers: 99}); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation for upload_batch_transfers > 64, got %v", err)
	}
	if _, err := svc.UpdateSettings(ctx, domain.GlobalSettings{UploadBatchMaxMB: 1024}); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation for upload_batch_max_mb > 512, got %v", err)
	}

	updated, err := svc.UpdateSettings(ctx, domain.GlobalSettings{
		Timezone:             "Asia/Tokyo",
		MaxConcurrentJobs:    5,
		UploadBatchTransfers: 24,
		UploadBatchCheckers:  48,
		UploadBatchMaxFiles:  120,
		UploadBatchMaxMB:     128,
		UploadSmallFileMaxMB: 32,
	})
	if err != nil {
		t.Fatalf("UpdateSettings failed: %v", err)
	}
	if updated.UploadBatchTransfers != 24 || updated.UploadBatchMaxMB != 128 {
		t.Fatalf("unexpected upload batch settings: %+v", updated)
	}
	if updated.Timezone != "Asia/Tokyo" || rn.maxConcurrent != 5 || sc.reloads != 1 {
		t.Fatalf("unexpected side effects: updated=%+v maxConcurrent=%d reloads=%d", updated, rn.maxConcurrent, sc.reloads)
	}

	badBundle := domain.ExportBundle{
		Settings: domain.GlobalSettings{Timezone: "Invalid/Zone"},
	}
	if err := svc.ImportBundle(ctx, badBundle, "merge"); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation for invalid bundle timezone, got %v", err)
	}

	// Webhook settings validation & TestWebhook
	if _, err := svc.UpdateSettings(ctx, domain.GlobalSettings{Timezone: "UTC", WebhookURL: "ftp://bad.example.com"}); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation for non-http webhook URL, got %v", err)
	}
	if _, err := svc.UpdateSettings(ctx, domain.GlobalSettings{Timezone: "UTC", WebhookURL: "https://hooks.example.com/1", WebhookOn: "bogus"}); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation for invalid webhook_on, got %v", err)
	}
	if err := svc.TestWebhook(ctx, ""); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation when testing empty webhook URL, got %v", err)
	}
	var testedSettings domain.GlobalSettings
	svc.webhookSettingsTester = func(_ context.Context, settings domain.GlobalSettings) error {
		testedSettings = settings
		return nil
	}
	if _, err := svc.UpdateSettings(ctx, domain.GlobalSettings{Timezone: "UTC", WebhookURL: "https://hooks.example.com/1", WebhookOn: "always", WebhookType: "ntfy"}); err != nil {
		t.Fatalf("UpdateSettings valid webhook failed: %v", err)
	}
	if err := svc.TestWebhook(ctx, ""); err != nil || testedSettings.WebhookURL != "https://hooks.example.com/1" || testedSettings.WebhookType != "ntfy" {
		t.Fatalf("TestWebhook fallback to saved settings failed: err=%v testedSettings=%+v", err, testedSettings)
	}
	if err := svc.TestWebhookSettings(ctx, "https://api.telegram.org/botx/sendMessage", "telegram", "", ""); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation when telegram chat ID missing, got %v", err)
	}
	if err := svc.TestWebhookSettings(ctx, "", "telegram", "123456:TOKEN", "-100123"); err != nil || testedSettings.TelegramBotToken != "123456:TOKEN" || testedSettings.TelegramChatID != "-100123" {
		t.Fatalf("TestWebhookSettings with bot token failed: err=%v testedSettings=%+v", err, testedSettings)
	}
	// Saving masked TelegramBotToken preserves existing stored token.
	if _, err := svc.UpdateSettings(ctx, domain.GlobalSettings{Timezone: "UTC", WebhookType: "telegram", TelegramBotToken: "999:SECRET", TelegramChatID: "-100999"}); err != nil {
		t.Fatalf("UpdateSettings telegram failed: %v", err)
	}
	preserved, err := svc.UpdateSettings(ctx, domain.GlobalSettings{Timezone: "UTC", WebhookType: "telegram", TelegramBotToken: rclone.MaskedSecret, TelegramChatID: "-100999"})
	if err != nil || preserved.TelegramBotToken != "999:SECRET" {
		t.Fatalf("expected masked TelegramBotToken preserved, got %+v err=%v", preserved, err)
	}

	// Plaintext ExportBundle must mask TelegramBotToken, and ImportBundle must preserve stored secret when given MaskedSecret.
	exported, err := svc.ExportBundle(ctx)
	if err != nil {
		t.Fatalf("ExportBundle failed: %v", err)
	}
	if exported.Settings.TelegramBotToken != rclone.MaskedSecret {
		t.Fatalf("expected ExportBundle to mask TelegramBotToken, got %q", exported.Settings.TelegramBotToken)
	}
	if err := svc.ImportBundle(ctx, exported, "merge"); err != nil {
		t.Fatalf("ImportBundle with masked token failed: %v", err)
	}
	afterImport, err := svc.GetSettings(ctx)
	if err != nil || afterImport.TelegramBotToken != "999:SECRET" {
		t.Fatalf("expected ImportBundle to preserve stored TelegramBotToken, got %+v err=%v", afterImport, err)
	}
}

func TestDeleteGuards(t *testing.T) {
	ctx := context.Background()
	svc, st, rn, _, _, _ := newTestService()
	st.jobs["j1"] = domain.Job{ID: "j1", Name: "J1", Source: "/a", Destination: "/b"}
	rn.runningJobID = "j1"

	if err := svc.DeleteJob(ctx, "j1"); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict when deleting active job, got %v", err)
	}

	st.runs["r1"] = domain.JobRun{ID: "r1", Status: "running"}
	if err := svc.DeleteRun(ctx, "r1"); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict when deleting running run, got %v", err)
	}
}

func TestReadRunLogsTailsOversizedFile(t *testing.T) {
	ctx := context.Background()
	svc, st, _, _, _, _ := newTestService()

	logPath := filepath.Join(t.TempDir(), "big.log")
	var sb strings.Builder
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&sb, "{\"level\":\"info\",\"msg\":\"line-%03d\"}\n", i)
	}
	if err := os.WriteFile(logPath, []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	st.runs["r-big"] = domain.JobRun{ID: "r-big", LogFile: logPath}

	res, err := svc.ReadRunLogs(ctx, "r-big", 1024, 15)
	if err != nil {
		t.Fatalf("ReadRunLogs failed: %v", err)
	}
	if !res.Truncated {
		t.Fatal("expected Truncated=true for tailed log")
	}
	if len(res.Lines) != 15 {
		t.Fatalf("expected 15 tailed lines, got %d", len(res.Lines))
	}
	if res.Lines[len(res.Lines)-1].Msg != "line-199" {
		t.Fatalf("expected final line to be line-199, got %q", res.Lines[len(res.Lines)-1].Msg)
	}
}

func TestStartRestoreAndListFileVersions(t *testing.T) {
	ctx := context.Background()
	svc, st, rn, _, _, br := newTestService()
	br.missingRemote = "ghost"

	// Unknown remote returns ErrRemoteNotFound
	if _, err := svc.StartRestore(ctx, rclone.RestoreRequest{
		SourceRemote: "ghost",
		SourcePath:   "a.txt",
		TargetPath:   "/tmp/a.txt",
	}); !errors.Is(err, rclone.ErrRemoteNotFound) {
		t.Fatalf("expected ErrRemoteNotFound, got %v", err)
	}

	// Invalid conflict_mode returns ErrValidation
	if _, err := svc.StartRestore(ctx, rclone.RestoreRequest{
		SourceRemote: "r2",
		SourcePath:   "a.txt",
		TargetPath:   "/tmp/a.txt",
		ConflictMode: "bogus",
	}); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}

	// Skip conflict + dry_run maps to --ignore-existing + restore_dry_run
	run, err := svc.StartRestore(ctx, rclone.RestoreRequest{
		SourceRemote: "r2",
		SourcePath:   "_versions/2026-11-26/photo.jpg",
		TargetPath:   "/data/photo.jpg",
		ConflictMode: "skip",
		DryRun:       true,
	})
	if err != nil {
		t.Fatalf("StartRestore failed: %v", err)
	}
	if run.Trigger != "restore_dry_run" || !run.DryRun {
		t.Fatalf("unexpected run metadata: %+v", run)
	}
	if rn.lastJob.Operation != "copyto" || rn.lastJob.CustomFlags != "--ignore-existing" {
		t.Fatalf("unexpected synthetic restore job: %+v", rn.lastJob)
	}

	// Job with custom backup_dir_base should be passed as candidate to ListVersions
	st.jobs["j1"] = domain.Job{
		ID:                "j1",
		Name:              "Photos",
		Destination:       "r2:backups/photos",
		VersioningEnabled: true,
		BackupDirBase:     "r2:archive/photos-vers",
	}
	if _, err := svc.ListFileVersions(ctx, "r2", "backups/photos/2026/a.jpg"); err != nil {
		t.Fatalf("ListFileVersions failed: %v", err)
	}
	if len(br.lastExtra) != 1 || br.lastExtra[0].BackupRoot != "archive/photos-vers" || br.lastExtra[0].RelPath != "2026/a.jpg" {
		t.Fatalf("unexpected extra version candidates: %+v", br.lastExtra)
	}
}

func TestDeleteRemoteGuards(t *testing.T) {
	ctx := context.Background()
	svc, st, _, _, _, br := newTestService()

	st.jobs["j-src"] = domain.Job{ID: "j-src", Name: "FromR2", Source: "r2:bucket", Destination: "/local"}
	st.jobs["j-dst"] = domain.Job{ID: "j-dst", Name: "ToB2", Source: "/local", Destination: "b2:backups"}
	st.jobs["j-ver"] = domain.Job{ID: "j-ver", Name: "CustomVer", Source: "/local", Destination: "s3:main", BackupDirBase: "archive:vers"}

	for _, name := range []string{"r2", "b2", "archive"} {
		if err := svc.DeleteRemote(ctx, name); !errors.Is(err, ErrConflict) {
			t.Fatalf("expected ErrConflict when deleting referenced remote %q, got %v", name, err)
		}
	}

	if err := svc.DeleteRemote(ctx, "unused"); err != nil {
		t.Fatalf("expected unreferenced remote delete to succeed, got %v", err)
	}
	if len(br.deletedRemotes) != 1 || br.deletedRemotes[0] != "unused" {
		t.Fatalf("expected unused remote deleted, got %v", br.deletedRemotes)
	}
}

func TestValidateJobAndJobsLifecycle(t *testing.T) {
	ctx := context.Background()
	svc, _, _, sc, hb, _ := newTestService()

	badJobs := []domain.Job{
		{Name: "", Source: "/a", Destination: "/b"},
		{Name: "NoSrc", Source: "", Destination: "/b"},
		{Name: "NoDst", Source: "/a", Destination: ""},
		{Name: "BadOp", Source: "/a", Destination: "/b", Operation: "invalid_op"},
		{Name: "BadDelTiming", Source: "/a", Destination: "/b", DeleteTiming: "later"},
		{Name: "BadSymlink", Source: "/a", Destination: "/b", SymlinkMode: "hardlink"},
		{Name: "BadBisync", Source: "/a", Destination: "/b", BisyncConflictResolve: "random"},
		{Name: "BadNotify", Source: "/a", Destination: "/b", NotifyOverride: "sometimes"},
		{Name: "BadCron", Source: "/a", Destination: "/b", Schedule: "bad cron"},
		{Name: "BadCutoff", Source: "/a", Destination: "/b", CutoffMode: "instant"},
		{Name: "BadFlag", Source: "/a", Destination: "/b", CustomFlags: "--rc"},
		{Name: "BadDryRunFlag", Source: "/a", Destination: "/b", CustomFlags: "--dry-run=false"},
		{Name: "BadLine\nName", Source: "/a", Destination: "/b"},
		{Name: "HyphenSrc", Source: "--dry-run=false", Destination: "/b"},
		{Name: "HyphenDst", Source: "/a", Destination: "-n"},
		{Name: "HyphenBackup", Source: "/a", Destination: "/b", BackupDirBase: "-rf"},
		{Name: "BadVerFormatSlash", Source: "/a", Destination: "/b", VersionFormat: "2006/01"},
		{Name: "BadVerFormatDots", Source: "/a", Destination: "/b", VersionFormat: "../escape"},
	}
	for _, bj := range badJobs {
		if _, err := svc.CreateJob(ctx, bj); !errors.Is(err, ErrValidation) {
			t.Errorf("CreateJob(%+v) expected ErrValidation, got %v", bj.Name, err)
		}
	}

	created, err := svc.CreateJob(ctx, domain.Job{
		Name:        "Docs Backup",
		Enabled:     true,
		Source:      "/docs",
		Destination: "r2:docs",
		Schedule:    "0 * * * *",
	})
	if err != nil {
		t.Fatalf("CreateJob failed: %v", err)
	}
	if created.Operation != "sync" {
		t.Fatalf("expected default operation 'sync', got %q", created.Operation)
	}

	got, err := svc.GetJob(ctx, created.ID)
	if err != nil || got.NextRun == nil {
		t.Fatalf("GetJob unexpected result: got=%+v err=%v", got, err)
	}

	list, err := svc.ListJobs(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListJobs unexpected: len=%d err=%v", len(list), err)
	}

	created.Name = "Docs Copy"
	created.Operation = "copy"
	if _, err := svc.UpdateJob(ctx, created.ID, *created); err != nil {
		t.Fatalf("UpdateJob copy failed: %v", err)
	}
	for _, validOp := range []string{"move", "check", "bisync"} {
		j := domain.Job{Name: "Op-" + validOp, Operation: validOp, Source: "/src", Destination: "r2:dst"}
		if err := ValidateJob(&j); err != nil {
			t.Fatalf("ValidateJob(%s) failed: %v", validOp, err)
		}
	}

	previewCmd, err := svc.PreviewJobCommand(ctx, domain.Job{
		Name:                  "Preview Bisync",
		Operation:             "bisync",
		Source:                "/local",
		Destination:           "r2:sync",
		BisyncConflictResolve: "newer",
		MinAge:                "1h",
	}, true, true)
	if err != nil {
		t.Fatalf("PreviewJobCommand failed: %v", err)
	}
	if !strings.Contains(previewCmd, "rclone ") || !strings.Contains(previewCmd, "bisync") || !strings.Contains(previewCmd, "--resync") || !strings.Contains(previewCmd, "--dry-run") {
		t.Fatalf("unexpected PreviewJobCommand output: %s", previewCmd)
	}
	updated, err := svc.UpdateJob(ctx, created.ID, *created)
	if err != nil || updated.Name != "Docs Copy" {
		t.Fatalf("UpdateJob failed: updated=%+v err=%v", updated, err)
	}

	cloned, err := svc.CloneJob(ctx, created.ID)
	if err != nil {
		t.Fatalf("CloneJob failed: %v", err)
	}
	if cloned.Enabled || cloned.Name != "Docs Copy (Copy)" {
		t.Fatalf("unexpected cloned job: %+v", cloned)
	}

	toggled, err := svc.ToggleJob(ctx, cloned.ID)
	if err != nil || !toggled.Enabled {
		t.Fatalf("ToggleJob failed: toggled=%+v err=%v", toggled, err)
	}

	if err := svc.DeleteJob(ctx, cloned.ID); err != nil {
		t.Fatalf("DeleteJob failed: %v", err)
	}
	if sc.reloads < 5 || len(hb.events) < 5 {
		t.Fatalf("expected scheduler reloads and broadcasts, reloads=%d events=%d", sc.reloads, len(hb.events))
	}
}

func TestReadRunLogsEdgeCases(t *testing.T) {
	ctx := context.Background()
	svc, st, _, _, _, _ := newTestService()

	// Missing log file returns empty result without error.
	st.runs["r-missing"] = domain.JobRun{ID: "r-missing", LogFile: filepath.Join(t.TempDir(), "does-not-exist.log")}
	missingRes, err := svc.ReadRunLogs(ctx, "r-missing", 0, 0)
	if err != nil || len(missingRes.Lines) != 0 {
		t.Fatalf("expected empty lines for missing log file, got %+v err=%v", missingRes, err)
	}

	// Mixed JSON and plain lines respect maxLines.
	logPath := filepath.Join(t.TempDir(), "mixed.log")
	content := strings.Join([]string{
		`{"time":"2026-11-27T01:00:00Z","level":"error","msg":"copy failed","object":"a.txt","size":42}`,
		`{"time":"2026-11-27T01:00:01Z","level":"notice","msg":"","stats":{"bytes":10}}`,
		`plain text message`,
	}, "\n")
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	st.runs["r-mixed"] = domain.JobRun{ID: "r-mixed", LogFile: logPath}

	res, err := svc.ReadRunLogs(ctx, "r-mixed", 0, 2)
	if err != nil {
		t.Fatalf("ReadRunLogs mixed failed: %v", err)
	}
	if !res.Truncated || len(res.Lines) != 2 {
		t.Fatalf("expected 2 tailed lines with Truncated=true, got %+v", res)
	}
	if !res.Lines[0].IsStats || res.Lines[1].Msg != "plain text message" {
		t.Fatalf("unexpected parsed log lines: %+v", res.Lines)
	}
}

func TestListLocalDirs(t *testing.T) {
	svc, _, _, _, _, _ := newTestService()

	if _, _, err := svc.ListLocalDirs("relative/path"); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation for relative path, got %v", err)
	}
	if _, _, err := svc.ListLocalDirs(filepath.Join(t.TempDir(), "missing")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected os.ErrNotExist for missing path, got %v", err)
	}

	root := t.TempDir()
	for _, name := range []string{"zeta", "Alpha", ".hidden"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	targetDir := t.TempDir()
	if err := os.Symlink(targetDir, filepath.Join(root, "linkdir")); err != nil {
		t.Fatal(err)
	}

	clean, dirs, err := svc.ListLocalDirs(root + "/")
	if err != nil {
		t.Fatalf("ListLocalDirs: %v", err)
	}
	if clean != filepath.Clean(root) {
		t.Fatalf("expected clean path %q, got %q", filepath.Clean(root), clean)
	}
	if len(dirs) != 4 {
		t.Fatalf("expected 4 dirs (excluding file.txt), got %+v", dirs)
	}
	wantOrder := []string{"Alpha", "linkdir", "zeta", ".hidden"}
	for i, want := range wantOrder {
		if dirs[i].Name != want || !dirs[i].IsDir {
			t.Fatalf("entry %d = %+v, want Name=%q IsDir=true", i, dirs[i], want)
		}
	}
}
