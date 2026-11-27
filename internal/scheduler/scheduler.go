package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/kevindiu/rclone-manager/internal/domain"
)

// Store defines the persistence operations required by Scheduler.
type Store interface {
	GetSettings(ctx context.Context, defaults domain.GlobalSettings) (domain.GlobalSettings, error)
	ListJobs(ctx context.Context) ([]domain.Job, error)
	GetJob(ctx context.Context, id string) (*domain.Job, error)
	PruneOldRuns(ctx context.Context, retentionDays int) (int64, error)
}

// JobStarter launches a job run when its cron schedule fires.
type JobStarter interface {
	StartJob(job domain.Job, trigger string, dryRun bool) (*domain.JobRun, error)
}

// Scheduler manages cron execution for backup jobs and periodic log retention pruning.
type Scheduler struct {
	store    Store
	runner   JobStarter
	logger   *slog.Logger
	defaults domain.GlobalSettings

	mu   sync.Mutex
	cron *cron.Cron
	loc  *time.Location
}

// New creates a Scheduler without starting it yet.
func New(store Store, r JobStarter, defaults domain.GlobalSettings, logger *slog.Logger) *Scheduler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Scheduler{
		store:    store,
		runner:   r,
		logger:   logger,
		defaults: defaults,
		loc:      time.UTC,
	}
}

// Start loads settings and jobs from SQLite and starts cron scheduling.
func (s *Scheduler) Start() error {
	return s.Reload()
}

// Stop halts scheduled runs gracefully.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	c := s.cron
	s.cron = nil
	s.mu.Unlock()
	if c != nil {
		ctx := c.Stop()
		<-ctx.Done()
	}
}

// Reload rebuilds the cron schedule from current settings and jobs in SQLite.
func (s *Scheduler) Reload() error {
	ctx := context.Background()
	settings, err := s.store.GetSettings(ctx, s.defaults)
	if err != nil {
		s.logger.Warn("scheduler failed to load settings, using defaults", "error", err)
		settings = s.defaults
	}
	jobs, err := s.store.ListJobs(ctx)
	if err != nil {
		return err
	}

	loc, err := time.LoadLocation(settings.Timezone)
	if err != nil || loc == nil {
		s.logger.Warn("scheduler invalid timezone, falling back to UTC", "timezone", settings.Timezone, "error", err)
		loc = time.UTC
	}

	newCron := cron.New(cron.WithLocation(loc))

	for _, job := range jobs {
		if !job.Enabled || strings.TrimSpace(job.Schedule) == "" {
			continue
		}
		jobID := job.ID
		jobName := job.Name
		if _, err := newCron.AddFunc(strings.TrimSpace(job.Schedule), func() {
			bgCtx := context.Background()
			latestJob, err := s.store.GetJob(bgCtx, jobID)
			if err != nil {
				s.logger.Warn("scheduled job lookup failed", "job_id", jobID, "error", err)
				return
			}
			if !latestJob.Enabled {
				return
			}
			if _, err := s.runner.StartJob(*latestJob, "schedule", false); err != nil {
				s.logger.Info("scheduled job skipped or failed to start", "job_id", jobID, "job_name", jobName, "reason", err)
			}
		}); err != nil {
			s.logger.Warn("skipping job with invalid cron expression", "job_id", jobID, "schedule", job.Schedule, "error", err)
			continue
		}
	}

	// Daily maintenance task at 03:00 in configured timezone to prune expired run history & logs.
	if _, err := newCron.AddFunc("0 3 * * *", func() {
		bgCtx := context.Background()
		curSettings, err := s.store.GetSettings(bgCtx, s.defaults)
		if err != nil {
			curSettings = s.defaults
		}
		if pruned, err := s.store.PruneOldRuns(bgCtx, curSettings.LogRetentionDays); err != nil {
			s.logger.Warn("daily log retention prune failed", "error", err)
		} else if pruned > 0 {
			s.logger.Info("daily maintenance pruned expired job runs", "count", pruned)
		}
	}); err != nil {
		s.logger.Error("failed to register maintenance cron", "error", err)
	}

	newCron.Start()

	s.mu.Lock()
	old := s.cron
	s.cron = newCron
	s.loc = loc
	s.mu.Unlock()

	if old != nil {
		old.Stop()
	}
	return nil
}

// ValidateCron validates a standard 5-field cron expression.
func ValidateCron(expr string) error {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil
	}
	_, err := cron.ParseStandard(expr)
	return err
}

// PreviewCron returns the next count scheduled times for expr in timezone tz.
func PreviewCron(expr, tz string, count int) ([]time.Time, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, fmt.Errorf("cron expression is empty")
	}
	sched, err := cron.ParseStandard(expr)
	if err != nil {
		return nil, err
	}
	loc := time.UTC
	if strings.TrimSpace(tz) != "" {
		if loaded, err := time.LoadLocation(tz); err == nil && loaded != nil {
			loc = loaded
		}
	}
	if count <= 0 {
		count = 5
	}
	if count > 20 {
		count = 20
	}

	times := make([]time.Time, 0, count)
	cur := time.Now().In(loc)
	for i := 0; i < count; i++ {
		next := sched.Next(cur)
		if next.IsZero() {
			break
		}
		times = append(times, next)
		cur = next
	}
	return times, nil
}

// NextRun returns the next scheduled run time for an enabled job's schedule.
func NextRun(expr, tz string) *time.Time {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil
	}
	times, err := PreviewCron(expr, tz, 1)
	if err != nil || len(times) == 0 {
		return nil
	}
	t := times[0]
	return &t
}
