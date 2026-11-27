package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/kevindiu/rclone-manager/internal/domain"
	"github.com/kevindiu/rclone-manager/internal/rclone"
)

// ErrValidation indicates invalid client-supplied input.
var ErrValidation = errors.New("validation error")

// ErrConflict indicates an operation conflicts with current runtime state.
var ErrConflict = errors.New("conflict")

// Store defines persistence operations needed by AppService.
type Store interface {
	GetSettings(ctx context.Context, defaults domain.GlobalSettings) (domain.GlobalSettings, error)
	SaveSettings(ctx context.Context, settings domain.GlobalSettings) error
	ListJobs(ctx context.Context) ([]domain.Job, error)
	GetJob(ctx context.Context, id string) (*domain.Job, error)
	CreateJob(ctx context.Context, job domain.Job) (*domain.Job, error)
	UpdateJob(ctx context.Context, id string, job domain.Job) (*domain.Job, error)
	DeleteJob(ctx context.Context, id string) error
	ImportBundle(ctx context.Context, bundle domain.ExportBundle, mode string, defaults domain.GlobalSettings) error
	ListRuns(ctx context.Context, jobID, status string, limit, offset int) ([]domain.JobRun, int, error)
	GetRun(ctx context.Context, id string) (*domain.JobRun, error)
	GetLastRunsByJobIDs(ctx context.Context) (map[string]domain.JobRun, error)
	DeleteRun(ctx context.Context, id string) error
}

// JobRunner defines execution controls needed by AppService.
type JobRunner interface {
	UpdateMaxConcurrent(limit int)
	ActiveRunByJob(jobID string) (string, bool)
	StartJob(job domain.Job, trigger string, dryRun bool) (*domain.JobRun, error)
	StopJob(jobID string) error
}

// ScheduleReloader reloads cron triggers when settings or jobs change.
type ScheduleReloader interface {
	Reload() error
}

// Broadcaster publishes real-time events to connected SSE clients.
type Broadcaster interface {
	Broadcast(eventType string, data any)
}

// RcloneBrowser defines remote inspection, deletion, and version-discovery operations needed by AppService.
type RcloneBrowser interface {
	EnsureRemoteExists(ctx context.Context, name string) error
	DeleteRemote(ctx context.Context, name string) error
	ListVersions(ctx context.Context, remote, subpath string, extra []rclone.VersionRootCandidate) ([]rclone.FileVersion, error)
}

// ParsedLogLine represents a single parsed rclone JSON log entry.
type ParsedLogLine struct {
	Time    string `json:"time,omitempty"`
	Level   string `json:"level,omitempty"`
	Msg     string `json:"msg"`
	Object  string `json:"object,omitempty"`
	Size    *int64 `json:"size,omitempty"`
	IsStats bool   `json:"is_stats,omitempty"`
}

// LogReadResult holds a bounded log preview and truncation metadata.
type LogReadResult struct {
	Lines      []ParsedLogLine `json:"lines"`
	Raw        string          `json:"raw"`
	Truncated  bool            `json:"truncated"`
	TotalBytes int64           `json:"total_bytes"`
}

// JobWithStatus enriches a Job with live runtime and schedule metadata.
type JobWithStatus struct {
	domain.Job
	NextRun     *time.Time     `json:"next_run,omitempty"`
	IsRunning   bool           `json:"is_running"`
	ActiveRunID string         `json:"active_run_id,omitempty"`
	LastRun     *domain.JobRun `json:"last_run,omitempty"`
}

// AppService orchestrates settings, jobs, runs, scheduling, remotes, and event broadcasting.
type AppService struct {
	store     Store
	runner    JobRunner
	scheduler ScheduleReloader
	hub       Broadcaster
	browser   RcloneBrowser
	logger    *slog.Logger
	defaults  domain.GlobalSettings
}

// New creates a new AppService.
func New(
	store Store,
	runner JobRunner,
	sched ScheduleReloader,
	hub Broadcaster,
	browser RcloneBrowser,
	defaults domain.GlobalSettings,
	logger *slog.Logger,
) *AppService {
	if logger == nil {
		logger = slog.Default()
	}
	return &AppService{
		store:     store,
		runner:    runner,
		scheduler: sched,
		hub:       hub,
		browser:   browser,
		logger:    logger,
		defaults:  defaults,
	}
}

