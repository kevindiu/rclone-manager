package api

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/kevindiu/rclone-manager/internal/rclone"
	"github.com/kevindiu/rclone-manager/internal/service"
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

func (s *Server) handleBrowseLocal(w http.ResponseWriter, r *http.Request) {
	dirPath := r.URL.Query().Get("path")
	cleanPath, entries, err := s.svc.ListLocalDirs(dirPath)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrValidation):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, os.ErrNotExist):
			writeError(w, http.StatusNotFound, "directory does not exist")
		case errors.Is(err, os.ErrPermission):
			writeError(w, http.StatusForbidden, "permission denied reading directory")
		default:
			s.writeInternalError(w, r, "list local directories failed", err)
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"path":    cleanPath,
		"entries": entries,
	})
}

func (s *Server) handleBrowseSize(w http.ResponseWriter, r *http.Request) {
	remote := strings.TrimSpace(r.URL.Query().Get("remote"))
	subpath := r.URL.Query().Get("path")
	if remote == "" {
		writeError(w, http.StatusBadRequest, "remote query parameter is required")
		return
	}
	sz, err := s.client.DirSize(r.Context(), remote, subpath)
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
		"remote": remote,
		"path":   cleanSub,
		"count":  sz.Count,
		"bytes":  sz.Bytes,
	})
}

func (s *Server) handleBrowseMkdir(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Remote string `json:"remote"`
		Path   string `json:"path"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload: "+err.Error())
		return
	}
	req.Remote = strings.TrimSpace(req.Remote)
	if req.Remote == "" || strings.TrimSpace(req.Path) == "" {
		writeError(w, http.StatusBadRequest, "remote and path are required")
		return
	}
	if err := s.client.MakeDir(r.Context(), req.Remote, req.Path); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, rclone.ErrRemoteNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "created"})
}

func (s *Server) handleBrowseUpload(w http.ResponseWriter, r *http.Request) {
	remote := strings.TrimSpace(r.URL.Query().Get("remote"))
	dirPath := r.URL.Query().Get("path")
	if remote == "" {
		writeError(w, http.StatusBadRequest, "remote query parameter is required")
		return
	}

	cleanDir, err := rclone.CleanSubpath(dirPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Time{})
	_ = rc.SetWriteDeadline(time.Time{})

	mr, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "multipart form-data is required: "+err.Error())
		return
	}

	for {
		part, nextErr := mr.NextPart()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			writeError(w, http.StatusBadRequest, "failed to read upload stream: "+nextErr.Error())
			return
		}
		if part.FormName() != "file" {
			_ = part.Close()
			continue
		}

		filename := path.Base(strings.ReplaceAll(strings.TrimSpace(part.FileName()), "\\", "/"))
		filename = strings.NewReplacer("\r", "", "\n", "").Replace(filename)
		if filename == "" || filename == "." || filename == ".." || filename == "/" {
			_ = part.Close()
			writeError(w, http.StatusBadRequest, "invalid upload filename")
			return
		}

		targetPath := filename
		if cleanDir != "" {
			targetPath = cleanDir + "/" + filename
		}
		uploadErr := s.client.UploadFile(r.Context(), remote, targetPath, part)
		_ = part.Close()
		if uploadErr != nil {
			status := http.StatusBadRequest
			if errors.Is(uploadErr, rclone.ErrRemoteNotFound) {
				status = http.StatusNotFound
			}
			writeError(w, status, uploadErr.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{
			"status": "uploaded",
			"path":   targetPath,
			"name":   filename,
		})
		return
	}

	writeError(w, http.StatusBadRequest, "multipart field \"file\" is required")
}

func (s *Server) handleBrowseRename(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Remote  string `json:"remote"`
		Path    string `json:"path"`
		NewName string `json:"new_name"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload: "+err.Error())
		return
	}
	req.Remote = strings.TrimSpace(req.Remote)
	newName := strings.TrimSpace(req.NewName)
	if req.Remote == "" || strings.TrimSpace(req.Path) == "" || newName == "" {
		writeError(w, http.StatusBadRequest, "remote, path, and new_name are required")
		return
	}
	if strings.ContainsAny(newName, "/\\") || newName == "." || newName == ".." {
		writeError(w, http.StatusBadRequest, "new_name must be a single file or folder name")
		return
	}
	cleanSrc, err := rclone.CleanSubpath(req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if cleanSrc == "" {
		writeError(w, http.StatusBadRequest, "cannot rename remote root")
		return
	}
	parent := path.Dir(cleanSrc)
	dstPath := newName
	if parent != "." && parent != "/" {
		dstPath = parent + "/" + newName
	}

	if err := s.client.RenamePath(r.Context(), req.Remote, cleanSrc, dstPath); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, rclone.ErrRemoteNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "renamed",
		"path":   dstPath,
	})
}

