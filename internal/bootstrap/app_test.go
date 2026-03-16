package bootstrap

import (
	"context"
	"errors"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/meindokuse/transaction-module/internal/adapters/consumer"
	"github.com/meindokuse/transaction-module/internal/config"
	"github.com/meindokuse/transaction-module/internal/controlers/dlq"
	"github.com/meindokuse/transaction-module/internal/controlers/httpdelivery"
	pkgPostgres "github.com/meindokuse/transaction-module/pkg/connect/postgres"
	pkgRedis "github.com/meindokuse/transaction-module/pkg/connect/redis"
)

func TestBuildAppReturnsPostgresError(t *testing.T) {
	origPg := newPgPool
	t.Cleanup(func() { newPgPool = origPg })

	newPgPool = func(ctx context.Context, cfg pkgPostgres.Config) (*pkgPostgres.PgPoolWrapper, error) {
		return nil, errors.New("pg failed")
	}

	_, err := BuildApp(context.Background(), &config.Config{})
	if err == nil {
		t.Fatal("BuildApp() error = nil, want postgres error")
	}
}

func TestBuildAppSuccessWithInjectedDependencies(t *testing.T) {
	origPg, origRedis := newPgPool, newRedisClient
	origTxRepo, origBloom := newTransactionRepo, newBloomFilterRepo
	origUC, origHandler := newTransactionUseCase, newTransactionHandler
	origDLQ, origConsumer, origValidator := newDLQHandler, newConsumer, newValidator
	t.Cleanup(func() {
		newPgPool = origPg
		newRedisClient = origRedis
		newTransactionRepo = origTxRepo
		newBloomFilterRepo = origBloom
		newTransactionUseCase = origUC
		newTransactionHandler = origHandler
		newDLQHandler = origDLQ
		newConsumer = origConsumer
		newValidator = origValidator
	})

	newPgPool = func(ctx context.Context, cfg pkgPostgres.Config) (*pkgPostgres.PgPoolWrapper, error) {
		return &pkgPostgres.PgPoolWrapper{}, nil
	}
	newRedisClient = func(ctx context.Context, cfg pkgRedis.Config) (*pkgRedis.RedisClientWrapper, error) {
		return &pkgRedis.RedisClientWrapper{}, nil
	}
	newDLQHandler = func(cfg config.ProducerConfig) *dlq.DLQHandler {
		return &dlq.DLQHandler{}
	}
	newConsumer = func(cfg config.ConsumerConfig, handler consumer.BatchHandler, errHandler consumer.ErrorHandler, validate *validator.Validate) *consumer.Consumer {
		return &consumer.Consumer{}
	}
	newValidator = func() *validator.Validate { return validator.New() }

	app, err := BuildApp(context.Background(), &config.Config{})
	if err != nil {
		t.Fatalf("BuildApp() error = %v", err)
	}
	if app == nil || app.Echo == nil {
		t.Fatal("BuildApp() returned nil app")
	}
	if len(app.Echo.Routes()) < 2 {
		t.Fatalf("routes = %d, want at least 2", len(app.Echo.Routes()))
	}
}

func TestSetupRouterConfiguresEcho(t *testing.T) {
	t.Parallel()

	app := &App{Echo: echo.New()}
	app.setupRouter(&httpdelivery.TransactionHandler{})

	if app.Echo.Validator == nil {
		t.Fatal("Echo validator was not configured")
	}
	if len(app.Echo.Routes()) < 2 {
		t.Fatalf("routes = %d, want at least 2", len(app.Echo.Routes()))
	}
}
