package runner

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/kevindiu/rclone-manager/internal/domain"
	"github.com/kevindiu/rclone-manager/internal/notify"
	"github.com/kevindiu/rclone-manager/internal/rclone"
)

// ErrJobAlreadyRunning is returned when attempting to start a job that is already queued or running.
var ErrJobAlreadyRunning = errors.New("job is already running")

// ErrJobNotRunning is returned when attempting to stop a job that is not currently active.
var ErrJobNotRunning = errors.New("job is not running")

type activeRun struct {
	mu       sync.RWMutex
	snapshot domain.ActiveRunSnapshot
	cancel   context.CancelFunc
	stopped  bool
}

// Runner manages job execution, concurrency, overlap protection, logs, and SSE broadcasts.
type Runner struct {
	store    RunStore
	client   RcloneOps
	hub      Broadcaster
	notifier RunNotifier
	logger   *slog.Logger
	logDir   string
	defaults domain.GlobalSettings

	gate *concurrencyGate

	mu     sync.RWMutex
	active map[string]*activeRun // keyed by jobID
	wg     sync.WaitGroup
}

// NewRunner creates a new job Runner.
func NewRunner(store RunStore, client RcloneOps, hub Broadcaster, logDir string, defaults domain.GlobalSettings, logger *slog.Logger) *Runner {
	if logger == nil {
		logger = slog.Default()
	}
	maxConcurrent := defaults.MaxConcurrentJobs
	if s, err := store.GetSettings(context.Background(), defaults); err == nil && s.MaxConcurrentJobs > 0 {
		maxConcurrent = s.MaxConcurrentJobs
	}
	return &Runner{
		store:    store,
		client:   client,
		hub:      hub,
		logger:   logger,
		logDir:   logDir,
		defaults: defaults,
		gate:     newConcurrencyGate(maxConcurrent),
		active:   make(map[string]*activeRun),
	}
}

// SetNotifier configures an optional external notifier for finished runs.
func (r *Runner) SetNotifier(n RunNotifier) {
	r.mu.Lock()
	r.notifier = n
	r.mu.Unlock()
}

// UpdateMaxConcurrent updates the concurrency limit dynamically.
func (r *Runner) UpdateMaxConcurrent(limit int) {
	r.gate.setLimit(limit)
}

// GetActiveSnapshots returns a snapshot of all currently queued or running jobs.
func (r *Runner) GetActiveSnapshots() []domain.ActiveRunSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]domain.ActiveRunSnapshot, 0, len(r.active))
	for _, ar := range r.active {
		ar.mu.RLock()
		out = append(out, ar.snapshot)
		ar.mu.RUnlock()
	}
	return out
}

// ActiveRunByJob returns the active run ID for jobID if running.
func (r *Runner) ActiveRunByJob(jobID string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ar, ok := r.active[jobID]
	if !ok {
		return "", false
	}
	ar.mu.RLock()
	id := ar.snapshot.RunID
	ar.mu.RUnlock()
	return id, true
}

