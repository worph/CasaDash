package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"

	"github.com/yundera/casadash/internal/appstore"
	"github.com/yundera/casadash/internal/installer"
)

type storeResponse struct {
	Apps       []*appstore.CatalogApp `json:"apps"`
	Categories []string               `json:"categories"`
	Recommend  []string               `json:"recommend"`
}

func (s *Server) handleStore(w http.ResponseWriter, _ *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "store unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, storeResponse{
		Apps:       s.store.Catalog(),
		Categories: s.store.Categories(),
		Recommend:  s.store.Recommend(),
	})
}

// --- App-store source management (add/remove custom stores) ---

func (s *Server) handleStoreSources(w http.ResponseWriter, _ *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "store unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string][]string{"sources": s.store.URLs()})
}

func (s *Server) handleAddStoreSource(w http.ResponseWriter, r *http.Request) {
	url, ok := decodeURL(w, r)
	if !ok {
		return
	}
	urls := s.store.URLs()
	for _, u := range urls {
		if u == url {
			s.applySources(w, r.Context(), urls) // already present
			return
		}
	}
	s.applySources(w, r.Context(), append(urls, url))
}

func (s *Server) handleRemoveStoreSource(w http.ResponseWriter, r *http.Request) {
	url, ok := decodeURL(w, r)
	if !ok {
		return
	}
	var kept []string
	for _, u := range s.store.URLs() {
		if u != url {
			kept = append(kept, u)
		}
	}
	s.applySources(w, r.Context(), kept)
}

func decodeURL(w http.ResponseWriter, r *http.Request) (string, bool) {
	var body struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.URL) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "url required"})
		return "", false
	}
	return strings.TrimSpace(body.URL), true
}

// applySources updates the store URLs, persists them, refreshes the catalog, and
// returns the new source list.
func (s *Server) applySources(w http.ResponseWriter, ctx context.Context, urls []string) {
	s.store.SetURLs(urls)
	cur := s.settings.Get()
	cur.StoreSources = urls
	_ = s.settings.Set(cur)

	rc, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	_ = s.store.Refresh(rc)

	writeJSON(w, http.StatusOK, map[string][]string{"sources": s.store.URLs()})
}

func (s *Server) handleStoreApp(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "store unavailable"})
		return
	}
	app, _, err := s.store.Get(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, app)
}

// handleInstall runs a blocking install (no progress) — kept for simple clients.
func (s *Server) handleInstall(w http.ResponseWriter, r *http.Request) {
	if s.installer == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "install unavailable"})
		return
	}
	id := chi.URLParam(r, "id")
	if err := s.installer.Install(r.Context(), id, nil); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.broadcastApps()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleInstallWS runs an install over a WebSocket, streaming progress events.
func (s *Server) handleInstallWS(w http.ResponseWriter, r *http.Request) {
	if s.installer == nil {
		http.Error(w, "install unavailable", http.StatusServiceUnavailable)
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{"*"}})
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
	ctx := conn.CloseRead(r.Context())

	send := func(ev installer.Event) {
		if raw, err := json.Marshal(ev); err == nil {
			_ = conn.Write(ctx, websocket.MessageText, raw)
		}
	}

	if err := s.installer.Install(ctx, chi.URLParam(r, "id"), send); err != nil {
		send(installer.Event{Phase: "error", Message: err.Error()})
	}
	s.broadcastApps()
}
