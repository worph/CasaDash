package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Server) requireApps(w http.ResponseWriter) bool {
	if s.apps == nil {
		writeJSON(w, http.StatusServiceUnavailable,
			map[string]string{"error": "docker unavailable"})
		return false
	}
	return true
}

func (s *Server) handleListApps(w http.ResponseWriter, r *http.Request) {
	if !s.requireApps(w) {
		return
	}
	list, err := s.apps.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleAppAction(w http.ResponseWriter, r *http.Request) {
	if !s.requireApps(w) {
		return
	}
	id := chi.URLParam(r, "id")
	action := chi.URLParam(r, "action")

	var err error
	switch action {
	case "start":
		err = s.apps.Start(r.Context(), id)
	case "stop":
		err = s.apps.Stop(r.Context(), id)
	case "restart":
		err = s.apps.Restart(r.Context(), id)
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown action"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleUninstallApp(w http.ResponseWriter, r *http.Request) {
	if !s.requireApps(w) {
		return
	}
	id := chi.URLParam(r, "id")
	archive := r.URL.Query().Get("archive") == "true"
	archiveName, err := s.apps.Uninstall(r.Context(), id, archive)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.broadcastApps()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "archive": archiveName})
}
