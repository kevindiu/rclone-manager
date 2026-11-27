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
)

type mockStore struct {
	settings domain.GlobalSettings
	jobs     map[string]domain.Job
	runs     map[string]domain.JobRun
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
	m.settings = bundle.Settings
	for _, j := range bundle.Jobs {
		m.jobs[j.ID] = j
	}
	return nil
}
func (m *mockStore) ListRuns(_ context.Context, _, _ string, _, _ int) ([]domain.JobRun, int, error) {
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

type mockRunner struct {
	maxConcurrent int
	runningJobID  string
}

func (m *mockRunner) UpdateMaxConcurrent(limit int) { m.maxConcurrent = limit }
func (m *mockRunner) ActiveRunByJob(jobID string) (string, bool) {
	if m.runningJobID == jobID {
		return "run-active", true
	}
	return "", false
}
func (m *mockRunner) StartJob(job domain.Job, trigger string, dryRun bool) (*domain.JobRun, error) {
	return &domain.JobRun{ID: "run-1", JobID: job.ID, Trigger: trigger, DryRun: dryRun}, nil
}
func (m *mockRunner) StopJob(string) error { return nil }

type mockScheduler struct{ reloads int }

func (m *mockScheduler) Reload() error {
	m.reloads++
	return nil
}

type mockHub struct{ events []string }

func (m *mockHub) Broadcast(eventType string, _ any) {
	m.events = append(m.events, eventType)
}

func newTestService() (*AppService, *mockStore, *mockRunner, *mockScheduler, *mockHub) {
	st := newMockStore()
	rn := &mockRunner{}
	sc := &mockScheduler{}
	hb := &mockHub{}
	defaults := domain.GlobalSettings{
		Transfers:         4,
		Checkers:          8,
		Timezone:          "UTC",
		MaxConcurrentJobs: 2,
	}
	return New(st, rn, sc, hb, defaults, slog.Default()), st, rn, sc, hb
}

func TestUpdateSettingsAndImportBundleValidation(t *testing.T) {
	ctx := context.Background()
	svc, _, rn, sc, _ := newTestService()

	if _, err := svc.UpdateSettings(ctx, domain.GlobalSettings{Timezone: "Mars/Olympus"}); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation for bad timezone, got %v", err)
	}

	updated, err := svc.UpdateSettings(ctx, domain.GlobalSettings{Timezone: "Asia/Tokyo", MaxConcurrentJobs: 5})
	if err != nil {
		t.Fatalf("UpdateSettings failed: %v", err)
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
}

func TestDeleteGuards(t *testing.T) {
	ctx := context.Background()
	svc, st, rn, _, _ := newTestService()
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
	svc, st, _, _, _ := newTestService()

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
