package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/yundera/casadash/internal/domains"
	"github.com/yundera/casadash/internal/usersettings"
)

func (s *Server) handleGetSettings(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.settings.Get())
}

func (s *Server) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	var in usersettings.Settings
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	if err := s.settings.Set(in); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, s.settings.Get())
}

// handlePutDomains replaces the additional domains apps are published on, then
// republishes the running ones onto the new list.
//
// Domains get their own endpoint instead of riding the generic settings PUT
// because that one auto-saves on a keystroke debounce, and this one recreates
// every container on the box. The republish runs in the background: it is a
// compose up per app, the tiles carry their own busy state, and the operator
// shouldn't be staring at a hung request while it happens.
func (s *Server) handlePutDomains(w http.ResponseWriter, r *http.Request) {
	var in []domains.Domain
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	for i := range in {
		in[i].Name = strings.TrimSpace(in[i].Name)
		in[i].Domain = strings.TrimSpace(in[i].Domain)
		if in[i].Name == "" || in[i].Domain == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "each domain needs a name and a domain"})
			return
		}
	}

	cur := s.settings.Get()
	cur.Domains = in
	if err := s.settings.Set(cur); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if s.apps != nil {
		go s.apps.Republish(context.Background())
	}
	writeJSON(w, http.StatusOK, s.settings.Get().Domains)
}
