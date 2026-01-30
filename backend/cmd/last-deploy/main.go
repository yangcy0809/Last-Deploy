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

	"last-deploy/internal/api"
	"last-deploy/internal/config"
	"last-deploy/internal/jobs"
	"last-deploy/internal/store"
	"last-deploy/internal/workspace"
)

func main() {
	cfg := config.Load()

	if err := workspace.EnsureDataDirs(cfg); err != nil {
		log.Fatalf("init data dirs: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(ctx, cfg.DBPath())
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer func() {
		_ = st.Close()
	}()

	queue := jobs.NewQueue(128)
	if err := jobs.EnqueuePersisted(ctx, st, queue); err != nil {
		log.Printf("enqueue persisted jobs: %v", err)
	}

	worker := jobs.NewWorker(st, queue, cfg)
	go worker.Run(ctx)

	r := api.NewRouter(st, queue, cfg)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Printf("listening on http://%s", cfg.Addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("listen: %v", err)
	}
}
