package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/khorihutama/hsi-go-api/internal/api"
	"github.com/khorihutama/hsi-go-api/internal/config"
	"github.com/khorihutama/hsi-go-api/internal/di"
	"github.com/khorihutama/hsi-go-api/internal/infra/database"
	"github.com/khorihutama/hsi-go-api/internal/infra/logging"
	redisinfra "github.com/khorihutama/hsi-go-api/internal/infra/redis"
	"github.com/khorihutama/hsi-go-api/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	logger := logging.New(cfg.AppEnv)

	ctx, cancelApp := context.WithCancel(context.Background())
	defer cancelApp()
	db, err := database.Connect(cfg.DSN(), cfg.IsProduction())
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	rdb, err := redisinfra.Connect(ctx, cfg.RedisAddr)
	if err != nil {
		logger.Error("redis connection failed", "error", err)
		os.Exit(1)
	}
	defer rdb.Close()

	container, err := di.Build(cfg, logger, db, rdb)
	if err != nil {
		logger.Error("dependency setup failed", "error", err)
		os.Exit(1)
	}
	container.BookingSvc.StartSweeper(ctx)
	router := api.NewRouter(cfg, logger, container)
	srv := server.New(":"+cfg.RestPort, router)

	go func() {
		logger.Info("http server listening", "addr", srv.Addr, "env", cfg.AppEnv)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server stopped", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	cancelApp()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
	logger.Info("server stopped")
}
