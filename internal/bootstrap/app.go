package bootstrap

import (
	"context"
	"log/slog"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/meindokuse/transaction-module/internal/adapters/consumer"
	"github.com/meindokuse/transaction-module/internal/adapters/postgres"
	adapterRedis "github.com/meindokuse/transaction-module/internal/adapters/redis"
	"github.com/meindokuse/transaction-module/internal/config"
	"github.com/meindokuse/transaction-module/internal/controlers/dlq"
	"github.com/meindokuse/transaction-module/internal/controlers/httpdelivery"
	"github.com/meindokuse/transaction-module/internal/usecase/transaction"
	pkgPostgres "github.com/meindokuse/transaction-module/pkg/connect/postgres"
	pkgRedis "github.com/meindokuse/transaction-module/pkg/connect/redis"
	"github.com/meindokuse/transaction-module/pkg/validate"
)

var (
	newPgPool             = pkgPostgres.NewPgPool
	newRedisClient        = pkgRedis.NewRedisClient
	newTransactionRepo    = postgres.NewTransactionRepository
	newBloomFilterRepo    = adapterRedis.NewBloomFilterRepository
	newTransactionUseCase = transaction.NewTransaction
	newTransactionHandler = httpdelivery.NewTransactionHandler
	newDLQHandler         = dlq.NewDLQHandler
	newConsumer           = consumer.NewConsumer
	newValidator          = validate.Get
)

type App struct {
	Echo   *echo.Echo
	Config *config.Config

	PgPool   *pkgPostgres.PgPoolWrapper
	Redis    *pkgRedis.RedisClientWrapper
	Consumer *consumer.Consumer
	DLQ      *dlq.DLQHandler
}

func BuildApp(ctx context.Context, cfg *config.Config) (*App, error) {
	app := &App{
		Config: cfg,
		Echo:   echo.New(),
	}

	slog.InfoContext(ctx, "initializing infrastructure")

	pgWrapper, err := newPgPool(ctx, cfg.DB)
	if err != nil {
		return nil, err
	}
	app.PgPool = pgWrapper
	slog.InfoContext(ctx, "postgres connection initialized")

	redisWrapper, err := newRedisClient(ctx, cfg.Redis)
	if err != nil {
		return nil, err
	}
	app.Redis = redisWrapper
	slog.InfoContext(ctx, "redis connection initialized")

	slog.InfoContext(ctx, "wiring dependencies")

	dbRepo := newTransactionRepo(pgWrapper.Pool)
	redisRepo := newBloomFilterRepo(redisWrapper.Client)
	txUseCase := newTransactionUseCase(dbRepo, redisRepo)
	txHandler := newTransactionHandler(txUseCase)

	app.setupRouter(txHandler)

	slog.InfoContext(ctx, "initializing message broker")

	dlqHandler := newDLQHandler(cfg.Kafka.Producer)
	app.DLQ = dlqHandler

	txConsumer := newConsumer(
		cfg.Kafka.Consumer,
		txUseCase.ProcessBatch,
		dlqHandler,
		newValidator(),
	)
	app.Consumer = txConsumer

	slog.InfoContext(ctx, "application dependencies initialized")

	return app, nil
}

func (a *App) setupRouter(txHandler *httpdelivery.TransactionHandler) {
	a.Echo.Use(middleware.Recover())
	a.Echo.Use(middleware.RequestLogger())
	a.Echo.Use(middleware.CORS())

	a.Echo.Validator = httpdelivery.NewEchoValidator(newValidator())

	api := a.Echo.Group("/api/v1")
	api.GET("/transactions", txHandler.GetAllTransactions)
	api.GET("/users/:user_id/transactions", txHandler.GetUserTransactions)
}
