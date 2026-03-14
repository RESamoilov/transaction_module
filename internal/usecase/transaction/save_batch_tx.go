package transaction

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/meindokuse/transaction-module/internal/domain"
)

func (uc *Transaction) ProcessBatch(ctx context.Context, txs []*domain.Transaction) error {
	if len(txs) == 0 {
		return nil
	}

	validTxs := make([]*domain.Transaction, 0, len(txs))
	idmKeys := make([]string, 0, len(txs))

	for _, tx := range txs {
		idempotencyKey := fmt.Sprintf("tx_idem_%s", tx.ID)

		mightExist, err := uc.redis.IsExists(ctx, idempotencyKey)
		if err != nil {
			slog.WarnContext(ctx, "redis bloom filter check failed, falling back to postgres", "err", err, "key", idempotencyKey)
			mightExist = false
		}

		if mightExist {
			isProcessed, err := uc.db.IsMessageProcessed(ctx, idempotencyKey)
			if err != nil {
				return fmt.Errorf("check fallback postgres: %w", err)
			}
			if isProcessed {
				slog.DebugContext(ctx, "duplicate transaction skipped", "key", idempotencyKey)
				continue
			}
		}

		validTxs = append(validTxs, tx)
		idmKeys = append(idmKeys, idempotencyKey)
	}

	if len(validTxs) == 0 {
		return nil
	}

	err := uc.db.Save(ctx, validTxs, idmKeys)
	if err != nil {
		return fmt.Errorf("db save batch: %w", err)
	}
	slog.InfoContext(ctx, "transaction batch persisted", "received_count", len(txs), "saved_count", len(validTxs))

	for _, key := range idmKeys {
		if err := uc.redis.Add(ctx, key); err != nil {
			slog.WarnContext(ctx, "failed to add key to redis bloom", "err", err, "key", key)
		}
	}

	return nil
}
