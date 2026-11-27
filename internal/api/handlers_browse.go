package api

import (
	"errors"
	"fmt"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/kevindiu/rclone-manager/internal/rclone"
)

func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	remote := strings.TrimSpace(r.URL.Query().Get("remote"))
	subpath := r.URL.Query().Get("path")
	if remote == "" {
		writeError(w, http.StatusBadRequest, "remote query parameter is required")
		return
	}
	entries, err := s.client.Browse(r.Context(), remote, subpath)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, rclone.ErrRemoteNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	cleanSub, _ := rclone.CleanSubpath(subpath)
	writeJSON(w, http.StatusOK, map[string]any{
		"remote":  remote,
		"path":    cleanSub,
		"entries": entries,
	})
}

func (s *Server) handleBrowseDownload(w http.ResponseWriter, r *http.Request) {
	remote := strings.TrimSpace(r.URL.Query().Get("remote"))
	subpath := r.URL.Query().Get("path")
	if remote == "" || strings.TrimSpace(subpath) == "" {
		writeError(w, http.StatusBadRequest, "remote and path query parameters are required")
		return
	}

	stat, err := s.client.StatFile(r.Context(), remote, subpath)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if stat.IsDir {
		writeError(w, http.StatusBadRequest, "cannot download a directory directly")
		return
	}

	// Disable write deadline for large file downloads.
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})

	filename := path.Base(strings.ReplaceAll(subpath, "\\", "/"))
	if stat.Name != "" {
		filename = stat.Name
	}
	filename = strings.NewReplacer("\r", "", "\n", "").Replace(filename)

	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": filename})
	if disposition == "" {
		disposition = "attachment"
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", disposition)
	if stat.Size >= 0 {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size))
	}

	if err := s.client.StreamFile(r.Context(), remote, subpath, w); err != nil {
		s.logger.Warn("browse download stream failed", "remote", remote, "path", subpath, "error", err)
		return
	}
}

func (s *Server) handleBrowseRestore(w http.ResponseWriter, r *http.Request) {
	var req rclone.RestoreRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload: "+err.Error())
		return
	}
	output, err := s.client.Restore(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "restored",
		"message": output,
	})
}
