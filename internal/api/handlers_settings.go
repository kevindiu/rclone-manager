package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/kevindiu/rclone-manager/internal/domain"
	"github.com/kevindiu/rclone-manager/internal/service"
)

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.svc.GetSettings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, settings)
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
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleExportConfig(w http.ResponseWriter, r *http.Request) {
	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if format == "" {
		format = "json"
	}

	bundle, err := s.svc.ExportBundle(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if format == "yaml" || format == "yml" {
		data, err := yaml.Marshal(bundle)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
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
		writeError(w, http.StatusInternalServerError, err.Error())
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
		var wrapped struct {
			Mode   string               `json:"mode"`
			Bundle *domain.ExportBundle `json:"bundle"`
		}
		if err := json.Unmarshal([]byte(trimmed), &wrapped); err == nil && wrapped.Bundle != nil {
			bundle = *wrapped.Bundle
			if wrapped.Mode != "" {
				mode = wrapped.Mode
			}
		} else if err := json.Unmarshal([]byte(trimmed), &bundle); err != nil {
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
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":        "imported",
		"mode":          mode,
		"jobs_imported": len(bundle.Jobs),
	})
}
