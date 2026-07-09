// Command casadash is a lightweight, dashboard-only reimagining of CasaOS.
// It serves the embedded Svelte UI plus a REST + WebSocket API and drives the
// host Docker engine over the mounted socket.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yundera/casadash/internal/config"
	"github.com/yundera/casadash/internal/server"
	"github.com/yundera/casadash/internal/ui"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("casadash: ")

	cfg := config.FromEnv()

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           server.New(cfg, ui.Dist()),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("listening on %s (data root %s)", cfg.Addr, cfg.DataRoot)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Print("shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
