package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/kevindiu/rclone-manager/internal/domain"
	"github.com/kevindiu/rclone-manager/internal/rclone"
	"github.com/kevindiu/rclone-manager/internal/service"
)

func maskSettingsForResponse(settings domain.GlobalSettings) domain.GlobalSettings {
	if settings.TelegramBotToken != "" {
		settings.TelegramBotToken = rclone.MaskedSecret
	}
	return settings
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.svc.GetSettings(r.Context())
	if err != nil {
		s.writeInternalError(w, r, "get settings failed", err)
		return
	}
	writeJSON(w, http.StatusOK, maskSettingsForResponse(settings))
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var incoming domain.GlobalSettings
	if err := decodeJSON(r, &incoming); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload: "+err.Error())
		return
	}

	updated, err := s.svc.UpdateSettings(r.Context(), incoming)
	if errors.Is(err, service.ErrValidation) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		s.writeInternalError(w, r, "update settings failed", err)
		return
	}
	writeJSON(w, http.StatusOK, maskSettingsForResponse(updated))
}

func (s *Server) handleTestWebhook(w http.ResponseWriter, r *http.Request) {
	var body struct {
		WebhookURL       string `json:"webhook_url"`
		WebhookType      string `json:"webhook_type"`
		TelegramBotToken string `json:"telegram_bot_token"`
		TelegramChatID   string `json:"telegram_chat_id"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload: "+err.Error())
		return
	}
	if err := s.svc.TestWebhookSettings(r.Context(), body.WebhookURL, body.WebhookType, body.TelegramBotToken, body.TelegramChatID); errors.Is(err, service.ErrValidation) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	} else if err != nil {
		writeError(w, http.StatusBadGateway, "webhook test failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

func (s *Server) handleExportConfig(w http.ResponseWriter, r *http.Request) {
	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if format == "" {
		format = "json"
	}

	bundle, err := s.svc.ExportBundle(r.Context())
	if err != nil {
		s.writeInternalError(w, r, "export bundle failed", err)
		return
	}

	if format == "yaml" || format == "yml" {
		data, err := yaml.Marshal(bundle)
		if err != nil {
			s.writeInternalError(w, r, "marshal yaml export failed", err)
			return
		}
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="rclone-manager-backup.yaml"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
		return
	}

	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		s.writeInternalError(w, r, "marshal json export failed", err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="rclone-manager-backup.json"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) handleImportConfig(w http.ResponseWriter, r *http.Request) {
	mode := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("mode")))
	if mode == "" {
		mode = "merge"
	}

	raw, err := io.ReadAll(io.LimitReader(r.Body, 8*1024*1024))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		writeError(w, http.StatusBadRequest, "empty import payload")
		return
	}

	var bundle domain.ExportBundle
	if strings.HasPrefix(trimmed, "{") {
		if err := json.Unmarshal([]byte(trimmed), &bundle); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON bundle: "+err.Error())
			return
		}
	} else {
		if err := yaml.Unmarshal([]byte(trimmed), &bundle); err != nil {
			writeError(w, http.StatusBadRequest, "invalid YAML bundle: "+err.Error())
			return
		}
	}

	if err := s.svc.ImportBundle(r.Context(), bundle, mode); errors.Is(err, service.ErrValidation) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	} else if err != nil {
		s.writeInternalError(w, r, "import bundle failed", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":        "imported",
		"mode":          mode,
		"jobs_imported": len(bundle.Jobs),
	})
}

func (s *Server) handleExportFullConfig(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Passphrase string `json:"passphrase"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload: "+err.Error())
		return
	}

	data, err := s.svc.ExportEncryptedBackup(r.Context(), body.Passphrase)
	if errors.Is(err, service.ErrValidation) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		s.writeInternalError(w, r, "export full encrypted backup failed", err)
		return
	}

	filename := fmt.Sprintf("rclone-manager-backup-%s.rcmbak", time.Now().UTC().Format("20060102-150405"))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) handleImportFullConfig(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 16*1024*1024))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var body struct {
		Content    string `json:"content"`
		Passphrase string `json:"passphrase"`
		Mode       string `json:"mode"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload: "+err.Error())
		return
	}
	mode := strings.ToLower(strings.TrimSpace(body.Mode))
	if mode == "" {
		mode = "merge"
	}

	res, err := s.svc.ImportEncryptedBackup(r.Context(), []byte(body.Content), body.Passphrase, mode)
	if errors.Is(err, service.ErrValidation) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		s.writeInternalError(w, r, "import full encrypted backup failed", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "imported",
		"mode":             mode,
		"jobs_imported":    res.JobsImported,
		"remotes_imported": res.RemotesImported,
		"remote_names":     res.RemoteNames,
	})
}
