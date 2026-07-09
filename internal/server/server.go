// Package server wires the HTTP router: REST API, WebSocket hub, and the
// embedded single-page app.
package server

import (
	"context"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/yundera/casadash/internal/apps"
	"github.com/yundera/casadash/internal/appstore"
	"github.com/yundera/casadash/internal/config"
	"github.com/yundera/casadash/internal/dockerx"
	"github.com/yundera/casadash/internal/installer"
	"github.com/yundera/casadash/internal/live"
	"github.com/yundera/casadash/internal/system"
	"github.com/yundera/casadash/internal/usersettings"
)

// Server holds shared dependencies for the HTTP handlers.
type Server struct {
	cfg       config.Config
	uiFS      fs.FS
	collector *system.Collector
	hub       *live.Hub
	dx        *dockerx.Client
	apps      *apps.Registry
	store     *appstore.Manager
	installer *installer.Installer
	settings  *usersettings.Store
}

// New builds the root HTTP handler. A nil-Docker environment still serves the
// dashboard (apps endpoints report the connection error).
func New(cfg config.Config, uiFS fs.FS) http.Handler {
	collector := system.NewCollector(cfg.DataRoot)
	s := &Server{
		cfg:       cfg,
		uiFS:      uiFS,
		collector: collector,
		hub:       live.NewHub(collector),
		settings:  usersettings.New(filepath.Join(cfg.StateDir(), "settings.json")),
	}

	if dx, err := dockerx.New(); err != nil {
		log.Printf("docker: %v (app management disabled)", err)
	} else {
		if err := dx.Ping(context.Background()); err != nil {
			log.Printf("docker: cannot reach daemon: %v", err)
		}
		s.dx = dx
		s.apps = apps.New(cfg, dx)
		s.hub.AppsSnapshot = s.appsSnapshot
		s.watchDocker()
	}

	// App store + installer (independent of Docker connectivity for browsing).
	// Persisted store sources (if any) take precedence over the env default.
	initialURLs := cfg.StoreURLs
	if ss := s.settings.Get().StoreSources; len(ss) > 0 {
		initialURLs = ss
	}
	s.store = appstore.New(initialURLs, filepath.Join(cfg.StateDir(), "appstore"))
	s.store.StartAutoRefresh(context.Background(), time.Hour)
	s.installer = installer.New(cfg, s.store, s.dx)

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)

	r.Get("/ping", s.handlePing)
	r.Get("/ws", s.hub.ServeWS)

	r.Route("/api", func(r chi.Router) {
		r.Get("/system/stats", s.handleSystemStats)
		r.Get("/apps", s.handleListApps)
		r.Get("/apps/{id}/config", s.handleGetConfig)
		r.Put("/apps/{id}/config", s.handlePutConfig)
		r.Get("/apps/{id}/logs", s.handleAppLogs)
		r.Get("/apps/{id}/stats", s.handleAppStats)
		r.Post("/apps/{id}/{action}", s.handleAppAction)
		r.Delete("/apps/{id}", s.handleUninstallApp)

		r.Get("/store", s.handleStore)
		r.Get("/store/sources", s.handleStoreSources)
		r.Post("/store/sources", s.handleAddStoreSource)
		r.Delete("/store/sources", s.handleRemoveStoreSource)
		r.Get("/store/app/{id}", s.handleStoreApp)
		r.Post("/store/{id}/install", s.handleInstall)
		r.Get("/store/{id}/install/ws", s.handleInstallWS)

		r.Get("/settings", s.handleGetSettings)
		r.Put("/settings", s.handlePutSettings)
	})

	r.Handle("/*", spaHandler(uiFS))
	return r
}

func (s *Server) handlePing(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleSystemStats(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.collector.Sample())
}

// appsSnapshot returns the current app list for the live "apps" channel.
func (s *Server) appsSnapshot() any {
	if s.apps == nil {
		return []any{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	list, err := s.apps.List(ctx)
	if err != nil {
		return []any{}
	}
	return list
}

// broadcastApps pushes the current app list to "apps" channel subscribers.
func (s *Server) broadcastApps() {
	s.hub.Broadcast(live.ChannelApps, s.appsSnapshot())
}

// watchDocker rebroadcasts the app list on container events, debounced.
func (s *Server) watchDocker() {
	go func() {
		trigger := make(chan struct{}, 1)
		go s.dx.WatchContainers(context.Background(), func() {
			select {
			case trigger <- struct{}{}:
			default:
			}
		})
		for range trigger {
			time.Sleep(400 * time.Millisecond) // debounce bursts
			s.hub.Broadcast(live.ChannelApps, s.appsSnapshot())
		}
	}()
}
