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

var (
	loadConfig       = config.Load
	buildApplication = bootstrap.BuildApp
	startHTTPServer  = func(ctx context.Context, app *bootstrap.App, cfg *config.Config) {
		slog.InfoContext(ctx, "starting http server", "address", cfg.Server.Address)

		app.Echo.Server.ReadTimeout = cfg.Server.ReadTimeout
		app.Echo.Server.WriteTimeout = cfg.Server.WriteTimeout
		app.Echo.Server.IdleTimeout = cfg.Server.IdleTimeout

		if err := app.Echo.Start(cfg.Server.Address); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.ErrorContext(ctx, "http server crashed", "err", err)
		}
	}
	startConsumerLoop = func(ctx context.Context, app *bootstrap.App) {
		slog.InfoContext(ctx, "starting kafka consumer")
		app.Consumer.Start(ctx)
	}
	gracefulShutdown = server.GracefulShutdown
	exitProcess      = os.Exit
)

func main() {
	cfg := loadConfig()
	setupLogger()

	ctx, cancel := context.WithCancel(context.Background())
	slog.InfoContext(ctx, "configuration loaded",
		"http_address", cfg.Server.Address,
		"kafka_topic", cfg.Kafka.Consumer.Topic,
		"kafka_group", cfg.Kafka.Consumer.GroupID,
	)

	app, err := buildApplication(ctx, cfg)
	if err != nil {
		slog.ErrorContext(ctx, "failed to build application", "err", err)
		exitProcess(1)
	}

	go func() {
		startHTTPServer(ctx, app, cfg)
	}()

	go func() {
		startConsumerLoop(ctx, app)
	}()

	slog.InfoContext(ctx, "application successfully started and running")

	gracefulShutdown(
		app.Echo,
		15*time.Second,
		cancel,

		app.Consumer,
		app.DLQ,
		app.PgPool,
		app.Redis,
	)
}

// setupLogger configures the process-wide structured logger.
func setupLogger() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
}
