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

	"lumescope/internal/background"
	"lumescope/internal/config"
	"lumescope/internal/db"
	lclient "lumescope/internal/lumera"
	"lumescope/internal/server"
)

func main() {
	cfg := config.Load()

	// Init DB
	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DB_DSN, cfg.DB_MaxConns)
	if err != nil {
		log.Fatalf("db connect failed: %v", err)
	}
	if err := db.Bootstrap(ctx, pool); err != nil {
		log.Fatalf("db bootstrap failed: %v", err)
	}

	// Lumera client
	lc := lclient.NewClient(cfg.LumeraAPIBase, cfg.HTTPTimeout)

	// Start background workers
	bgCtx, bgCancel := context.WithCancel(context.Background())
	runner := background.NewRunner(cfg, pool, lc)
	runner.Start(bgCtx)

	r := server.NewRouter(cfg)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	go func() {
		log.Printf("LumeScope API starting on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	bgCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown error: %v", err)
	}
	db.Close(pool)
	log.Printf("LumeScope API stopped")
}
