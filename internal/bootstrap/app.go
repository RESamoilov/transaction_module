package bootstrap

import (
	"context"
	"log/slog"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	// Адаптеры (Доставка и Инфраструктура)
	"github.com/meindokuse/transaction-module/internal/adapters/consumer"
	"github.com/meindokuse/transaction-module/internal/adapters/postgres"
	adapterRedis "github.com/meindokuse/transaction-module/internal/adapters/redis"

	// Контроллеры (Delivery)
	"github.com/meindokuse/transaction-module/internal/controlers/dlq"
	"github.com/meindokuse/transaction-module/internal/controlers/httpdelivery"

	// Остальное
	"github.com/meindokuse/transaction-module/internal/config"
	"github.com/meindokuse/transaction-module/internal/usecase/transaction"

	// Пакеты инфраструктурных подключений
	pkgPostgres "github.com/meindokuse/transaction-module/pkg/connect/postgres"
	pkgRedis "github.com/meindokuse/transaction-module/pkg/connect/redis"
	"github.com/meindokuse/transaction-module/pkg/validate"
)

// App содержит все зависимости и запущенные компоненты сервера
type App struct {
	Echo   *echo.Echo
	Config *config.Config

	// Ресурсы, которые нужно будет закрыть при Graceful Shutdown
	PgPool   *pkgPostgres.PgPoolWrapper
	Redis    *pkgRedis.RedisClientWrapper
	Consumer *consumer.Consumer
	DLQ      *dlq.DLQHandler
}

// BuildApp - это наш DI контейнер
func BuildApp(ctx context.Context, cfg *config.Config) (*App, error) {
	app := &App{
		Config: cfg,
		Echo:   echo.New(),
	}

	slog.Info("Initializing infrastructure...")

	// 1. Инициализация PostgreSQL (Инфраструктура)
	pgWrapper, err := pkgPostgres.NewPgPool(ctx, cfg.DB)
	if err != nil {
		return nil, err
	}
	app.PgPool = pgWrapper // Сохраняем для Shutdown

	// 2. Инициализация Redis (Инфраструктура)
	redisWrapper, err := pkgRedis.NewRedisClient(ctx, cfg.Redis)
	if err != nil {
		return nil, err
	}
	app.Redis = redisWrapper // Сохраняем для Shutdown

	slog.Info("Wiring dependencies...")

	// 3. Инициализация Репозиториев (Data Layer)
	dbRepo := postgres.NewTransactionRepository(pgWrapper.Pool)
	redisRepo := adapterRedis.NewBloomFilterRepository(redisWrapper.Client)

	// 4. Инициализация UseCase (Domain/Business Layer)
	txUseCase := transaction.NewTransaction(dbRepo, redisRepo)

	// 5. Инициализация HTTP Хендлера (REST API Delivery)
	txHandler := httpdelivery.NewTransactionHandler(txUseCase)

	// 6. Настройка HTTP роутера
	app.setupRouter(txHandler)

	slog.Info("Initializing Message Broker (Kafka)...")

	// 7. Инициализация Kafka DLQ Handler
	dlqHandler := dlq.NewDLQHandler(cfg.Kafka.Producer)
	app.DLQ = dlqHandler

	// 8. Инициализация Kafka Consumer
	txConsumer := consumer.NewConsumer(
		cfg.Kafka.Consumer,
		txUseCase.ProcessBatch,
		dlqHandler,
		validate.Get(),
	)
	app.Consumer = txConsumer

	return app, nil
}

func (a *App) setupRouter(txHandler *httpdelivery.TransactionHandler) {
	// Подключаем базовые middleware
	a.Echo.Use(middleware.Recover())
	a.Echo.Use(middleware.RequestLogger())
	a.Echo.Use(middleware.CORS())

	// Настраиваем валидатор для DTO
	a.Echo.Validator = httpdelivery.NewEchoValidator(validate.Get())

	// Регистрация маршрутов
	api := a.Echo.Group("/api/v1")
	api.GET("/transactions", txHandler.GetAllTransactions)
	api.GET("/users/:user_id/transactions", txHandler.GetUserTransactions)
}
