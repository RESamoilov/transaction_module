package transaction

import (
	"context"
	"fmt"

	"github.com/meindokuse/transaction-module/internal/domain"
)

func (t *Transaction) GetTxByUserID(ctx context.Context, filters domain.TransactionFilter, page domain.PageTransaction) ([]*domain.Transaction, error) {
	if filters.UserID == "" {
		return nil, fmt.Errorf("userID is empty")
	}
	txs, err := t.db.GetByUserID(ctx, filters, page)
	if err != nil {
		return nil, fmt.Errorf("GetTxByUserID err: %w", err)
	}
	return txs, nil
}
