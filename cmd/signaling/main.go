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

	"github.com/pj-hoakari/tolo-signaling/internal/config"
	"github.com/pj-hoakari/tolo-signaling/internal/connectserver"
	firestorerepo "github.com/pj-hoakari/tolo-signaling/internal/firestore"
	"github.com/pj-hoakari/tolo-signaling/internal/signaling"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("signaling: %v", err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()

	// TODO: 認証機構繋ぎ込み
	if cfg.Firestore.EmulatorHost != "" && os.Getenv("FIRESTORE_EMULATOR_HOST") == "" {
		if err := os.Setenv("FIRESTORE_EMULATOR_HOST", cfg.Firestore.EmulatorHost); err != nil {
			return err
		}
	}

	store, err := firestorerepo.New(context.Background(), cfg.Firestore.ProjectID)
	if err != nil {
		return err
	}
	defer store.Close()

	svc := signaling.NewService(store)
	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           connectserver.NewH2CHandler(connectserver.NewHandler(svc)),
		ReadHeaderTimeout: 10 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		log.Printf("signaling server listening on %s (firestore project=%s emulator=%q)",
			cfg.Addr, cfg.Firestore.ProjectID, os.Getenv("FIRESTORE_EMULATOR_HOST"))
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
		log.Print("signaling server shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	}
}
