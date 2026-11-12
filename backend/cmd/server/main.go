// Command server runs the HTTP API.
//
// Configuration (environment variables, Cloud Run style):
//
//	PORT                   listen port (default 8080; Cloud Run sets it)
//	CATMOD_TEMPLATES       game files dir (default ../templates, the repo's `make fetch` output)
//	ALLOWED_ORIGINS        comma-separated frontend origins for CORS (default http://localhost:4321)
//	MAX_UPLOAD_MB          largest photo accepted (default 15)
//	MAX_CONCURRENT_BUILDS  builds running at once (default 2)
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/KJulo/Deadlock-picture-mod/backend/internal/api"
	"github.com/KJulo/Deadlock-picture-mod/backend/internal/build"
	"github.com/KJulo/Deadlock-picture-mod/backend/internal/photo"
	"github.com/KJulo/Deadlock-picture-mod/backend/internal/templates"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(log *slog.Logger, key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		log.Error("invalid setting, using default", "key", key, "value", v, "default", fallback)
		return fallback
	}
	return n
}

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil)) // JSON lines: Cloud Logging parses them
	slog.SetDefault(log)

	tplDir := templates.Dir("../templates")
	tpl, err := build.LoadTemplates(tplDir)
	if err != nil {
		log.Error("cannot load game files", "dir", tplDir, "err", err)
		os.Exit(1)
	}
	cfg := api.Config{
		MaxUploadBytes: int64(envInt(log, "MAX_UPLOAD_MB", 15)) << 20,
		MaxConcurrent:  envInt(log, "MAX_CONCURRENT_BUILDS", 2),
		QueueTimeout:   20 * time.Second,
		AllowedOrigins: strings.Split(env("ALLOWED_ORIGINS", "http://localhost:4321"), ","),
		Logger:         log,
	}
	srv, err := api.New(cfg, func(data []byte, p photo.Params) (*build.Result, error) {
		return build.Mod(tpl, data, p)
	})
	if err != nil {
		log.Error("cannot create server", "err", err)
		os.Exit(1)
	}

	httpServer := &http.Server{
		Addr:              ":" + env("PORT", "8080"),
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,  // slowloris protection
		ReadTimeout:       60 * time.Second,  // slow mobile uploads
		WriteTimeout:      90 * time.Second,  // queue wait + build + 14 MB download
		IdleTimeout:       120 * time.Second, // keep-alive
	}

	// Cloud Run sends SIGTERM before stopping an instance: finish in-flight requests, then exit.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		log.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		httpServer.Shutdown(shutdownCtx)
	}()

	log.Info("listening", "addr", httpServer.Addr, "origins", cfg.AllowedOrigins,
		"max_upload_mb", cfg.MaxUploadBytes>>20, "max_concurrent", cfg.MaxConcurrent)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("server error", "err", err)
		os.Exit(1)
	}
}
