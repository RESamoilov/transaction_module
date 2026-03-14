# Casino Transaction Module

Go-сервис для приема событий ставок и выигрышей через Kafka, сохранения транзакций в PostgreSQL и выдачи истории транзакций через HTTP API.

## Что делает сервис

- читает события транзакций из Kafka;
- валидирует сообщения и отправляет невалидные события в DLQ;
- сохраняет транзакции в PostgreSQL;
- отдает историю транзакций по пользователю и по всей системе;
- поддерживает фильтрацию по `transaction_type` и cursor-based pagination.

## Стек

- Go
- Kafka
- PostgreSQL
- Redis
- Echo

## Структура события Kafka

```json
{
  "id": "tx-1",
  "user_id": "user-1",
  "transaction_type": "bet",
  "amount": 100.5,
  "timestamp": "2026-03-14T12:00:00Z"
}
```

Допустимые значения `transaction_type`: `bet`, `win`.

## HTTP API

### Получить все транзакции

`GET /api/v1/transactions`

Query-параметры:

- `transaction_type` — `bet` или `win`
- `limit` — от `1` до `100`
- `cursor` — курсор следующей страницы

Пример:

```http
GET /api/v1/transactions?transaction_type=bet&limit=20
```

### Получить транзакции пользователя

`GET /api/v1/users/:user_id/transactions`

Query-параметры:

- `transaction_type` — `bet` или `win`
- `limit` — от `1` до `100`
- `cursor` — курсор следующей страницы

Пример:

```http
GET /api/v1/users/user-1/transactions?transaction_type=win&limit=10
```

### Формат ответа

```json
{
  "data": [
    {
      "id": "tx-1",
      "user_id": "user-1",
      "amount": 100.5,
      "transaction_type": "bet",
      "timestamp": "2026-03-14T12:00:00Z"
    }
  ],
  "meta": {
    "has_more": false
  }
}
```

## Конфигурация

Основные переменные окружения:

- `CONFIG_PATH`
- `KAFKA_BROKERS`
- `DB_DSN`
- `HTTP_ADDRESS`
- `REDIS_PASSWORD`

Пример `.env`:

```env
CONFIG_PATH=config/config.yaml
HTTP_ADDRESS=:8080
POSTGRES_USER=casino
POSTGRES_PASSWORD=secretpassword
POSTGRES_DB=casino_transactions
REDIS_PASSWORD=redis_pass
DB_DSN=postgres://casino:secretpassword@postgres:5432/casino_transactions?sslmode=disable
REDIS_ADDR=redis:6379
KAFKA_BROKERS=kafka:9092
```

## Запуск

Для локальной разработки сервис запускается через Docker Compose. Это основной и поддерживаемый сценарий запуска.

Поднять весь стек:

```bash
docker compose up --build
```

Запуск в фоне:

```bash
docker compose up -d --build
```

Что поднимется:

- `postgres`
- `redis` на базе `redis-stack-server`, чтобы работали Bloom-команды
- `kafka` в KRaft-режиме
- `kafka-init` для создания основных топиков
- `migrate` для применения SQL-миграций
- `app`

HTTP API после старта будет доступен на `http://localhost:8080`.

Остановить стек:

```bash
docker compose down
```

Остановить стек и удалить volumes:

```bash
docker compose down -v
```

## Нагрузочный тест

Для быстрой локальной проверки потока сообщений можно использовать встроенный генератор событий:

- `tools/loadtester`

Запуск:

```bash
go run ./tools/loadtester/main
```

После старта loadtester поднимет HTTP-сервер на `http://localhost:8081`.

Чтобы запустить отправку сообщений в Kafka, отправь `POST` запрос:

```bash
curl -X POST http://localhost:8081/load
```

Что делает loadtester:

- в течение `5` секунд отправляет сообщения в Kafka topic `casino.transactions.v1`
- генерирует валидные события с полями `id`, `user_id`, `transaction_type`, `amount`, `timestamp`
- случайно распределяет тип транзакции между `bet` и `win`

После этого можно проверить:

- HTTP API на `http://localhost:8080/api/v1/transactions`
- данные в PostgreSQL через `docker compose exec postgres psql -U casino -d casino_transactions`

## Тесты

Запуск всех доступных тестов:

```bash
go test ./...
```

Запуск с coverage:

```bash
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out
```

## Integration tests

Интеграционный тест репозитория использует PostgreSQL из `DB_DSN`.

- если `DB_DSN` не задан, тест будет `skip`;
- если PostgreSQL недоступен, тест будет `skip`;
- тест создает временную схему и удаляет ее после завершения.

## Файлы с тестами

- `internal/dto/pagination_test.go`
- `internal/usecase/transaction/transaction_test.go`
- `internal/controlers/httpdelivery/tx_handlers_test.go`
- `internal/adapters/consumer/consumer_test.go`
- `internal/adapters/postgres/transaction_integration_test.go`
