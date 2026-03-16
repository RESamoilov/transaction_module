package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/meindokuse/transaction-module/internal/domain"
)

type queryPool interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}

type TransactionRepository struct {
	pool queryPool
}

func NewTransactionRepository(pool queryPool) *TransactionRepository {
	return &TransactionRepository{pool: pool}
}

func (r *TransactionRepository) Save(ctx context.Context, txBatch []*domain.Transaction, idempotencyKeys []string) error {
	if len(txBatch) == 0 {
		return nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
        CREATE TEMP TABLE tmp_processed_messages (
            idempotency_key VARCHAR(255)
        ) ON COMMIT DROP;
        
        CREATE TEMP TABLE tmp_transactions (
            id VARCHAR(255),
            user_id VARCHAR(255),
            amount DECIMAL(15,4),
            type VARCHAR(50),
            timestamp TIMESTAMP WITH TIME ZONE
        ) ON COMMIT DROP;
    `)
	if err != nil {
		return fmt.Errorf("create temp tables: %w", err)
	}

	if len(idempotencyKeys) > 0 {
		_, err = tx.CopyFrom(
			ctx,
			pgx.Identifier{"tmp_processed_messages"},
			[]string{"idempotency_key"},
			pgx.CopyFromSlice(len(idempotencyKeys), func(i int) ([]interface{}, error) {
				return []interface{}{idempotencyKeys[i]}, nil
			}),
		)
		if err != nil {
			return fmt.Errorf("copyfrom tmp_processed_messages: %w", err)
		}
	}

	_, err = tx.CopyFrom(
		ctx,
		pgx.Identifier{"tmp_transactions"},
		[]string{"id", "user_id", "amount", "type", "timestamp"},
		pgx.CopyFromSlice(len(txBatch), func(i int) ([]interface{}, error) {
			item := txBatch[i]
			return []interface{}{
				item.ID,
				item.UserID,
				item.Amount,
				item.Type,
				item.Timestamp,
			}, nil
		}),
	)
	if err != nil {
		return fmt.Errorf("copyfrom tmp_transactions: %w", err)
	}

	_, err = tx.Exec(ctx, `
        INSERT INTO processed_messages (idempotency_key)
        SELECT idempotency_key FROM tmp_processed_messages
        ON CONFLICT (idempotency_key) DO NOTHING;
        
        INSERT INTO transactions (id, user_id, amount, type, timestamp)
        SELECT id, user_id, amount, type, timestamp FROM tmp_transactions
        ON CONFLICT (id) DO NOTHING;
    `)
	if err != nil {
		return fmt.Errorf("merge insert on conflict: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	slog.InfoContext(ctx, "postgres batch committed", "transactions_count", len(txBatch), "idempotency_keys_count", len(idempotencyKeys))

	return nil
}

func (r *TransactionRepository) GetByUserID(ctx context.Context, filter domain.TransactionFilter, page domain.PageTransaction) ([]*domain.Transaction, error) {
	query := `SELECT id, user_id, amount, type, timestamp FROM transactions`

	var conditions []string
	var args []interface{}
	argID := 1

	if filter.UserID != "" {
		conditions = append(conditions, fmt.Sprintf("user_id = $%d", argID))
		args = append(args, filter.UserID)
		argID++
	} else {
		return nil, fmt.Errorf("user_id is required for GetByUserID")
	}

	if filter.Type != "" {
		conditions = append(conditions, fmt.Sprintf("type = $%d", argID))
		args = append(args, filter.Type)
		argID++
	}

	if !page.LastTimestamp.IsZero() && page.LastID != "" {
		conditions = append(conditions, fmt.Sprintf("(timestamp, id) < ($%d, $%d)", argID, argID+1))
		args = append(args, page.LastTimestamp, page.LastID)
		argID += 2
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	limit := 20
	if page.Limit > 0 {
		limit = page.Limit
	}

	query += fmt.Sprintf(" ORDER BY timestamp DESC, id DESC LIMIT $%d", argID)
	args = append(args, limit)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query GetByUserID: %w", err)
	}
	defer rows.Close()

	var result []*domain.Transaction
	for rows.Next() {
		var tx domain.Transaction
		if err := rows.Scan(&tx.ID, &tx.UserID, &tx.Amount, &tx.Type, &tx.Timestamp); err != nil {
			return nil, fmt.Errorf("scan GetByUserID: %w", err)
		}
		result = append(result, &tx)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows err GetByUserID: %w", err)
	}

	if result == nil {
		return make([]*domain.Transaction, 0), nil
	}

	return result, nil
}

func (r *TransactionRepository) GetAll(ctx context.Context, filter domain.TransactionFilter, page domain.PageTransaction) ([]*domain.Transaction, error) {
	query := `SELECT id, user_id, amount, type, timestamp FROM transactions`

	var conditions []string
	var args []interface{}
	argID := 1

	if filter.Type != "" {
		conditions = append(conditions, fmt.Sprintf("type = $%d", argID))
		args = append(args, filter.Type)
		argID++
	}

	if !page.LastTimestamp.IsZero() && page.LastID != "" {
		conditions = append(conditions, fmt.Sprintf("(timestamp, id) < ($%d, $%d)", argID, argID+1))
		args = append(args, page.LastTimestamp, page.LastID)
		argID += 2
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += fmt.Sprintf(" ORDER BY timestamp DESC, id DESC LIMIT $%d", argID)
	args = append(args, page.Limit)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query GetAll: %w", err)
	}
	defer rows.Close()

	var result []*domain.Transaction
	for rows.Next() {
		var tx domain.Transaction
		if err := rows.Scan(&tx.ID, &tx.UserID, &tx.Amount, &tx.Type, &tx.Timestamp); err != nil {
			return nil, fmt.Errorf("scan GetAll: %w", err)
		}
		result = append(result, &tx)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows err GetAll: %w", err)
	}

	if result == nil {
		return make([]*domain.Transaction, 0), nil
	}

	return result, nil
}

func (r *TransactionRepository) IsMessageProcessed(ctx context.Context, idempotencyKey string) (bool, error) {
	var exists bool

	query := `
        SELECT EXISTS(
            SELECT 1 
            FROM processed_messages 
            WHERE idempotency_key = $1
        )
    `

	err := r.pool.QueryRow(ctx, query, idempotencyKey).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check message processed: %w", err)
	}

	return exists, nil
}