func (s *AppService) reloadScheduler() {
	if err := s.scheduler.Reload(); err != nil {
		s.logger.Warn("failed to reload scheduler", "error", err)
	}
}

// GetSettings returns global settings.
func (s *AppService) GetSettings(ctx context.Context) (domain.GlobalSettings, error) {
	return s.store.GetSettings(ctx, s.defaults)
}

// UpdateSettings validates, saves, and applies global settings.
func (s *AppService) UpdateSettings(ctx context.Context, incoming domain.GlobalSettings) (domain.GlobalSettings, error) {
	if incoming.Timezone != "" {
		if _, err := time.LoadLocation(incoming.Timezone); err != nil {
			return domain.GlobalSettings{}, fmt.Errorf("%w: invalid timezone: %s", ErrValidation, incoming.Timezone)
		}
	}
	if incoming.MaxConcurrentJobs < 1 {
		incoming.MaxConcurrentJobs = 1
	}

	if err := s.store.SaveSettings(ctx, incoming); err != nil {
		return domain.GlobalSettings{}, err
	}
	s.runner.UpdateMaxConcurrent(incoming.MaxConcurrentJobs)
	s.reloadScheduler()

	updated, err := s.store.GetSettings(ctx, s.defaults)
	if err != nil {
		return incoming, nil
	}
	return updated, nil
}

// ExportBundle exports settings and all jobs into a portable bundle.
func (s *AppService) ExportBundle(ctx context.Context) (domain.ExportBundle, error) {
	settings, err := s.store.GetSettings(ctx, s.defaults)
	if err != nil {
		return domain.ExportBundle{}, err
	}
	jobs, err := s.store.ListJobs(ctx)
	if err != nil {
		return domain.ExportBundle{}, err
	}
	return domain.ExportBundle{
		Version:    1,
		ExportedAt: time.Now().UTC(),
		Settings:   settings,
		Jobs:       jobs,
	}, nil
}

// ImportBundle validates and imports settings and jobs atomically.
func (s *AppService) ImportBundle(ctx context.Context, bundle domain.ExportBundle, mode string) error {
	if mode != "merge" && mode != "replace" {
		return fmt.Errorf("%w: mode must be merge or replace", ErrValidation)
	}
	if bundle.Settings.Timezone != "" {
		if _, err := time.LoadLocation(bundle.Settings.Timezone); err != nil {
			return fmt.Errorf("%w: invalid timezone: %s", ErrValidation, bundle.Settings.Timezone)
		}
	}
	for i := range bundle.Jobs {
		if err := ValidateJob(&bundle.Jobs[i]); err != nil {
			return fmt.Errorf("%w: job %q: %v", ErrValidation, bundle.Jobs[i].Name, err)
		}
	}
	if err := s.store.ImportBundle(ctx, bundle, mode, s.defaults); err != nil {
		return err
	}
	s.runner.UpdateMaxConcurrent(bundle.Settings.MaxConcurrentJobs)
	s.reloadScheduler()
	s.hub.Broadcast("job_updated", map[string]any{"imported": len(bundle.Jobs)})
	return nil
}

// DeleteRemote deletes a remote only if no configured job still references it.
func (s *AppService) DeleteRemote(ctx context.Context, name string) error {
	cleanName := strings.TrimSpace(name)
	jobs, err := s.store.ListJobs(ctx)
	if err != nil {
		return err
	}

	var refs []string
	for _, job := range jobs {
		if jobReferencesRemote(job, cleanName) {
			refs = append(refs, job.Name)
		}
	}
	if len(refs) > 0 {
		return fmt.Errorf("%w: remote %q is referenced by job(s): %s", ErrConflict, cleanName, strings.Join(refs, ", "))
	}

	if s.browser == nil {
		return errors.New("rclone client unavailable")
	}
	return s.browser.DeleteRemote(ctx, cleanName)
}

func jobReferencesRemote(job domain.Job, remoteName string) bool {
	if remoteName == "" {
		return false
	}
	for _, candidate := range []string{job.Source, job.Destination, job.BackupDirBase} {
		if rem, _, ok := splitRemotePath(candidate); ok && rem == remoteName {
			return true
		}
	}
	return false
}
