package api

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/kevindiu/rclone-manager/internal/rclone"
	"github.com/kevindiu/rclone-manager/internal/runner"
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
	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
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

const (
	maxBatchUploadFiles = 500
	maxBatchUploadBytes = 512 << 20
)

func (s *Server) handleBrowseUploadBatch(w http.ResponseWriter, r *http.Request) {
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

	transfers := 16
	checkers := 32
	maxFiles := 80
	maxMB := 64
	bwLimit := ""
	if settings, setErr := s.svc.GetSettings(r.Context()); setErr == nil {
		if settings.UploadBatchTransfers > 0 {
			transfers = settings.UploadBatchTransfers
		}
		if settings.UploadBatchCheckers > 0 {
			checkers = settings.UploadBatchCheckers
		}
		if settings.UploadBatchMaxFiles > 0 {
			maxFiles = settings.UploadBatchMaxFiles
		}
		if settings.UploadBatchMaxMB > 0 {
			maxMB = settings.UploadBatchMaxMB
		}
		bwLimit = settings.BwLimit
	}
	maxFiles = min(max(maxFiles, 1), maxBatchUploadFiles)
	maxBytes := min(int64(max(maxMB, 1))<<20, maxBatchUploadBytes)

	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Time{})
	_ = rc.SetWriteDeadline(time.Time{})

	mr, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "multipart form-data is required: "+err.Error())
		return
	}

	tmpDir, err := os.MkdirTemp("", "rclone-upload-batch-*")
	if err != nil {
		s.writeInternalError(w, r, "create upload staging directory", err)
		return
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	tmpPrefix := tmpDir + string(os.PathSeparator)
	var pendingRelPath string
	var totalBytes int64
	uploadedCount := 0

	for {
		part, nextErr := mr.NextPart()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			writeError(w, http.StatusBadRequest, "failed to read upload stream: "+nextErr.Error())
			return
		}

		switch part.FormName() {
		case "relpath":
			rawRel, readErr := io.ReadAll(io.LimitReader(part, 4096))
			_ = part.Close()
			if readErr != nil {
				writeError(w, http.StatusBadRequest, "failed to read relpath field")
				return
			}
			pendingRelPath = strings.TrimSpace(string(rawRel))
		case "file":
			if uploadedCount >= maxFiles {
				_ = part.Close()
				writeError(w, http.StatusBadRequest, fmt.Sprintf("batch upload exceeds %d files limit", maxFiles))
				return
			}

			rel := pendingRelPath
			pendingRelPath = ""
			if rel == "" {
				rel = part.FileName()
			}
			rel = strings.NewReplacer("\r", "", "\n", "").Replace(rel)
			cleanRel, cleanErr := rclone.CleanSubpath(rel)
			if cleanErr != nil || cleanRel == "" {
				_ = part.Close()
				writeError(w, http.StatusBadRequest, "invalid relative upload path")
				return
			}

			destPath := filepath.Join(tmpDir, filepath.FromSlash(cleanRel))
			if !strings.HasPrefix(destPath, tmpPrefix) {
				_ = part.Close()
				writeError(w, http.StatusBadRequest, "invalid relative upload path")
				return
			}
			if mkErr := os.MkdirAll(filepath.Dir(destPath), 0o700); mkErr != nil {
				_ = part.Close()
				s.writeInternalError(w, r, "create upload staging subdirectory", mkErr)
				return
			}

			dst, openErr := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
			if openErr != nil {
				_ = part.Close()
				s.writeInternalError(w, r, "create upload staging file", openErr)
				return
			}

			remaining := maxBytes - totalBytes
			n, copyErr := io.Copy(dst, io.LimitReader(part, remaining+1))
			closeDstErr := dst.Close()
			_ = part.Close()
			if copyErr != nil {
				writeError(w, http.StatusBadRequest, "failed to stream uploaded file: "+copyErr.Error())
				return
			}
			if closeDstErr != nil {
				s.writeInternalError(w, r, "close upload staging file", closeDstErr)
				return
			}
			totalBytes += n
			if totalBytes > maxBytes {
				writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("batch upload exceeds %d MiB limit", maxBytes>>20))
				return
			}
			uploadedCount++
		default:
			_ = part.Close()
		}
	}

	if uploadedCount == 0 {
		writeError(w, http.StatusBadRequest, "at least one file is required")
		return
	}

	if uploadErr := s.client.UploadDir(r.Context(), tmpDir, remote, cleanDir, transfers, checkers, bwLimit); uploadErr != nil {
		status := http.StatusBadRequest
		if errors.Is(uploadErr, rclone.ErrRemoteNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, uploadErr.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"status": "uploaded",
		"count":  uploadedCount,
		"path":   cleanDir,
	})
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
		Remote  string                   `json:"remote"`
		Entries []rclone.BulkDeleteEntry `json:"entries"`
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

	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
	deleted, failed, err := s.client.BulkDeletePaths(r.Context(), req.Remote, req.Entries)
	if err != nil {
		if errors.Is(err, rclone.ErrRemoteNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
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

type lazyDownloadWriter struct {
	w           http.ResponseWriter
	contentType string
	disposition string
	size        int64
	started     bool
}

func (l *lazyDownloadWriter) initHeaders() {
	if l.started {
		return
	}
	l.started = true
	l.w.Header().Set("Content-Type", l.contentType)
	l.w.Header().Set("Content-Disposition", l.disposition)
	if l.size >= 0 {
		l.w.Header().Set("Content-Length", fmt.Sprintf("%d", l.size))
	}
}

func (l *lazyDownloadWriter) Write(p []byte) (int, error) {
	if !l.started {
		l.initHeaders()
	}
	return l.w.Write(p)
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

	lw := &lazyDownloadWriter{
		w:           w,
		contentType: contentType,
		disposition: disposition,
		size:        stat.Size,
	}

	if err := s.client.StreamFile(r.Context(), remote, subpath, lw); err != nil {
		s.logger.Warn("browse download stream failed", "remote", remote, "path", subpath, "error", err)
		if !lw.started {
			writeError(w, http.StatusBadGateway, "failed to download file")
		}
		return
	}
	if !lw.started {
		lw.initHeaders()
		w.WriteHeader(http.StatusOK)
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
	if errors.Is(err, runner.ErrShuttingDown) {
		writeError(w, http.StatusServiceUnavailable, err.Error())
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
	if errors.Is(err, runner.ErrShuttingDown) {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	if err != nil {
		s.writeInternalError(w, r, "start restore failed", err)
		return
	}
	writeJSON(w, http.StatusAccepted, run)
}
