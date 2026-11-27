package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kevindiu/rclone-manager/internal/api"
	"github.com/kevindiu/rclone-manager/internal/config"
	"github.com/kevindiu/rclone-manager/internal/db"
	"github.com/kevindiu/rclone-manager/internal/events"
	"github.com/kevindiu/rclone-manager/internal/rclone"
	"github.com/kevindiu/rclone-manager/internal/runner"
	"github.com/kevindiu/rclone-manager/internal/scheduler"
	"github.com/kevindiu/rclone-manager/internal/service"
	"github.com/kevindiu/rclone-manager/web"
)

var version = "dev"

func newLogger(levelStr, formatStr string) *slog.Logger {
	level := slog.LevelInfo
	switch levelStr {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	opts := &slog.HandlerOptions{Level: level}
	if formatStr == "json" {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config failed", "error", err)
		os.Exit(1)
	}

	logger := newLogger(cfg.LogLevel, cfg.LogFormat)
	slog.SetDefault(logger)

	store, err := db.Open(cfg.DBPath)
	if err != nil {
		logger.Error("open database failed", "path", cfg.DBPath, "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := store.Close(); err != nil {
			logger.Warn("close database warning", "error", err)
		}
	}()

	ctx := context.Background()
	if recovered, err := store.MarkInterruptedRunsFailed(ctx); err != nil {
		logger.Warn("failed to mark interrupted runs", "error", err)
	} else if recovered > 0 {
		logger.Info("recovered interrupted job runs from previous session", "count", recovered)
	}

	settings, err := store.GetSettings(ctx, cfg.Defaults)
	if err != nil {
		logger.Error("load settings failed", "error", err)
		os.Exit(1)
	}
	if pruned, err := store.PruneOldRuns(ctx, settings.LogRetentionDays); err != nil {
		logger.Warn("initial log pruning failed", "error", err)
	} else if pruned > 0 {
		logger.Info("pruned expired job run logs", "count", pruned)
	}

	client := rclone.NewClient(cfg.RcloneBin, cfg.RcloneConfig)
	if rver, _, err := client.Version(ctx); err != nil {
		logger.Warn("rclone binary check failed", "bin", cfg.RcloneBin, "error", err)
	} else {
		logger.Info("rclone initialized", "version", rver, "config", cfg.RcloneConfig)
	}

	hub := events.NewHub()
	runEngine := runner.NewRunner(store, client, hub, cfg.LogDir, cfg.Defaults, logger)
	sched := scheduler.New(store, runEngine, cfg.Defaults, logger)
	if err := sched.Start(); err != nil {
		logger.Error("start scheduler failed", "error", err)
		os.Exit(1)
	}

	svc := service.New(store, runEngine, sched, hub, client, cfg.Defaults, logger)
	authSvc := service.NewAuthService(store, cfg.SessionTTL, logger)
	if err := authSvc.SeedAdminFromEnv(ctx, cfg.AdminUser, cfg.AdminPassword); err != nil {
		logger.Error("seed admin user failed", "error", err)
		os.Exit(1)
	}
	if prunedSessions, err := authSvc.PruneExpiredSessions(ctx); err != nil {
		logger.Warn("initial expired session pruning failed", "error", err)
	} else if prunedSessions > 0 {
		logger.Info("pruned expired sessions", "count", prunedSessions)
	}

	apiSrv := api.NewServer(cfg, svc, authSvc, client, runEngine, hub, web.DistFS, version, logger)
	httpSrv := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           apiSrv.Handler(),
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("server listening", "version", version, "addr", "http://"+cfg.Addr())
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server error", "error", err)
			os.Exit(1)
		}
	}()

	<-sigCtx.Done()
	logger.Info("shutting down rclone-manager")

	sched.Stop()
	runEngine.Shutdown()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		logger.Warn("http shutdown warning", "error", err)
	}
	logger.Info("shutdown complete")
}
