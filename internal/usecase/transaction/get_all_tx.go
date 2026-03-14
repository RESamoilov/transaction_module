package transaction

import (
	"context"
	"fmt"

	"github.com/meindokuse/transaction-module/internal/domain"
)

func (t *Transaction) GetAll(ctx context.Context, filters domain.TransactionFilter, page domain.PageTransaction) ([]*domain.Transaction, error) {

	txs, err := t.db.GetAll(ctx, filters, page)
	if err != nil {
		return nil, fmt.Errorf("GetAll err: %w", err)
	}
	return txs, nil
}
