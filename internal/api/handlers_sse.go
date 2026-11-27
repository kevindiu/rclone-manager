package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	rawToken := sessionTokenFromRequest(r)
	rc := http.NewResponseController(w)
	// Disable HTTP write timeout for long-lived SSE streams.
	_ = rc.SetWriteDeadline(time.Time{})

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch, cleanup := s.hub.Subscribe()
	defer cleanup()

	writeEvent := func(evType string, data any) error {
		payload, err := json.Marshal(data)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", evType, payload); err != nil {
			return err
		}
		return rc.Flush()
	}

	// Send initial active runs snapshot immediately upon connecting.
	if err := writeEvent("snapshot", map[string]any{
		"active_runs": s.runner.GetActiveSnapshots(),
	}); err != nil {
		return
	}

	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case ev, open := <-ch:
			if !open {
				return
			}
			if err := writeEvent(ev.Type, ev.Data); err != nil {
				return
			}
		case <-heartbeat.C:
			if s.auth != nil {
				if _, _, err := s.auth.Authenticate(r.Context(), rawToken); err != nil {
					return
				}
			}
			if _, err := fmt.Fprint(w, ": heartbeat\n\n"); err != nil {
				return
			}
			if err := rc.Flush(); err != nil {
				return
			}
		}
	}
}
