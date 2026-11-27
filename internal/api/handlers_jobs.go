package api

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/kevindiu/rclone-manager/internal/db"
	"github.com/kevindiu/rclone-manager/internal/domain"
	"github.com/kevindiu/rclone-manager/internal/runner"
	"github.com/kevindiu/rclone-manager/internal/scheduler"
	"github.com/kevindiu/rclone-manager/internal/service"
)

func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := s.svc.ListJobs(r.Context())
	if err != nil {
		s.writeInternalError(w, r, "list jobs failed", err)
		return
	}
	writeJSON(w, http.StatusOK, jobs)
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	job, err := s.svc.GetJob(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		s.writeInternalError(w, r, "get job failed", err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	var job domain.Job
	if err := decodeJSON(r, &job); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload: "+err.Error())
		return
	}
	created, err := s.svc.CreateJob(r.Context(), job)
	if errors.Is(err, service.ErrValidation) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		s.writeInternalError(w, r, "create job failed", err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleUpdateJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var job domain.Job
	if err := decodeJSON(r, &job); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload: "+err.Error())
		return
	}
	updated, err := s.svc.UpdateJob(r.Context(), id, job)
	if errors.Is(err, service.ErrValidation) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		s.writeInternalError(w, r, "update job failed", err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := s.svc.DeleteJob(r.Context(), id)
	if errors.Is(err, service.ErrConflict) {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		s.writeInternalError(w, r, "delete job failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleCloneJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	created, err := s.svc.CloneJob(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		s.writeInternalError(w, r, "clone job failed", err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleToggleJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	updated, err := s.svc.ToggleJob(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		s.writeInternalError(w, r, "toggle job failed", err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleRunJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		DryRun bool `json:"dry_run"`
		Resync bool `json:"resync"`
	}
	if err := decodeJSON(r, &req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid JSON payload: "+err.Error())
		return
	}

	run, err := s.svc.RunJobWithResync(r.Context(), id, req.DryRun, req.Resync)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	if errors.Is(err, runner.ErrJobAlreadyRunning) {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if errors.Is(err, runner.ErrShuttingDown) {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	if err != nil {
		s.writeInternalError(w, r, "run job failed", err)
		return
	}
	writeJSON(w, http.StatusAccepted, run)
}

func (s *Server) handlePreviewJobCommand(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Job    domain.Job `json:"job"`
		DryRun bool       `json:"dry_run"`
		Resync bool       `json:"resync"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload: "+err.Error())
		return
	}
	cmd, err := s.svc.PreviewJobCommand(r.Context(), req.Job, req.DryRun, req.Resync)
	if errors.Is(err, service.ErrValidation) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		s.writeInternalError(w, r, "preview job command failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"command": cmd})
}

func (s *Server) handleDryRunDraftJob(w http.ResponseWriter, r *http.Request) {
	var job domain.Job
	if err := decodeJSON(r, &job); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload: "+err.Error())
		return
	}
	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
	res, err := s.svc.DryRunDraftJob(r.Context(), job)
	if errors.Is(err, service.ErrValidation) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		s.writeInternalError(w, r, "draft dry run failed", err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleStopJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.svc.StopJob(id); errors.Is(err, runner.ErrJobNotRunning) {
		writeError(w, http.StatusConflict, err.Error())
		return
	} else if err != nil {
		s.writeInternalError(w, r, "stop job failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopping"})
}

func (s *Server) handleCronPreview(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Schedule string `json:"schedule"`
		Timezone string `json:"timezone"`
		Count    int    `json:"count"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload: "+err.Error())
		return
	}
	tz := strings.TrimSpace(req.Timezone)
	if tz == "" {
		if settings, err := s.svc.GetSettings(r.Context()); err == nil {
			tz = settings.Timezone
		}
	}
	times, err := scheduler.PreviewCron(req.Schedule, tz, req.Count)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"timezone": tz,
		"next":     times,
	})
}
