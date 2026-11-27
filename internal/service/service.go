package service

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"strings"
	"time"

	"github.com/kevindiu/rclone-manager/internal/db"
	"github.com/kevindiu/rclone-manager/internal/domain"
	"github.com/kevindiu/rclone-manager/internal/rclone"
	"github.com/kevindiu/rclone-manager/internal/scheduler"
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

// RcloneBrowser defines remote inspection & version-discovery operations needed by AppService.
type RcloneBrowser interface {
	EnsureRemoteExists(ctx context.Context, name string) error
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

// AppService orchestrates settings, jobs, runs, scheduling, and event broadcasting.
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

// ListJobs returns all jobs enriched with live runtime status and next scheduled run.
func (s *AppService) ListJobs(ctx context.Context) ([]JobWithStatus, error) {
	jobs, err := s.store.ListJobs(ctx)
	if err != nil {
		return nil, err
	}
	settings, err := s.store.GetSettings(ctx, s.defaults)
	if err != nil {
		settings = s.defaults
	}
	lastRuns, err := s.store.GetLastRunsByJobIDs(ctx)
	if err != nil {
		s.logger.Warn("failed to load last runs map", "error", err)
	}

	out := make([]JobWithStatus, 0, len(jobs))
	for _, job := range jobs {
		out = append(out, s.enrichJob(job, settings.Timezone, lastRuns))
	}
	return out, nil
}

// GetJob returns a single enriched job by ID.
func (s *AppService) GetJob(ctx context.Context, id string) (JobWithStatus, error) {
	job, err := s.store.GetJob(ctx, id)
	if err != nil {
		return JobWithStatus{}, err
	}
	settings, err := s.store.GetSettings(ctx, s.defaults)
	if err != nil {
		settings = s.defaults
	}
	lastRuns, err := s.store.GetLastRunsByJobIDs(ctx)
	if err != nil {
		s.logger.Warn("failed to load last runs map", "error", err)
	}
	return s.enrichJob(*job, settings.Timezone, lastRuns), nil
}

// CreateJob validates and persists a new job, then reloads the scheduler.
func (s *AppService) CreateJob(ctx context.Context, job domain.Job) (*domain.Job, error) {
	if err := ValidateJob(&job); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	created, err := s.store.CreateJob(ctx, job)
	if err != nil {
		return nil, err
	}
	s.reloadScheduler()
	s.hub.Broadcast("job_updated", created)
	return created, nil
}

// UpdateJob validates and updates an existing job, then reloads the scheduler.
func (s *AppService) UpdateJob(ctx context.Context, id string, job domain.Job) (*domain.Job, error) {
	if err := ValidateJob(&job); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	updated, err := s.store.UpdateJob(ctx, id, job)
	if err != nil {
		return nil, err
	}
	s.reloadScheduler()
	s.hub.Broadcast("job_updated", updated)
	return updated, nil
}

// DeleteJob deletes an idle job and reloads the scheduler.
func (s *AppService) DeleteJob(ctx context.Context, id string) error {
	if _, running := s.runner.ActiveRunByJob(id); running {
		return fmt.Errorf("%w: cannot delete job while it is running", ErrConflict)
	}
	if err := s.store.DeleteJob(ctx, id); err != nil {
		return err
	}
	s.reloadScheduler()
	s.hub.Broadcast("job_updated", map[string]string{"deleted_id": id})
	return nil
}

// CloneJob duplicates an existing job (disabled by default).
func (s *AppService) CloneJob(ctx context.Context, id string) (*domain.Job, error) {
	existing, err := s.store.GetJob(ctx, id)
	if err != nil {
		return nil, err
	}
	clone := *existing
	clone.ID = ""
	clone.Name = existing.Name + " (Copy)"
	clone.Enabled = false

	created, err := s.store.CreateJob(ctx, clone)
	if err != nil {
		return nil, err
	}
	s.reloadScheduler()
	s.hub.Broadcast("job_updated", created)
	return created, nil
}

// ToggleJob flips a job's enabled state and reloads the scheduler.
func (s *AppService) ToggleJob(ctx context.Context, id string) (*domain.Job, error) {
	job, err := s.store.GetJob(ctx, id)
	if err != nil {
		return nil, err
	}
	job.Enabled = !job.Enabled
	updated, err := s.store.UpdateJob(ctx, id, *job)
	if err != nil {
		return nil, err
	}
	s.reloadScheduler()
	s.hub.Broadcast("job_updated", updated)
	return updated, nil
}

// RunJob starts a manual or dry-run execution of a job.
func (s *AppService) RunJob(ctx context.Context, id string, dryRun bool) (*domain.JobRun, error) {
	job, err := s.store.GetJob(ctx, id)
	if err != nil {
		return nil, err
	}
	trigger := "manual"
	if dryRun {
		trigger = "dry_run"
	}
	return s.runner.StartJob(*job, trigger, dryRun)
}

// StopJob gracefully stops an active job run.
func (s *AppService) StopJob(id string) error {
	return s.runner.StopJob(id)
}

// StartRestore launches an asynchronous restore job via Runner so progress streams over SSE
// and execution history/logs are persisted in SQLite.
func (s *AppService) StartRestore(ctx context.Context, req rclone.RestoreRequest) (*domain.JobRun, error) {
	if s.browser != nil {
		if err := s.browser.EnsureRemoteExists(ctx, req.SourceRemote); err != nil {
			return nil, err
		}
	}
	srcTarget, err := rclone.FormatRemoteTarget(req.SourceRemote, req.SourcePath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	target := strings.TrimSpace(req.TargetPath)
	if target == "" {
		return nil, fmt.Errorf("%w: target_path is required", ErrValidation)
	}

	var customFlags string
	switch strings.TrimSpace(req.ConflictMode) {
	case "", "overwrite":
	case "skip":
		customFlags = "--ignore-existing"
	case "newer_only":
		customFlags = "--update"
	default:
		return nil, fmt.Errorf("%w: invalid conflict_mode %q", ErrValidation, req.ConflictMode)
	}

	op := "copyto"
	if req.IsDir {
		op = "copy"
	}

	cleanSub, _ := rclone.CleanSubpath(req.SourcePath)
	label := path.Base(cleanSub)
	if label == "" || label == "." {
		label = req.SourceRemote + ":"
	}

	restoreJob := domain.Job{
		ID:          "restore-" + db.NewID(),
		Name:        "Restore: " + label,
		Operation:   op,
		Source:      srcTarget,
		Destination: target,
		CustomFlags: customFlags,
	}

	trigger := "restore"
	if req.DryRun {
		trigger = "restore_dry_run"
	}
	return s.runner.StartJob(restoreJob, trigger, req.DryRun)
}

// ListFileVersions discovers historical backup-dir versions of a path on remote.
func (s *AppService) ListFileVersions(ctx context.Context, remote, subpath string) ([]rclone.FileVersion, error) {
	if s.browser == nil {
		return []rclone.FileVersion{}, nil
	}
	cleanSub, err := rclone.CleanSubpath(subpath)
	if err != nil || cleanSub == "" {
		return nil, fmt.Errorf("%w: valid file or folder path is required", ErrValidation)
	}

	var extra []rclone.VersionRootCandidate
	if jobs, err := s.store.ListJobs(ctx); err == nil {
		for _, job := range jobs {
			if !job.VersioningEnabled {
				continue
			}
			destRemote, destSub, ok := splitRemotePath(job.Destination)
			if !ok || destRemote != remote {
				continue
			}
			rel, inside := relativeSub(destSub, cleanSub)
			if !inside {
				continue
			}
			backupRoot := path.Join(destSub, "_versions")
			if strings.TrimSpace(job.BackupDirBase) != "" {
				bRem, bSub, ok := splitRemotePath(job.BackupDirBase)
				if !ok || bRem != remote {
					continue
				}
				backupRoot = bSub
			}
			extra = append(extra, rclone.VersionRootCandidate{
				BackupRoot: backupRoot,
				RelPath:    rel,
			})
		}
	}

	return s.browser.ListVersions(ctx, remote, cleanSub, extra)
}

func splitRemotePath(p string) (remote string, subpath string, ok bool) {
	trimmed := strings.TrimSpace(p)
	if trimmed == "" || strings.HasPrefix(trimmed, "/") {
		return "", "", false
	}
	idx := strings.IndexByte(trimmed, ':')
	if idx <= 0 {
		return "", "", false
	}
	remote = trimmed[:idx]
	sub, err := rclone.CleanSubpath(trimmed[idx+1:])
	if err != nil {
		return "", "", false
	}
	return remote, sub, true
}

func relativeSub(baseSub, targetSub string) (string, bool) {
	if baseSub == "" {
		return targetSub, targetSub != ""
	}
	prefix := baseSub + "/"
	if strings.HasPrefix(targetSub, prefix) {
		rel := strings.TrimPrefix(targetSub, prefix)
		return rel, rel != ""
	}
	return "", false
}

// ListRuns lists historical job runs.
func (s *AppService) ListRuns(ctx context.Context, jobID, status string, limit, offset int) ([]domain.JobRun, int, error) {
	return s.store.ListRuns(ctx, jobID, status, limit, offset)
}

// GetRun returns a single job run.
func (s *AppService) GetRun(ctx context.Context, id string) (*domain.JobRun, error) {
	return s.store.GetRun(ctx, id)
}

// DeleteRun removes a completed run record and its log file.
func (s *AppService) DeleteRun(ctx context.Context, id string) error {
	run, err := s.store.GetRun(ctx, id)
	if err != nil {
		return err
	}
	if run.Status == "running" || run.Status == "queued" {
		return fmt.Errorf("%w: cannot delete an active run", ErrConflict)
	}
	return s.store.DeleteRun(ctx, id)
}

// GetRunLogFilePath returns the log file path for a run after verifying the run exists.
func (s *AppService) GetRunLogFilePath(ctx context.Context, id string) (string, error) {
	run, err := s.store.GetRun(ctx, id)
	if err != nil {
		return "", err
	}
	return run.LogFile, nil
}

// ReadRunLogs loads and parses the tail of a run's log file without loading huge files entirely into memory.
func (s *AppService) ReadRunLogs(ctx context.Context, id string, maxBytes int64, maxLines int) (LogReadResult, error) {
	if maxBytes <= 0 {
		maxBytes = 2 * 1024 * 1024
	}
	if maxLines <= 0 {
		maxLines = 1500
	}

	run, err := s.store.GetRun(ctx, id)
	if err != nil {
		return LogReadResult{}, err
	}
	if run.LogFile == "" {
		return LogReadResult{Lines: []ParsedLogLine{}}, nil
	}

	f, err := os.Open(run.LogFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return LogReadResult{Lines: []ParsedLogLine{}}, nil
		}
		return LogReadResult{}, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return LogReadResult{}, err
	}
	totalBytes := info.Size()
	truncated := false

	if totalBytes > maxBytes {
		if _, err := f.Seek(totalBytes-maxBytes, io.SeekStart); err != nil {
			return LogReadResult{}, err
		}
		truncated = true
	}

	rawBytes, err := io.ReadAll(io.LimitReader(f, maxBytes))
	if err != nil {
		return LogReadResult{}, err
	}
	raw := string(rawBytes)
	if truncated {
		// Drop partial first line caused by seeking into the middle of a line.
		if idx := strings.IndexByte(raw, '\n'); idx >= 0 {
			raw = raw[idx+1:]
		}
	}

	lines := make([]ParsedLogLine, 0)
	scanner := bufio.NewScanner(strings.NewReader(raw))
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var obj struct {
			Time   string `json:"time"`
			Level  string `json:"level"`
			Msg    string `json:"msg"`
			Object string `json:"object"`
			Size   *int64 `json:"size"`
			Stats  any    `json:"stats"`
		}
		if strings.HasPrefix(line, "{") && json.Unmarshal([]byte(line), &obj) == nil {
			lines = append(lines, ParsedLogLine{
				Time:    obj.Time,
				Level:   obj.Level,
				Msg:     strings.TrimSpace(obj.Msg),
				Object:  obj.Object,
				Size:    obj.Size,
				IsStats: obj.Stats != nil,
			})
		} else {
			lines = append(lines, ParsedLogLine{
				Level: "info",
				Msg:   line,
			})
		}
	}

	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
		truncated = true
		rawLines := strings.Split(strings.TrimRight(raw, "\n"), "\n")
		if len(rawLines) > maxLines {
			raw = strings.Join(rawLines[len(rawLines)-maxLines:], "\n")
		}
	}

	return LogReadResult{
		Lines:      lines,
		Raw:        raw,
		Truncated:  truncated,
		TotalBytes: totalBytes,
	}, nil
}

