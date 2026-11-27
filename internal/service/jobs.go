package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/kevindiu/rclone-manager/internal/domain"
	"github.com/kevindiu/rclone-manager/internal/rclone"
	"github.com/kevindiu/rclone-manager/internal/scheduler"
)

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
	switch job.Operation {
	case "copy", "sync", "move", "check":
	default:
		return errors.New("operation must be 'copy', 'sync', 'move', or 'check'")
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
