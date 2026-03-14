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
	slog.InfoContext(ctx, "configuration loaded",
		"http_address", cfg.Server.Address,
		"kafka_topic", cfg.Kafka.Consumer.Topic,
		"kafka_group", cfg.Kafka.Consumer.GroupID,
	)

	// 2. Сборка всего приложения (DI контейнер)
	app, err := bootstrap.BuildApp(ctx, cfg)
	if err != nil {
		slog.ErrorContext(ctx, "failed to build application", "err", err)
		os.Exit(1)
	}

	go func() {
		slog.InfoContext(ctx, "starting http server", "address", cfg.Server.Address)

		app.Echo.Server.ReadTimeout = cfg.Server.ReadTimeout
		app.Echo.Server.WriteTimeout = cfg.Server.WriteTimeout
		app.Echo.Server.IdleTimeout = cfg.Server.IdleTimeout

		if err := app.Echo.Start(cfg.Server.Address); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.ErrorContext(ctx, "http server crashed", "err", err)
		}
	}()

	go func() {
		slog.InfoContext(ctx, "starting kafka consumer")
		app.Consumer.Start(ctx)
	}()

	slog.InfoContext(ctx, "application successfully started and running")

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
