package api

import (
	"errors"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/kevindiu/rclone-manager/internal/db"
	"github.com/kevindiu/rclone-manager/internal/service"
)

func (s *Server) handleListRuns(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	jobID := q.Get("job_id")
	status := q.Get("status")
	trigger := q.Get("trigger")
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))

	runs, total, err := s.svc.ListRuns(r.Context(), jobID, status, trigger, limit, offset)
	if err != nil {
		s.writeInternalError(w, r, "list runs failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"runs":  runs,
		"total": total,
	})
}

func (s *Server) handleRunStats(w http.ResponseWriter, r *http.Request) {
	days := 14
	if raw := r.URL.Query().Get("days"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			days = parsed
		}
	}
	if days < 1 {
		days = 1
	} else if days > 90 {
		days = 90
	}
	stats, err := s.svc.GetDailyRunStats(r.Context(), days)
	if err != nil {
		s.writeInternalError(w, r, "get daily run stats failed", err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) handleClearRuns(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	deleted, err := s.svc.ClearFinishedRuns(r.Context(), q.Get("job_id"), q.Get("status"), q.Get("trigger"))
	if err != nil {
		s.writeInternalError(w, r, "clear finished runs failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"deleted": deleted})
}

func (s *Server) handleGetRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	run, err := s.svc.GetRun(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "run not found")
		return
	}
	if err != nil {
		s.writeInternalError(w, r, "get run failed", err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (s *Server) handleGetRunLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if r.URL.Query().Get("download") == "1" {
		logPath, err := s.svc.GetRunLogFilePath(r.Context(), id)
		if errors.Is(err, db.ErrNotFound) {
			writeError(w, http.StatusNotFound, "run not found")
			return
		}
		if err != nil {
			s.writeInternalError(w, r, "get run log path failed", err)
			return
		}

		rc := http.NewResponseController(w)
		_ = rc.SetWriteDeadline(time.Time{})

		filename := "run-" + filepath.Base(id) + ".log"
		disposition := mime.FormatMediaType("attachment", map[string]string{"filename": filename})
		if disposition == "" {
			disposition = "attachment"
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", disposition)

		if logPath == "" {
			w.WriteHeader(http.StatusOK)
			return
		}
		f, err := os.Open(logPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				w.WriteHeader(http.StatusOK)
				return
			}
			s.writeInternalError(w, r, "open run log file failed", err)
			return
		}
		defer func() { _ = f.Close() }()

		info, err := f.Stat()
		if err != nil {
			s.writeInternalError(w, r, "stat run log file failed", err)
			return
		}
		http.ServeContent(w, r, filename, info.ModTime(), f)
		return
	}

	maxLines, _ := strconv.Atoi(r.URL.Query().Get("max_lines"))
	res, err := s.svc.ReadRunLogs(r.Context(), id, 0, maxLines)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "run not found")
		return
	}
	if err != nil {
		s.writeInternalError(w, r, "read run logs failed", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"run_id":      id,
		"lines":       res.Lines,
		"raw":         res.Raw,
		"truncated":   res.Truncated,
		"total_bytes": res.TotalBytes,
	})
}

func (s *Server) handleDeleteRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := s.svc.DeleteRun(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "run not found")
		return
	}
	if errors.Is(err, service.ErrConflict) {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		s.writeInternalError(w, r, "delete run failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
