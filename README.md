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
KAFKA_BROKERS=localhost:9092
DB_DSN=postgres://casino:secretpassword@localhost:5432/casino_transactions?sslmode=disable&pool_max_conns=50
HTTP_ADDRESS=:8081
REDIS_PASSWORD=redis_pass
```

## Запуск

1. Поднять PostgreSQL, Kafka и Redis.
2. Применить миграции из директории `migrations/`.
3. Запустить сервис:

```bash
go run ./cmd/server
```

## Docker

Сервис можно поднять целиком через Docker Compose:

```bash
docker compose up --build
```

Что поднимется:

- `postgres`
- `redis` на базе `redis-stack-server`, чтобы работали Bloom-команды
- `kafka` в KRaft-режиме
- `kafka-init` для создания основных топиков
- `migrate` для применения SQL-миграций
- `app`

HTTP API после старта будет доступен на `http://localhost:8080`.

Для контейнерного запуска используется отдельный конфиг:

- `config/config.docker.yaml`

Сборка контейнера вручную:

```bash
docker build -t casino-transaction-module .
docker run --rm -p 8080:8080 casino-transaction-module
```

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

## CI

В проект добавлен GitHub Actions pipeline:

- `.github/workflows/ci.yml`

Pipeline делает следующее:

- поднимает PostgreSQL service для integration-теста репозитория;
- запускает `go test ./...`;
- запускает `go build ./...`;
- проверяет, что Docker-образ собирается.

## Файлы с тестами

- `internal/dto/pagination_test.go`
- `internal/usecase/transaction/transaction_test.go`
- `internal/controlers/httpdelivery/tx_handlers_test.go`
- `internal/adapters/consumer/consumer_test.go`
- `internal/adapters/postgres/transaction_integration_test.go`
