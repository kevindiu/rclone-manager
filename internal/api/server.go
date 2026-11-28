package api

import (
	"encoding/json"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"runtime/debug"
	"strings"
	"time"

	"github.com/kevindiu/rclone-manager/internal/config"
	"github.com/kevindiu/rclone-manager/internal/events"
	"github.com/kevindiu/rclone-manager/internal/rclone"
	"github.com/kevindiu/rclone-manager/internal/runner"
	"github.com/kevindiu/rclone-manager/internal/service"
)

// Server wires REST API handlers, SSE streaming, and the embedded SPA.
type Server struct {
	cfg        *config.Config
	svc        *service.AppService
	auth       *service.AuthService
	client     *rclone.Client
	runner     *runner.Runner
	hub        *events.Hub
	logger     *slog.Logger
	spaFS      fs.FS
	appVersion string
	startedAt  time.Time
	csrf       *http.CrossOriginProtection
	mux        *http.ServeMux
}

// NewServer creates and configures the HTTP server router.
func NewServer(
	cfg *config.Config,
	svc *service.AppService,
	authSvc *service.AuthService,
	client *rclone.Client,
	r *runner.Runner,
	hub *events.Hub,
	spaFS fs.FS,
	appVersion string,
	logger *slog.Logger,
) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	csrf := http.NewCrossOriginProtection()
	for _, origin := range cfg.CORSOrigins {
		if origin == "*" {
			continue
		}
		if err := csrf.AddTrustedOrigin(origin); err != nil {
			logger.Warn("ignoring invalid trusted CORS origin for CSRF protection", "origin", origin, "error", err)
		}
	}
	s := &Server{
		cfg:        cfg,
		svc:        svc,
		auth:       authSvc,
		client:     client,
		runner:     r,
		hub:        hub,
		logger:     logger,
		spaFS:      spaFS,
		appVersion: appVersion,
		startedAt:  time.Now().UTC(),
		csrf:       csrf,
		mux:        http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

// Handler returns the root HTTP handler with middleware applied.
func (s *Server) Handler() http.Handler {
	return s.withMiddleware(s.mux)
}

func (s *Server) registerRoutes() {
	// Authentication (public status/setup/login/logout + protected password & session management)
	s.mux.HandleFunc("GET /api/auth/status", s.handleAuthStatus)
	s.mux.HandleFunc("POST /api/auth/setup", s.handleAuthSetup)
	s.mux.HandleFunc("POST /api/auth/login", s.handleAuthLogin)
	s.mux.HandleFunc("POST /api/auth/logout", s.handleAuthLogout)
	s.mux.HandleFunc("POST /api/auth/change-password", s.handleAuthChangePassword)
	s.mux.HandleFunc("GET /api/auth/sessions", s.handleAuthSessions)
	s.mux.HandleFunc("POST /api/auth/sessions/revoke-others", s.handleAuthRevokeOtherSessions)

	// System
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/system/info", s.handleSystemInfo)

	// Settings & portable config backup
	s.mux.HandleFunc("GET /api/settings", s.handleGetSettings)
	s.mux.HandleFunc("PUT /api/settings", s.handleUpdateSettings)
	s.mux.HandleFunc("POST /api/settings/test-webhook", s.handleTestWebhook)
	s.mux.HandleFunc("GET /api/config/export", s.handleExportConfig)
	s.mux.HandleFunc("POST /api/config/import", s.handleImportConfig)
	s.mux.HandleFunc("POST /api/config/export-full", s.handleExportFullConfig)
	s.mux.HandleFunc("POST /api/config/import-full", s.handleImportFullConfig)

	// Remotes
	s.mux.HandleFunc("GET /api/remotes/providers", s.handleListProviders)
	s.mux.HandleFunc("GET /api/remotes", s.handleListRemotes)
	s.mux.HandleFunc("POST /api/remotes", s.handleCreateRemote)
	s.mux.HandleFunc("POST /api/remotes/test-draft", s.handleTestDraftRemote)
	s.mux.HandleFunc("PUT /api/remotes/{name}", s.handleUpdateRemote)
	s.mux.HandleFunc("DELETE /api/remotes/{name}", s.handleDeleteRemote)
	s.mux.HandleFunc("POST /api/remotes/import-conf", s.handleImportConf)
	s.mux.HandleFunc("POST /api/remotes/{name}/test", s.handleTestRemote)
	s.mux.HandleFunc("GET /api/remotes/{name}/about", s.handleAboutRemote)
	s.mux.HandleFunc("POST /api/remotes/{name}/cleanup", s.handleCleanupRemote)
	s.mux.HandleFunc("POST /api/remotes/{name}/dedupe", s.handleDedupeRemote)

	// Jobs & Cron
	s.mux.HandleFunc("GET /api/jobs", s.handleListJobs)
	s.mux.HandleFunc("POST /api/jobs", s.handleCreateJob)
	s.mux.HandleFunc("POST /api/jobs/preview-command", s.handlePreviewJobCommand)
	s.mux.HandleFunc("POST /api/jobs/dry-run", s.handleDryRunDraftJob)
	s.mux.HandleFunc("GET /api/jobs/{id}", s.handleGetJob)
	s.mux.HandleFunc("PUT /api/jobs/{id}", s.handleUpdateJob)
	s.mux.HandleFunc("DELETE /api/jobs/{id}", s.handleDeleteJob)
	s.mux.HandleFunc("POST /api/jobs/{id}/clone", s.handleCloneJob)
	s.mux.HandleFunc("POST /api/jobs/{id}/toggle", s.handleToggleJob)
	s.mux.HandleFunc("POST /api/jobs/{id}/run", s.handleRunJob)
	s.mux.HandleFunc("POST /api/jobs/{id}/stop", s.handleStopJob)
	s.mux.HandleFunc("POST /api/cron/preview", s.handleCronPreview)

	// Runs & Logs
	s.mux.HandleFunc("GET /api/runs", s.handleListRuns)
	s.mux.HandleFunc("GET /api/runs/stats", s.handleRunStats)
	s.mux.HandleFunc("DELETE /api/runs", s.handleClearRuns)
	s.mux.HandleFunc("GET /api/runs/{id}", s.handleGetRun)
	s.mux.HandleFunc("GET /api/runs/{id}/logs", s.handleGetRunLogs)
	s.mux.HandleFunc("DELETE /api/runs/{id}", s.handleDeleteRun)

	// File Browser & Restore
	s.mux.HandleFunc("GET /api/browse", s.handleBrowse)
	s.mux.HandleFunc("GET /api/browse/local", s.handleBrowseLocal)
	s.mux.HandleFunc("GET /api/browse/size", s.handleBrowseSize)
	s.mux.HandleFunc("POST /api/browse/mkdir", s.handleBrowseMkdir)
	s.mux.HandleFunc("POST /api/browse/upload", s.handleBrowseUpload)
	s.mux.HandleFunc("POST /api/browse/upload-batch", s.handleBrowseUploadBatch)
	s.mux.HandleFunc("POST /api/browse/rename", s.handleBrowseRename)
	s.mux.HandleFunc("DELETE /api/browse/entry", s.handleBrowseDelete)
	s.mux.HandleFunc("POST /api/browse/bulk-delete", s.handleBrowseBulkDelete)
	s.mux.HandleFunc("GET /api/browse/download", s.handleBrowseDownload)
	s.mux.HandleFunc("GET /api/browse/versions", s.handleBrowseVersions)
	s.mux.HandleFunc("POST /api/browse/restore", s.handleBrowseRestore)

	// SSE Live Progress
	s.mux.HandleFunc("GET /api/events", s.handleSSE)

	// Catch-all for unmatched /api/* and SPA fallback
	s.mux.HandleFunc("/", s.handleSPA)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

func (sr *statusRecorder) Unwrap() http.ResponseWriter {
	return sr.ResponseWriter
}

func (sr *statusRecorder) Flush() {
	if f, ok := sr.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *Server) withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		defer func() {
			if recVal := recover(); recVal != nil {
				s.logger.Error("http handler panic",
					"panic", recVal,
					"method", r.Method,
					"path", r.URL.Path,
					"stack", string(debug.Stack()),
				)
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()

		// Baseline browser security headers.
		rec.Header().Set("X-Content-Type-Options", "nosniff")
		rec.Header().Set("X-Frame-Options", "DENY")
		rec.Header().Set("Referrer-Policy", "no-referrer")

		// Apply CORS only when explicitly configured via RCLONE_MANAGER_CORS_ORIGINS.
		if origin := r.Header.Get("Origin"); origin != "" {
			if allowed, allowCreds := s.matchAllowedOrigin(origin); allowed {
				rec.Header().Set("Access-Control-Allow-Origin", origin)
				rec.Header().Set("Vary", "Origin")
				rec.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				rec.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				if allowCreds {
					rec.Header().Set("Access-Control-Allow-Credentials", "true")
				}
			}
		}

		if r.Method == http.MethodOptions {
			rec.WriteHeader(http.StatusNoContent)
			return
		}

		// Cross-site request forgery protection for state-changing browser API requests.
		cleanedPath := path.Clean("/" + r.URL.Path)
		if (cleanedPath == "/api" || strings.HasPrefix(cleanedPath, "/api/")) && s.csrf != nil {
			if err := s.csrf.Check(r); err != nil {
				writeError(rec, http.StatusForbidden, "cross-origin request blocked")
				return
			}
		}

		// Mandatory session cookie enforcement for all protected /api/* routes.
		if !isPublicPath(r.URL.Path) {
			if s.auth == nil {
				writeError(rec, http.StatusUnauthorized, "authentication required")
				return
			}
			user, tokenHash, err := s.auth.Authenticate(r.Context(), sessionTokenFromRequest(r))
			if err != nil {
				writeError(rec, http.StatusUnauthorized, "authentication required")
				return
			}
			r = r.WithContext(withAuthContext(r.Context(), authContext{
				User:      *user,
				TokenHash: tokenHash,
			}))
		}

		next.ServeHTTP(rec, r)

		if strings.HasPrefix(r.URL.Path, "/api/") && r.URL.Path != "/api/health" && r.URL.Path != "/api/events" {
			s.logger.Info("http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"duration_ms", time.Since(start).Milliseconds(),
			)
		}
	})
}

func (s *Server) matchAllowedOrigin(origin string) (allowed bool, allowCreds bool) {
	for _, entry := range s.cfg.CORSOrigins {
		if entry == "*" {
			return true, false
		}
		if strings.EqualFold(entry, origin) {
			return true, true
		}
	}
	return false, false
}

func isPublicPath(urlPath string) bool {
	cleaned := path.Clean("/" + urlPath)
	if cleaned != "/api" && !strings.HasPrefix(cleaned, "/api/") {
		return true
	}
	switch cleaned {
	case "/api/health",
		"/api/auth/status",
		"/api/auth/setup",
		"/api/auth/login",
		"/api/auth/logout":
		return true
	default:
		return false
	}
}

func (s *Server) handleSPA(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeError(w, http.StatusNotFound, "API endpoint not found")
		return
	}
	if s.spaFS == nil {
		http.NotFound(w, r)
		return
	}

	reqPath := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if reqPath == "" {
		reqPath = "index.html"
	}

	if serveEmbeddedFile(w, r, s.spaFS, reqPath) {
		return
	}
	if !serveEmbeddedFile(w, r, s.spaFS, "index.html") {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("rclone-manager API is running. Build frontend in web/ to serve UI."))
	}
}

func serveEmbeddedFile(w http.ResponseWriter, r *http.Request, fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)
	if err != nil || info.IsDir() {
		return false
	}
	f, err := fsys.Open(name)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()

	rs, ok := f.(io.ReadSeeker)
	if !ok {
		return false
	}

	setCacheHeaders(w, name)
	http.ServeContent(w, r, name, info.ModTime(), rs)
	return true
}

func setCacheHeaders(w http.ResponseWriter, name string) {
	switch {
	case strings.HasPrefix(name, "assets/"):
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	case name == "index.html" || name == "sw.js" || name == "manifest.webmanifest":
		w.Header().Set("Cache-Control", "no-cache")
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (s *Server) writeInternalError(w http.ResponseWriter, r *http.Request, msg string, err error) {
	s.logger.Error(msg, "method", r.Method, "path", r.URL.Path, "error", err)
	writeError(w, http.StatusInternalServerError, "internal server error")
}

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 4*1024*1024))
	return dec.Decode(dst)
}
