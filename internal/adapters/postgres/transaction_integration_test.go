//go:build integration

package postgres

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/meindokuse/transaction-module/internal/domain"
)

func TestTransactionRepositoryIntegration(t *testing.T) {
	_ = godotenv.Load(filepath.Join("..", "..", "..", ".env"))

	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		t.Skip("DB_DSN is not set, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	baseConn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Skipf("postgres is unavailable for integration test: %v", err)
	}
	defer baseConn.Close(context.Background())

	schema := fmt.Sprintf("txmod_test_%d", time.Now().UnixNano())
	if _, err := baseConn.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("CREATE SCHEMA error = %v", err)
	}
	t.Cleanup(func() {
		_, _ = baseConn.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
	})

	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}
	poolCfg.MaxConns = 2
	poolCfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET search_path TO "+schema)
		return err
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		t.Fatalf("NewWithConfig() error = %v", err)
	}
	defer pool.Close()

	upPath := filepath.Join("..", "..", "..", "migrations", "000001_init_schema.up.sql")
	upSQL, err := os.ReadFile(upPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", upPath, err)
	}
	if _, err := pool.Exec(ctx, string(upSQL)); err != nil {
		t.Fatalf("apply migration error = %v", err)
	}

	repo := NewTransactionRepository(pool)
	ts1 := time.Date(2026, 3, 14, 12, 0, 0, 0, time.UTC)
	ts2 := time.Date(2026, 3, 14, 11, 0, 0, 0, time.UTC)
	batch := []*domain.Transaction{
		{ID: "tx-1", UserID: "user-1", Amount: 10.5, Type: domain.TxTypeBet, Timestamp: ts1},
		{ID: "tx-2", UserID: "user-2", Amount: 20.5, Type: domain.TxTypeWin, Timestamp: ts2},
	}
	keys := []string{"tx_idem_tx-1", "tx_idem_tx-2"}

	if err := repo.Save(ctx, batch, keys); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := repo.Save(ctx, batch, keys); err != nil {
		t.Fatalf("Save() duplicate batch error = %v", err)
	}

	processed, err := repo.IsMessageProcessed(ctx, "tx_idem_tx-1")
	if err != nil {
		t.Fatalf("IsMessageProcessed() error = %v", err)
	}
	if !processed {
		t.Fatal("IsMessageProcessed() = false, want true")
	}

	userTxs, err := repo.GetByUserID(ctx, domain.TransactionFilter{UserID: "user-1", Type: "bet"}, domain.PageTransaction{Limit: 10})
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if len(userTxs) != 1 || userTxs[0].ID != "tx-1" {
		t.Fatalf("GetByUserID() = %+v, want one tx-1", userTxs)
	}

	allWin, err := repo.GetAll(ctx, domain.TransactionFilter{Type: "win"}, domain.PageTransaction{Limit: 10})
	if err != nil {
		t.Fatalf("GetAll() error = %v", err)
	}
	if len(allWin) != 1 || allWin[0].ID != "tx-2" {
		t.Fatalf("GetAll() = %+v, want one tx-2", allWin)
	}
}
