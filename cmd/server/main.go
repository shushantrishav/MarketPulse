package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"marketpulse/internal/api"
	"marketpulse/internal/config"
	"marketpulse/internal/domain/service"
	"marketpulse/internal/infra/feed"
	"marketpulse/internal/infra/redis"
)

func main() {
	cfg := config.Load()

	logger, _ := zap.NewProduction()
	defer logger.Sync()

	redisClient := redis.NewClient(cfg)
	feedClient := feed.NewClient(cfg)

	stateRepo := redis.NewStateRepository(redisClient, cfg.MaxSymbols)
	intradaySvc := service.NewIntradayService(stateRepo, feedClient)

	router := api.NewRouter(cfg, logger, intradaySvc)

	srv := &http.Server{
		Addr:    cfg.HTTPPort,
		Handler: router,
	}

	go func() {
		logger.Info("server started", zap.String("addr", cfg.HTTPPort))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("server failed", zap.Error(err))
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	logger.Info("shutdown requested")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", zap.Error(err))
	} else {
		logger.Info("shutdown complete")
	}
}
