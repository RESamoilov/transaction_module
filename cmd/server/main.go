package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/meindokuse/transaction-module/internal/bootstrap"
	"github.com/meindokuse/transaction-module/internal/config"
	"github.com/meindokuse/transaction-module/pkg/server"
)

func main() {
	cfg := config.Load()
	setupLogger()

	ctx, cancel := context.WithCancel(context.Background())

	// 2. Сборка всего приложения (DI контейнер)
	app, err := bootstrap.BuildApp(ctx, cfg)
	if err != nil {
		slog.Error("Failed to build application", "err", err)
		os.Exit(1)
	}

	go func() {
		slog.Info("Starting HTTP server", "address", cfg.Server.Address)

		app.Echo.Server.ReadTimeout = cfg.Server.ReadTimeout
		app.Echo.Server.WriteTimeout = cfg.Server.WriteTimeout
		app.Echo.Server.IdleTimeout = cfg.Server.IdleTimeout

		if err := app.Echo.Start(cfg.Server.Address); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("HTTP server crashed", "err", err)
		}
	}()

	go func() {
		slog.Info("Starting Kafka consumer...")
		app.Consumer.Start(ctx)
	}()

	slog.Info("Application successfully started and running")

	server.GracefulShutdown(
		app.Echo,
		15*time.Second,
		cancel,

		app.Consumer,
		app.DLQ,
		app.PgPool,
		app.Redis,
	)
}

func setupLogger() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
}
