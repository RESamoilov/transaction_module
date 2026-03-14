package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Config struct {
	Addr     string `yaml:"addr" env:"REDIS_ADDR" env-required:"true"`
	Password string `yaml:"password" env:"REDIS_PASSWORD" env-required:"true"`
	DB       int    `yaml:"db" env:"REDIS_DB" env-default:"0"`

	PoolSize     int `yaml:"pool_size" env-default:"100"`
	MinIdleConns int `yaml:"min_idle_conns" env-default:"10"`

	DialTimeout  time.Duration `yaml:"dial_timeout" env-default:"5s"`
	ReadTimeout  time.Duration `yaml:"read_timeout" env-default:"3s"`
	WriteTimeout time.Duration `yaml:"write_timeout" env-default:"3s"`
	PoolTimeout  time.Duration `yaml:"pool_timeout" env-default:"4s"`
}

type RedisClientWrapper struct {
	Client *redis.Client
}

func (w *RedisClientWrapper) Close() error {
	if w.Client != nil {
		if err := w.Client.Close(); err != nil {
			return err
		}
	}
	return nil
}

func NewRedisClient(ctx context.Context, cfg Config) (*RedisClientWrapper, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,

		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,

		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		PoolTimeout:  cfg.PoolTimeout,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	clientWrapped := &RedisClientWrapper{Client: client}

	return clientWrapped, nil
}
