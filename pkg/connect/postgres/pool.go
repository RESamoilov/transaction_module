package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	DSN             string        `yaml:"dsn" env:"DB_DSN" env-required:"true"`
	MaxOpenConns    int           `yaml:"max_open_conns" env-default:"50"`
	MaxIdleConns    int           `yaml:"max_idle_conns" env-default:"10"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" env-default:"1h"`
	ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time" env-default:"30m"`
}

type PgPoolWrapper struct {
	Pool *pgxpool.Pool
}

func (w *PgPoolWrapper) Close() error {
	if w.Pool != nil {
		w.Pool.Close()
	}
	return nil
}

func NewPgPool(ctx context.Context, cfg Config) (*PgPoolWrapper, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse pgx config: %w", err)
	}

	poolConfig.MaxConns = int32(cfg.MaxOpenConns)
	poolConfig.MinConns = int32(cfg.MaxIdleConns)
	poolConfig.MaxConnLifetime = cfg.ConnMaxLifetime
	poolConfig.MaxConnIdleTime = cfg.ConnMaxIdleTime
	poolConfig.MaxConnLifetimeJitter = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create pgx pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return &PgPoolWrapper{Pool: pool}, nil
}
