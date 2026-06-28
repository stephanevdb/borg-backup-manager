package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/borg-backup-manager/backend/internal/api"
	"github.com/borg-backup-manager/backend/internal/auth"
	"github.com/borg-backup-manager/backend/internal/backup"
	"github.com/borg-backup-manager/backend/internal/config"
	"github.com/borg-backup-manager/backend/internal/crypto"
	"github.com/borg-backup-manager/backend/internal/db"
	"github.com/borg-backup-manager/backend/internal/health"
	"github.com/borg-backup-manager/backend/internal/notify"
	"github.com/borg-backup-manager/backend/internal/scheduler"
	"github.com/borg-backup-manager/backend/internal/stream"
	"github.com/joho/godotenv"
)

//go:embed static/*
var staticFS embed.FS

func main() {
	_ = godotenv.Load()
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	_ = os.MkdirAll(cfg.DataDir, 0o755)

	store, err := db.Open(cfg.DatabaseURI)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	enc, err := crypto.NewEncryptor(cfg.SecretKey)
	if err != nil {
		log.Fatal(err)
	}

	authSvc, err := auth.NewService(cfg, store)
	if err != nil {
		log.Fatalf("auth init: %v", err)
	}

	hub := stream.NewHub()
	notifier := notify.New(store)
	engine := backup.NewEngine(cfg, store, enc, hub, notifier)
	sched := scheduler.New(cfg, store, engine)
	sched.SetNotifier(notifier)
	healthSvc := health.New(cfg, store, engine, sched)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	engine.StartWorker(ctx)
	sched.Start(ctx)
	defer sched.Stop()

	var staticHandler http.Handler
	if sub, err := fs.Sub(staticFS, "static"); err == nil {
		if entries, _ := fs.ReadDir(sub, "."); len(entries) > 0 {
			staticHandler = http.FileServer(http.FS(sub))
		}
	}

	srv := api.NewServer(cfg, store, authSvc, engine, sched, healthSvc, notifier, hub, enc, staticHandler)
	httpServer := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: srv.Router(),
	}

	go func() {
		log.Printf("Borg Backup Manager listening on :%s", cfg.Port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = httpServer.Shutdown(shutdownCtx)
}