func (s *AppService) enrichJob(job domain.Job, timezone string, lastRuns map[string]domain.JobRun) JobWithStatus {
	out := JobWithStatus{Job: job}
	if job.Enabled && strings.TrimSpace(job.Schedule) != "" {
		out.NextRun = scheduler.NextRun(job.Schedule, timezone)
	}
	if runID, running := s.runner.ActiveRunByJob(job.ID); running {
		out.IsRunning = true
		out.ActiveRunID = runID
	}
	if lastRuns != nil {
		if lr, ok := lastRuns[job.ID]; ok {
			copyRun := lr
			out.LastRun = &copyRun
		}
	}
	return out
}

// ValidateJob validates job configuration fields and custom flags.
func ValidateJob(job *domain.Job) error {
	job.Name = strings.TrimSpace(job.Name)
	job.Source = strings.TrimSpace(job.Source)
	job.Destination = strings.TrimSpace(job.Destination)
	job.Schedule = strings.TrimSpace(job.Schedule)

	if job.Name == "" {
		return errors.New("job name is required")
	}
	if job.Source == "" {
		return errors.New("source path is required")
	}
	if job.Destination == "" {
		return errors.New("destination path is required")
	}
	if job.Operation == "" {
		job.Operation = "sync"
	}
	if job.Operation != "copy" && job.Operation != "sync" {
		return errors.New("operation must be 'copy' or 'sync'")
	}
	if job.Schedule != "" {
		if err := scheduler.ValidateCron(job.Schedule); err != nil {
			return errors.New("invalid cron schedule: " + err.Error())
		}
	}
	if job.CutoffMode != "" && job.CutoffMode != "soft" && job.CutoffMode != "hard" && job.CutoffMode != "cautious" {
		return errors.New("cutoff_mode must be soft, hard, or cautious")
	}
	if strings.TrimSpace(job.CustomFlags) != "" {
		if _, err := rclone.ParseAndValidateFlags(job.CustomFlags); err != nil {
			return err
		}
	}
	return nil
}