func (s *Server) handleBrowseDelete(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	remote := strings.TrimSpace(q.Get("remote"))
	subpath := q.Get("path")
	isDir := q.Get("is_dir") == "1" || strings.EqualFold(q.Get("is_dir"), "true")
	if remote == "" || strings.TrimSpace(subpath) == "" {
		writeError(w, http.StatusBadRequest, "remote and path query parameters are required")
		return
	}
	if err := s.client.DeletePath(r.Context(), remote, subpath, isDir); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, rclone.ErrRemoteNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleBrowseBulkDelete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Remote  string `json:"remote"`
		Entries []struct {
			Path  string `json:"path"`
			IsDir bool   `json:"is_dir"`
		} `json:"entries"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload: "+err.Error())
		return
	}
	req.Remote = strings.TrimSpace(req.Remote)
	if req.Remote == "" || len(req.Entries) == 0 {
		writeError(w, http.StatusBadRequest, "remote and non-empty entries are required")
		return
	}
	if len(req.Entries) > 200 {
		writeError(w, http.StatusBadRequest, "bulk delete supports at most 200 entries per request")
		return
	}
	for _, entry := range req.Entries {
		if strings.TrimSpace(entry.Path) == "" {
			writeError(w, http.StatusBadRequest, "every entry must include a path")
			return
		}
	}
	if err := s.client.EnsureRemoteExists(r.Context(), req.Remote); err != nil {
		if errors.Is(err, rclone.ErrRemoteNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	type bulkFailure struct {
		Path  string `json:"path"`
		Error string `json:"error"`
	}
	deleted := 0
	failed := make([]bulkFailure, 0)
	for _, entry := range req.Entries {
		if err := s.client.DeletePath(r.Context(), req.Remote, entry.Path, entry.IsDir); err != nil {
			failed = append(failed, bulkFailure{Path: entry.Path, Error: err.Error()})
		} else {
			deleted++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"deleted": deleted,
		"failed":  failed,
	})
}

func safeInlineContentType(filename string) (string, bool) {
	ext := strings.ToLower(path.Ext(filename))
	switch ext {
	case ".png":
		return "image/png", true
	case ".jpg", ".jpeg":
		return "image/jpeg", true
	case ".gif":
		return "image/gif", true
	case ".webp":
		return "image/webp", true
	case ".pdf":
		return "application/pdf", true
	case ".txt", ".md", ".log", ".json", ".yaml", ".yml", ".toml", ".ini", ".conf", ".cfg",
		".csv", ".xml", ".sh", ".bash", ".zsh", ".go", ".ts", ".tsx", ".js", ".jsx",
		".py", ".rs", ".c", ".h", ".cpp", ".java", ".css", ".sql", ".env", ".gitignore":
		return "text/plain; charset=utf-8", true
	}
	base := strings.ToLower(path.Base(filename))
	if base == "dockerfile" || base == "makefile" || base == ".gitignore" || base == ".env" {
		return "text/plain; charset=utf-8", true
	}
	return "", false
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

	contentType := "application/octet-stream"
	dispType := "attachment"
	inlineReq := r.URL.Query().Get("inline") == "1" || strings.EqualFold(r.URL.Query().Get("inline"), "true")
	if inlineReq {
		if safeType, ok := safeInlineContentType(filename); ok {
			contentType = safeType
			dispType = "inline"
		}
	}

	disposition := mime.FormatMediaType(dispType, map[string]string{"filename": filename})
	if disposition == "" {
		disposition = dispType
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", disposition)
	if stat.Size >= 0 {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size))
	}

	if err := s.client.StreamFile(r.Context(), remote, subpath, w); err != nil {
		s.logger.Warn("browse download stream failed", "remote", remote, "path", subpath, "error", err)
		return
	}
}

func (s *Server) handleBrowseVersions(w http.ResponseWriter, r *http.Request) {
	remote := strings.TrimSpace(r.URL.Query().Get("remote"))
	subpath := r.URL.Query().Get("path")
	if remote == "" || strings.TrimSpace(subpath) == "" {
		writeError(w, http.StatusBadRequest, "remote and path query parameters are required")
		return
	}
	versions, err := s.svc.ListFileVersions(r.Context(), remote, subpath)
	if errors.Is(err, rclone.ErrRemoteNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if errors.Is(err, service.ErrValidation) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		s.writeInternalError(w, r, "browse versions failed", err)
		return
	}
	cleanSub, _ := rclone.CleanSubpath(subpath)
	writeJSON(w, http.StatusOK, map[string]any{
		"remote":   remote,
		"path":     cleanSub,
		"versions": versions,
	})
}

func (s *Server) handleBrowseRestore(w http.ResponseWriter, r *http.Request) {
	var req rclone.RestoreRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload: "+err.Error())
		return
	}
	run, err := s.svc.StartRestore(r.Context(), req)
	if errors.Is(err, rclone.ErrRemoteNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if errors.Is(err, service.ErrValidation) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		s.writeInternalError(w, r, "start restore failed", err)
		return
	}
	writeJSON(w, http.StatusAccepted, run)
}
