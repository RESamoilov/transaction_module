package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func TestRedisClientWrapperCloseNil(t *testing.T) {
	t.Parallel()

	var w RedisClientWrapper
	if err := w.Close(); err != nil {
		t.Fatalf("Close() error = %v, want nil", err)
	}
}

func TestNewRedisClientUnavailable(t *testing.T) {
	t.Parallel()

	_, err := NewRedisClient(context.Background(), Config{
		Addr:         "127.0.0.1:1",
		Password:     "secret",
		DialTimeout:  10 * time.Millisecond,
		ReadTimeout:  10 * time.Millisecond,
		WriteTimeout: 10 * time.Millisecond,
		PoolTimeout:  10 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("NewRedisClient() error = nil, want ping error")
	}
}

func TestNewRedisClientSuccess(t *testing.T) {
	t.Parallel()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() error = %v", err)
	}
	defer mr.Close()

	client, err := NewRedisClient(context.Background(), Config{
		Addr:         mr.Addr(),
		Password:     "",
		DialTimeout:  time.Second,
		ReadTimeout:  time.Second,
		WriteTimeout: time.Second,
		PoolTimeout:  time.Second,
	})
	if err != nil {
		t.Fatalf("NewRedisClient() error = %v", err)
	}
	defer client.Close()

	if client.Client == nil {
		t.Fatal("Client = nil, want initialized redis client")
	}
}

func TestRedisClientWrapperCloseNonNil(t *testing.T) {
	t.Parallel()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() error = %v", err)
	}
	defer mr.Close()

	client, err := NewRedisClient(context.Background(), Config{
		Addr:         mr.Addr(),
		Password:     "",
		DialTimeout:  time.Second,
		ReadTimeout:  time.Second,
		WriteTimeout: time.Second,
		PoolTimeout:  time.Second,
	})
	if err != nil {
		t.Fatalf("NewRedisClient() error = %v", err)
	}

	if err := client.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
