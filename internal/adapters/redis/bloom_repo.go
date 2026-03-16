package redis

import (
	"context"

	"github.com/redis/go-redis/v9"
)

const bloomFilterKey = "transactions:bloom:idempotency"

type BloomFilterRepository struct {
	client *redis.Client
}

func NewBloomFilterRepository(client *redis.Client) *BloomFilterRepository {
	return &BloomFilterRepository{client: client}
}

// IsExists performs a probabilistic existence check in the Bloom filter.
func (r *BloomFilterRepository) IsExists(ctx context.Context, idempotencyKey string) (bool, error) {
	cmd := r.client.Do(ctx, "BF.EXISTS", bloomFilterKey, idempotencyKey)

	res, err := cmd.Int()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}

// Add stores the idempotency key after a successful database write.
func (r *BloomFilterRepository) Add(ctx context.Context, idempotencyKey string) error {
	cmd := r.client.Do(ctx, "BF.ADD", bloomFilterKey, idempotencyKey)
	return cmd.Err()
}
