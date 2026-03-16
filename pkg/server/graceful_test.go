package server

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

type stopCloserStub struct {
	stopped bool
	closed  bool
}

func (s *stopCloserStub) Stop(ctx context.Context) error {
	s.stopped = true
	return nil
}

func (s *stopCloserStub) Close() error {
	s.closed = true
	return nil
}

type ioCloserStub struct {
	closed bool
}

func (s *ioCloserStub) Close() error {
	s.closed = true
	return nil
}

func TestGracefulShutdownStopsResources(t *testing.T) {
	origWait := waitForShutdownSignal
	origShutdown := shutdownEchoServer
	t.Cleanup(func() {
		waitForShutdownSignal = origWait
		shutdownEchoServer = origShutdown
	})

	waitForShutdownSignal = func(quit <-chan os.Signal) {}
	shutdownEchoServer = func(e *echo.Echo, ctx context.Context) error { return nil }

	stopStub := &stopCloserStub{}
	closeStub := &ioCloserStub{}

	GracefulShutdown(echo.New(), time.Second, func() {}, stopStub, closeStub, struct{}{})

	if !stopStub.stopped {
		t.Fatal("Stop() was not called")
	}
	if !closeStub.closed {
		t.Fatal("Close() was not called")
	}
}

type stopErrorStub struct{}

func (s *stopErrorStub) Stop(ctx context.Context) error { return errors.New("stop failed") }

type closeErrorStub struct{}

func (s *closeErrorStub) Close() error { return errors.New("close failed") }

func TestShutdownResourcesHandlesErrorsAndUnknownClosers(t *testing.T) {
	t.Parallel()

	shutdownResources(context.Background(), &stopErrorStub{}, &closeErrorStub{}, struct{}{}, nil)
}

func TestGracefulShutdownHandlesEchoShutdownError(t *testing.T) {
	origWait := waitForShutdownSignal
	origShutdown := shutdownEchoServer
	t.Cleanup(func() {
		waitForShutdownSignal = origWait
		shutdownEchoServer = origShutdown
	})

	waitForShutdownSignal = func(quit <-chan os.Signal) {}
	shutdownEchoServer = func(e *echo.Echo, ctx context.Context) error { return errors.New("shutdown failed") }

	GracefulShutdown(echo.New(), time.Second, nil)
}