// StartJob launches a job asynchronously while enforcing per-job overlap protection.
func (r *Runner) StartJob(job domain.Job, trigger string, dryRun bool) (*domain.JobRun, error) {
	now := time.Now().UTC()
	if dryRun && trigger == "" {
		trigger = "dry_run"
	}
	if trigger == "" {
		trigger = "manual"
	}

	r.mu.Lock()
	if _, running := r.active[job.ID]; running {
		r.mu.Unlock()
		if trigger == "schedule" {
			skippedRun := domain.JobRun{
				ID:           domain.NewID(),
				JobID:        job.ID,
				JobName:      job.Name,
				Trigger:      trigger,
				DryRun:       dryRun,
				Status:       "skipped",
				StartedAt:    now,
				FinishedAt:   &now,
				ErrorMessage: "Skipped: previous run still in progress",
			}
			if err := r.store.CreateRun(context.Background(), skippedRun); err != nil {
				r.logger.Warn("failed to record skipped run", "job_id", job.ID, "error", err)
			}
			r.hub.Broadcast("run_completed", skippedRun)
		}
		return nil, ErrJobAlreadyRunning
	}

	runID := domain.NewID()
	logPath := filepath.Join(r.logDir, runID+".log")
	run := domain.JobRun{
		ID:        runID,
		JobID:     job.ID,
		JobName:   job.Name,
		Trigger:   trigger,
		DryRun:    dryRun,
		Status:    "queued",
		StartedAt: now,
		LogFile:   logPath,
	}
	if err := r.store.CreateRun(context.Background(), run); err != nil {
		r.mu.Unlock()
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	ar := &activeRun{
		snapshot: domain.ActiveRunSnapshot{
			RunID:     runID,
			JobID:     job.ID,
			JobName:   job.Name,
			Trigger:   trigger,
			DryRun:    dryRun,
			Status:    "queued",
			StartedAt: now,
		},
		cancel: cancel,
	}
	r.active[job.ID] = ar
	r.wg.Add(1)
	r.mu.Unlock()

	r.hub.Broadcast("run_started", ar.snapshot)
	go r.execute(ctx, job, run, ar)

	return &run, nil
}

// StopJob gracefully cancels a running or queued job.
// Subprocess termination and SIGKILL fallback are handled safely by exec.CommandContext.
func (r *Runner) StopJob(jobID string) error {
	r.mu.RLock()
	ar, ok := r.active[jobID]
	r.mu.RUnlock()
	if !ok {
		return ErrJobNotRunning
	}

	ar.mu.Lock()
	ar.stopped = true
	ar.mu.Unlock()

	ar.cancel()
	return nil
}

// Wait waits for all currently active runs to finish.
func (r *Runner) Wait() {
	r.wg.Wait()
}

// Shutdown cancels all running jobs and waits for them to clean up.
func (r *Runner) Shutdown() {
	r.mu.RLock()
	ids := make([]string, 0, len(r.active))
	for id := range r.active {
		ids = append(ids, id)
	}
	r.mu.RUnlock()
	for _, id := range ids {
		_ = r.StopJob(id)
	}
	r.wg.Wait()
}

func (r *Runner) execute(ctx context.Context, job domain.Job, run domain.JobRun, ar *activeRun) {
	defer r.wg.Done()
	defer ar.cancel()
	defer func() {
		r.mu.Lock()
		delete(r.active, job.ID)
		r.mu.Unlock()
	}()

	finish := func(status, errMsg string) {
		r.finishRun(&run, status, errMsg, job.NotifyOverride)
	}

	settings, err := r.store.GetSettings(ctx, r.defaults)
	if err != nil {
		settings = r.defaults
	}
	r.gate.setLimit(settings.MaxConcurrentJobs)

	if err := r.gate.acquire(ctx); err != nil {
		finish("cancelled", "Cancelled while queued")
		return
	}
	defer r.gate.release()

	if err := os.MkdirAll(r.logDir, 0o755); err != nil {
		finish("failed", fmt.Sprintf("Failed to create log dir: %v", err))
		return
	}
	logFile, err := os.OpenFile(run.LogFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		finish("failed", fmt.Sprintf("Failed to open log file: %v", err))
		return
	}
	defer func() {
		if closeErr := logFile.Close(); closeErr != nil {
			r.logger.Warn("failed to close run log file", "run_id", run.ID, "error", closeErr)
		}
	}()

	writeLog := func(level, msg string) {
		line := map[string]any{
			"time":  time.Now().UTC().Format(time.RFC3339Nano),
			"level": level,
			"msg":   msg,
		}
		if b, err := json.Marshal(line); err == nil {
			_, _ = logFile.Write(append(b, '\n'))
		}
	}

	ar.mu.Lock()
	ar.snapshot.Status = "running"
	ar.mu.Unlock()
	run.Status = "running"
	if err := r.store.UpdateRun(context.Background(), run); err != nil {
		r.logger.Warn("failed to update run status to running", "run_id", run.ID, "error", err)
	}
	r.hub.Broadcast("progress", ar.snapshot)

	// Pre-flight safety guards
	if job.AbortIfSourceEmpty {
		writeLog("info", fmt.Sprintf("Pre-flight check: verifying source %q is not empty...", job.Source))
		nonEmpty, err := r.client.CheckSourceNotEmpty(ctx, job.Source)
		if err != nil {
			writeLog("error", "Pre-flight check failed: "+err.Error())
			finish("failed", "Pre-flight source check failed: "+err.Error())
			return
		}
		if !nonEmpty {
			msg := "Pre-flight guard aborted run: source directory is empty"
			writeLog("error", msg)
			finish("failed", msg)
			return
		}
	}

	if strings.TrimSpace(job.RequiredMarkerFile) != "" {
		writeLog("info", fmt.Sprintf("Pre-flight check: verifying marker file %q exists in %q...", job.RequiredMarkerFile, job.Source))
		exists, err := r.client.CheckMarkerExists(ctx, job.Source, job.RequiredMarkerFile)
		if err != nil || !exists {
			msg := fmt.Sprintf("Pre-flight guard aborted run: required marker file %q not found", job.RequiredMarkerFile)
			if err != nil {
				msg += " (" + err.Error() + ")"
			}
			writeLog("error", msg)
			finish("failed", msg)
			return
		}
	}

	loc, err := time.LoadLocation(settings.Timezone)
	if err != nil {
		loc = time.UTC
	}

	args, backupBase, datedBackupDir, err := BuildRcloneArgs(job, settings, r.client.ConfPath(), run.DryRun, time.Now().In(loc))
	if err != nil {
		writeLog("error", "Failed to build rclone command: "+err.Error())
		finish("failed", err.Error())
		return
	}

	run.BackupDirUsed = datedBackupDir
	run.CommandPreview = r.client.Bin() + " " + FormatCommandPreview(args)
	if err := r.store.UpdateRun(context.Background(), run); err != nil {
		r.logger.Warn("failed to update run command preview", "run_id", run.ID, "error", err)
	}
	writeLog("info", "Executing: "+run.CommandPreview)

	pr, pw, err := os.Pipe()
	if err != nil {
		writeLog("error", "Pipe creation failed: "+err.Error())
		finish("failed", err.Error())
		return
	}

	cmd := exec.CommandContext(ctx, r.client.Bin(), args...)
	cmd.Stdout = pw
	cmd.Stderr = pw
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			return cmd.Process.Signal(syscall.SIGINT)
		}
		return nil
	}
	cmd.WaitDelay = 10 * time.Second

	if err := cmd.Start(); err != nil {
		_ = pw.Close()
		_ = pr.Close()
		writeLog("error", "Failed to start rclone: "+err.Error())
		finish("failed", "Failed to start rclone: "+err.Error())
		return
	}
	_ = pw.Close()

	var lastErrLine string
	scanner := bufio.NewScanner(pr)
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)
	lastBroadcast := time.Time{}

	for scanner.Scan() {
		line := scanner.Text()
		_, _ = logFile.WriteString(line + "\n")

		if prog, ok := rclone.ParseProgressLine(line); ok {
			ar.mu.Lock()
			ar.snapshot.Progress = prog
			snapCopy := ar.snapshot
			ar.mu.Unlock()

			run.BytesTransferred = prog.Bytes
			run.TotalBytes = prog.TotalBytes
			run.FilesTransferred = prog.Transfers
			run.FilesChecked = prog.Checks
			run.FilesDeleted = prog.Deletes
			run.ErrorsCount = prog.Errors

			if time.Since(lastBroadcast) >= 400*time.Millisecond {
				r.hub.Broadcast("progress", snapCopy)
				lastBroadcast = time.Now()
			}
		} else if msg := extractErrorMsg(line); msg != "" {
			lastErrLine = msg
		}
	}
	_ = pr.Close()

	waitErr := cmd.Wait()

	ar.mu.RLock()
	wasStopped := ar.stopped
	finalSnap := ar.snapshot
	ar.mu.RUnlock()

	r.hub.Broadcast("progress", finalSnap)

	if wasStopped || ctx.Err() != nil {
		writeLog("warning", "Job run was stopped by user or shutdown")
		finish("cancelled", "Stopped by user")
		return
	}

	if waitErr != nil {
		errMsg := lastErrLine
		if errMsg == "" {
			errMsg = waitErr.Error()
		}
		writeLog("error", "rclone exited with error: "+errMsg)
		finish("failed", errMsg)
		return
	}

	// Post-run version retention pruning
	if !run.DryRun && job.VersioningEnabled && job.VersionRetentionDays > 0 && backupBase != "" {
		pruned, pruneErr := r.pruneExpiredVersions(ctx, backupBase, job.VersionFormat, job.VersionRetentionDays, loc, writeLog)
		if len(pruned) > 0 {
			run.PrunedVersions = pruned
		}
		if pruneErr != nil {
			r.logger.Warn("version retention pruning warning", "run_id", run.ID, "error", pruneErr)
			writeLog("warning", "Version retention pruning warning: "+pruneErr.Error())
		}
	}

	writeLog("info", "Job run completed successfully")
	finish("success", "")
}

