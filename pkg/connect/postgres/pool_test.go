package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPgPoolWrapperCloseNil(t *testing.T) {
	t.Parallel()

	var w PgPoolWrapper
	if err := w.Close(); err != nil {
		t.Fatalf("Close() error = %v, want nil", err)
	}
}

func TestNewPgPoolInvalidDSN(t *testing.T) {
	t.Parallel()

	_, err := NewPgPool(context.Background(), Config{DSN: "://bad-dsn"})
	if err == nil {
		t.Fatal("NewPgPool() error = nil, want parse error")
	}
}

func TestPgPoolWrapperCloseNonNil(t *testing.T) {
	t.Parallel()

	cfg, err := pgxpool.ParseConfig("postgres://casino:casino@127.0.0.1:1/casino_transactions?sslmode=disable")
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewWithConfig() error = %v", err)
	}

	w := PgPoolWrapper{Pool: pool}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() error = %v, want nil", err)
	}
}

func TestNewPgPoolPingError(t *testing.T) {
	t.Parallel()

	_, err := NewPgPool(context.Background(), Config{
		DSN:             "postgres://casino:casino@127.0.0.1:1/casino_transactions?sslmode=disable",
		MaxOpenConns:    2,
		MaxIdleConns:    1,
		ConnMaxLifetime: time.Second,
		ConnMaxIdleTime: time.Second,
	})
	if err == nil {
		t.Fatal("NewPgPool() error = nil, want ping error")
	}
}
