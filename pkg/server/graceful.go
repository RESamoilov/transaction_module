package server

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
)

var (
	waitForShutdownSignal = func(quit <-chan os.Signal) {
		<-quit
	}
	shutdownEchoServer = func(e *echo.Echo, ctx context.Context) error {
		return e.Shutdown(ctx)
	}
)

func GracefulShutdown(e *echo.Echo, timeout time.Duration, cancel context.CancelFunc, closers ...interface{}) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	waitForShutdownSignal(quit)
	slog.Info("Interrupt signal received, starting graceful shutdown...")

	if cancel != nil {
		cancel()
	}

	ctx, shutdownCancel := context.WithTimeout(context.Background(), timeout)
	defer shutdownCancel()

	if err := shutdownEchoServer(e, ctx); err != nil {
		slog.Error("Echo server forced to shutdown", "err", err)
	}

	shutdownResources(ctx, closers...)

	slog.Info("Server exited gracefully")
}

func shutdownResources(ctx context.Context, closers ...interface{}) {
	for _, item := range closers {
		if item == nil {
			continue
		}

		switch c := item.(type) {
		case interface{ Stop(context.Context) error }:
			slog.Info("Stopping resource...", "type", fmt.Sprintf("%T", c))
			if err := c.Stop(ctx); err != nil {
				slog.Error("Error stopping resource", "err", err)
			}
		case io.Closer:
			slog.Info("Closing resource...", "type", fmt.Sprintf("%T", c))
			if err := c.Close(); err != nil {
				slog.Error("Error closing resource", "err", err)
			}
		default:
			slog.Warn("Unknown closer type, skipping", "type", fmt.Sprintf("%T", c))
		}
	}
}
