package transaction

import (
	"context"

	"github.com/meindokuse/transaction-module/internal/domain"
)

type DB interface {
	Save(ctx context.Context, txBatch []*domain.Transaction, idempotencyKeys []string) error
	GetByUserID(ctx context.Context, filter domain.TransactionFilter, page domain.PageTransaction) ([]*domain.Transaction, error)
	GetAll(ctx context.Context, filter domain.TransactionFilter, page domain.PageTransaction) ([]*domain.Transaction, error)
	IsMessageProcessed(ctx context.Context, idempotencyKey string) (bool, error)
}

type Redis interface {
	IsExists(ctx context.Context, idempotencyKey string) (bool, error)
	Add(ctx context.Context, idempotencyKey string) error
}

type Transaction struct {
	db    DB
	redis Redis
}

func NewTransaction(db DB, redis Redis) *Transaction {
	return &Transaction{
		db:    db,
		redis: redis,
	}
}