func (r *Runner) finishRun(run *domain.JobRun, status, errMsg, notifyOverride string) {
	now := time.Now().UTC()
	run.Status = status
	run.FinishedAt = &now
	run.DurationMs = now.Sub(run.StartedAt).Milliseconds()
	if errMsg != "" {
		run.ErrorMessage = errMsg
	}
	if err := r.store.UpdateRun(context.Background(), *run); err != nil {
		r.logger.Error("failed to finalize run record", "run_id", run.ID, "status", status, "error", err)
	}
	r.hub.Broadcast("run_completed", *run)

	r.mu.RLock()
	notifier := r.notifier
	r.mu.RUnlock()
	if notifier != nil {
		settings, err := r.store.GetSettings(context.Background(), r.defaults)
		if err != nil {
			settings = r.defaults
		}
		settings.WebhookOn = notify.EffectiveWebhookOn(settings.WebhookOn, notifyOverride)
		if settings.WebhookURL != "" && notify.ShouldNotify(settings.WebhookOn, status) {
			runCopy := *run
			r.wg.Add(1)
			go func() {
				defer r.wg.Done()
				nCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				notifier.NotifyRun(nCtx, settings, runCopy)
			}()
		}
	}
}

func extractErrorMsg(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || trimmed[0] != '{' {
		return ""
	}
	var entry struct {
		Level string `json:"level"`
		Msg   string `json:"msg"`
	}
	if err := json.Unmarshal([]byte(trimmed), &entry); err != nil {
		return ""
	}
	if entry.Level == "error" || entry.Level == "critical" {
		return entry.Msg
	}
	return ""
}
