package server

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/yundera/casadash/internal/apps"
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
	list := s.listApps(r.Context())
	if list == nil {
		list = []apps.App{}
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

// handleCheckUpdate reports whether the app's reference store carries a newer
// docker-compose.yml than the installed copy (see installer.CheckUpdate).
func (s *Server) handleCheckUpdate(w http.ResponseWriter, r *http.Request) {
	if !s.requireApps(w) {
		return
	}
	if s.installer == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "store unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	st, err := s.installer.CheckUpdate(ctx, chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// handleApplyUpdate copies the store's current compose over the app's strict base
// (when it differs) and brings the stack back up. The tile shows a "…" overlay
// while it runs.
func (s *Server) handleApplyUpdate(w http.ResponseWriter, r *http.Request) {
	if !s.requireApps(w) {
		return
	}
	if s.installer == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "store unavailable"})
		return
	}
	id := chi.URLParam(r, "id")
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	var updated bool
	err := s.apps.WithBusy(id, func() error {
		var e error
		updated, e = s.installer.ApplyUpdate(ctx, id)
		return e
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.broadcastApps()
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "updated": updated})
}

func (s *Server) handleUninstallApp(w http.ResponseWriter, r *http.Request) {
	if !s.requireApps(w) {
		return
	}
	id := chi.URLParam(r, "id")
	// zip=true compresses the archived folder; otherwise it is a plain rename.
	// Either way the app's data is preserved (docs/app-model.md).
	zip := r.URL.Query().Get("zip") == "true"
	archiveName, err := s.apps.Uninstall(r.Context(), id, zip)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// Drop any lingering install-progress overlay (e.g. a failed install) so the
	// tile disappears with the archived app instead of ghosting as an error.
	if s.installer != nil {
		s.installer.ClearInstall(id)
	}
	s.broadcastApps()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "archive": archiveName})
}
