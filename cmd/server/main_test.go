package main

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/meindokuse/transaction-module/internal/adapters/consumer"
	"github.com/meindokuse/transaction-module/internal/bootstrap"
	"github.com/meindokuse/transaction-module/internal/config"
	"github.com/meindokuse/transaction-module/internal/domain"
)

func TestSetupLoggerSetsDefaultLogger(t *testing.T) {
	t.Parallel()

	setupLogger()
	if got := slog.Default(); got == nil {
		t.Fatal("slog.Default() = nil, want logger")
	}
}

func TestMainExitsWhenBuildFails(t *testing.T) {
	origLoadConfig, origBuildApp := loadConfig, buildApplication
	origExit, origStartHTTP, origStartConsumer, origShutdown := exitProcess, startHTTPServer, startConsumerLoop, gracefulShutdown
	t.Cleanup(func() {
		loadConfig = origLoadConfig
		buildApplication = origBuildApp
		exitProcess = origExit
		startHTTPServer = origStartHTTP
		startConsumerLoop = origStartConsumer
		gracefulShutdown = origShutdown
	})

	loadConfig = func() *config.Config { return &config.Config{} }
	buildApplication = func(ctx context.Context, cfg *config.Config) (*bootstrap.App, error) {
		return nil, errors.New("boom")
	}
	startHTTPServer = func(ctx context.Context, app *bootstrap.App, cfg *config.Config) {}
	startConsumerLoop = func(ctx context.Context, app *bootstrap.App) {}
	gracefulShutdown = func(e *echo.Echo, timeout time.Duration, cancel context.CancelFunc, closers ...interface{}) {}

	called := false
	exitProcess = func(code int) {
		called = true
		panic(code)
	}

	defer func() {
		if recover() == nil {
			t.Fatal("main() did not exit")
		}
		if !called {
			t.Fatal("exitProcess was not called")
		}
	}()

	main()
}

func TestMainRunsStartupHooks(t *testing.T) {
	origLoadConfig, origBuildApp := loadConfig, buildApplication
	origExit, origStartHTTP, origStartConsumer, origShutdown := exitProcess, startHTTPServer, startConsumerLoop, gracefulShutdown
	t.Cleanup(func() {
		loadConfig = origLoadConfig
		buildApplication = origBuildApp
		exitProcess = origExit
		startHTTPServer = origStartHTTP
		startConsumerLoop = origStartConsumer
		gracefulShutdown = origShutdown
	})

	loadConfig = func() *config.Config {
		return &config.Config{Server: config.HTTPServer{Address: ":8080"}}
	}
	buildApplication = func(ctx context.Context, cfg *config.Config) (*bootstrap.App, error) {
		return &bootstrap.App{Echo: echo.New(), Consumer: &consumer.Consumer{}}, nil
	}

	httpStarted := make(chan struct{}, 1)
	consumerStarted := make(chan struct{}, 1)
	shutdownCalled := make(chan struct{}, 1)

	startHTTPServer = func(ctx context.Context, app *bootstrap.App, cfg *config.Config) { httpStarted <- struct{}{} }
	startConsumerLoop = func(ctx context.Context, app *bootstrap.App) { consumerStarted <- struct{}{} }
	gracefulShutdown = func(e *echo.Echo, timeout time.Duration, cancel context.CancelFunc, closers ...interface{}) {
		shutdownCalled <- struct{}{}
	}
	exitProcess = func(code int) { t.Fatalf("unexpected exit: %d", code) }

	main()

	select {
	case <-httpStarted:
	case <-time.After(time.Second):
		t.Fatal("HTTP startup hook was not called")
	}
	select {
	case <-consumerStarted:
	case <-time.After(time.Second):
		t.Fatal("consumer startup hook was not called")
	}
	select {
	case <-shutdownCalled:
	case <-time.After(time.Second):
		t.Fatal("graceful shutdown was not called")
	}
}

func TestStartHTTPServerAppliesTimeouts(t *testing.T) {
	t.Parallel()

	app := &bootstrap.App{Echo: echo.New()}
	cfg := &config.Config{
		Server: config.HTTPServer{
			Address:      "127.0.0.1:0",
			ReadTimeout:  time.Second,
			WriteTimeout: 2 * time.Second,
			IdleTimeout:  3 * time.Second,
		},
	}

	done := make(chan struct{})
	go func() {
		startHTTPServer(context.Background(), app, cfg)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)

	if app.Echo.Server.ReadTimeout != time.Second {
		t.Fatalf("ReadTimeout = %v, want %v", app.Echo.Server.ReadTimeout, time.Second)
	}
	if app.Echo.Server.WriteTimeout != 2*time.Second {
		t.Fatalf("WriteTimeout = %v, want %v", app.Echo.Server.WriteTimeout, 2*time.Second)
	}
	if app.Echo.Server.IdleTimeout != 3*time.Second {
		t.Fatalf("IdleTimeout = %v, want %v", app.Echo.Server.IdleTimeout, 3*time.Second)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := app.Echo.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("startHTTPServer() did not return after shutdown")
	}
}

func TestStartConsumerLoopReturnsOnCanceledContext(t *testing.T) {
	t.Parallel()

	cfg := config.ConsumerConfig{
		Brokers:        []string{"127.0.0.1:9092"},
		Topic:          "casino.transactions.v1",
		GroupID:        "test-group",
		WorkersCount:   1,
		BatchSize:      1,
		BatchTimeout:   time.Millisecond,
		RetryMaxDelay:  time.Millisecond,
		MaxRetries:     1,
		MaxDlqRetries:  1,
		MinBytes:       1,
		MaxBytes:       10,
		MaxWait:        time.Millisecond,
		ReadBackoffMin: time.Millisecond,
		ReadBackoffMax: time.Millisecond,
	}

	realConsumer := consumer.NewConsumer(cfg, func(ctx context.Context, txs []*domain.Transaction) error {
		return nil
	}, nil, validator.New())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		startConsumerLoop(ctx, &bootstrap.App{Consumer: realConsumer})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("startConsumerLoop() did not return on canceled context")
	}
}
