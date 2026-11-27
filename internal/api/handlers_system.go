package api

import (
	"net/http"
	"runtime"
	"time"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	rcloneVer, _, err := s.client.Version(r.Context())
	status := "ok"
	if err != nil {
		status = "degraded"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":         status,
		"rclone_version": rcloneVer,
		"app_version":    s.appVersion,
	})
}

func (s *Server) handleSystemInfo(w http.ResponseWriter, r *http.Request) {
	rcloneVer, osArch, _ := s.client.Version(r.Context())
	settings, err := s.svc.GetSettings(r.Context())
	if err != nil {
		settings = s.cfg.Defaults
	}

	writeJSON(w, http.StatusOK, SystemInfo{
		AppVersion:    s.appVersion,
		RcloneVersion: rcloneVer,
		RcloneOSArch:  osArch,
		GoVersion:     runtime.Version(),
		DataDir:       s.cfg.DataDir,
		RcloneConfig:  s.cfg.RcloneConfig,
		Timezone:      settings.Timezone,
		UptimeSeconds: int64(time.Since(s.startedAt).Seconds()),
	})
}
