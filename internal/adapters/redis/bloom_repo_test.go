package redis

import (
	"context"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

func newUnavailableRedisClient() *goredis.Client {
	return goredis.NewClient(&goredis.Options{
		Addr:         "127.0.0.1:1",
		DialTimeout:  10 * time.Millisecond,
		ReadTimeout:  10 * time.Millisecond,
		WriteTimeout: 10 * time.Millisecond,
	})
}

func TestBloomFilterRepositoryIsExistsReturnsError(t *testing.T) {
	t.Parallel()

	client := newUnavailableRedisClient()
	defer client.Close()

	repo := NewBloomFilterRepository(client)
	exists, err := repo.IsExists(context.Background(), "key")
	if err == nil {
		t.Fatal("IsExists() error = nil, want redis error")
	}
	if exists {
		t.Fatal("IsExists() = true, want false on redis error")
	}
}

func TestBloomFilterRepositoryAddReturnsError(t *testing.T) {
	t.Parallel()

	client := newUnavailableRedisClient()
	defer client.Close()

	repo := NewBloomFilterRepository(client)
	if err := repo.Add(context.Background(), "key"); err == nil {
		t.Fatal("Add() error = nil, want redis error")
	}
}
