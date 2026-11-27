package api

import (
	"errors"
	"net/http"

	"github.com/kevindiu/rclone-manager/internal/rclone"
	"github.com/kevindiu/rclone-manager/internal/service"
)

func (s *Server) handleListProviders(w http.ResponseWriter, r *http.Request) {
	provs, err := s.client.Providers(r.Context())
	if err != nil {
		s.writeInternalError(w, r, "list providers failed", err)
		return
	}
	writeJSON(w, http.StatusOK, provs)
}

func (s *Server) handleListRemotes(w http.ResponseWriter, r *http.Request) {
	remotes, err := s.client.ListRemotes(r.Context())
	if err != nil {
		s.writeInternalError(w, r, "list remotes failed", err)
		return
	}
	writeJSON(w, http.StatusOK, remotes)
}

func (s *Server) handleCreateRemote(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   string            `json:"name"`
		Type   string            `json:"type"`
		Params map[string]string `json:"params"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload: "+err.Error())
		return
	}
	if err := s.client.CreateRemote(r.Context(), req.Name, req.Type, req.Params); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, rclone.ErrRemoteExists) {
			status = http.StatusConflict
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"name": req.Name, "status": "created"})
}

func (s *Server) handleUpdateRemote(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req struct {
		Params map[string]string `json:"params"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload: "+err.Error())
		return
	}
	if err := s.client.UpdateRemote(r.Context(), name, req.Params); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, rclone.ErrRemoteNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"name": name, "status": "updated"})
}

func (s *Server) handleDeleteRemote(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.svc.DeleteRemote(r.Context(), name); err != nil {
		status := http.StatusBadRequest
		switch {
		case errors.Is(err, service.ErrConflict):
			status = http.StatusConflict
		case errors.Is(err, rclone.ErrRemoteNotFound):
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"name": name, "status": "deleted"})
}

func (s *Server) handleImportConf(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Content string `json:"content"`
		Mode    string `json:"mode"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload: "+err.Error())
		return
	}
	if req.Mode == "" {
		req.Mode = "merge"
	}
	imported, err := s.client.ImportConf(r.Context(), req.Content, req.Mode)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "imported",
		"mode":     req.Mode,
		"imported": imported,
	})
}

func (s *Server) handleTestRemote(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	res, err := s.client.TestRemote(r.Context(), name)
	if err != nil {
		if errors.Is(err, rclone.ErrRemoteNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		s.writeInternalError(w, r, "test remote failed", err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleAboutRemote(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	res, err := s.client.AboutRemote(r.Context(), name)
	if err != nil {
		if errors.Is(err, rclone.ErrRemoteNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		s.writeInternalError(w, r, "about remote failed", err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleCleanupRemote(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.client.CleanupRemote(r.Context(), name); err != nil {
		if errors.Is(err, rclone.ErrRemoteNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"name": name, "status": "cleaned"})
}
