package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"wlgposter/internal/app"
	"wlgposter/internal/config"
	"wlgposter/internal/observability"
)

func main() {
	cfg := config.MustLoad()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger, shutdown, err := observability.Init(ctx, cfg)
	if err != nil {
		logger.Warn().Err(err).Msg("Observability init degraded")
	}

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), observability.ShutdownTimeout())
		defer cancel()

		if err := shutdown(shutdownCtx); err != nil {
			logger.Error().Err(err).Msg("Observability shutdown failed")
		}
	}()

	logger.Info().Msg("Starting...")

	application := app.New(ctx, cfg, logger)
	application.Run(ctx)
}
