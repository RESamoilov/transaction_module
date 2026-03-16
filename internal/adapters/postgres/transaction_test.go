package postgres

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/meindokuse/transaction-module/internal/domain"
	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func TestSaveEmptyBatchReturnsNil(t *testing.T) {
	t.Parallel()

	repo := NewTransactionRepository(nil)
	if err := repo.Save(context.Background(), nil, nil); err != nil {
		t.Fatalf("Save() error = %v, want nil", err)
	}
}

func TestSaveBatchSuccess(t *testing.T) {
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer mock.Close()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("CREATE TEMP TABLE tmp_processed_messages")).WillReturnResult(pgxmock.NewResult("CREATE", 1))
	mock.ExpectCopyFrom(pgx.Identifier{"tmp_processed_messages"}, []string{"idempotency_key"}).WillReturnResult(2)
	mock.ExpectCopyFrom(pgx.Identifier{"tmp_transactions"}, []string{"id", "user_id", "amount", "type", "timestamp"}).WillReturnResult(2)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO processed_messages (idempotency_key)")).WillReturnResult(pgxmock.NewResult("INSERT", 2))
	mock.ExpectCommit()

	repo := NewTransactionRepository(mock)
	err = repo.Save(context.Background(), []*domain.Transaction{
		{ID: "tx-1", UserID: "user-1", Amount: 10, Type: domain.TxTypeBet, Timestamp: time.Now().UTC()},
		{ID: "tx-2", UserID: "user-2", Amount: 20, Type: domain.TxTypeWin, Timestamp: time.Now().UTC()},
	}, []string{"k1", "k2"})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ExpectationsWereMet() error = %v", err)
	}
}

func TestGetByUserIDRequiresUserID(t *testing.T) {
	t.Parallel()

	repo := NewTransactionRepository(nil)
	_, err := repo.GetByUserID(context.Background(), domain.TransactionFilter{}, domain.PageTransaction{Limit: 10})
	if err == nil {
		t.Fatal("GetByUserID() error = nil, want required user_id error")
	}
}

func TestGetByUserIDReturnsRows(t *testing.T) {
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer mock.Close()

	rows := pgxmock.NewRows([]string{"id", "user_id", "amount", "type", "timestamp"}).
		AddRow("tx-1", "user-1", 10.5, "bet", time.Now().UTC())

	mock.ExpectQuery("SELECT id, user_id, amount, type, timestamp FROM transactions").
		WithArgs("user-1", "bet", 10).
		WillReturnRows(rows)

	repo := NewTransactionRepository(mock)
	got, err := repo.GetByUserID(context.Background(), domain.TransactionFilter{UserID: "user-1", Type: "bet"}, domain.PageTransaction{Limit: 10})
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if len(got) != 1 || got[0].ID != "tx-1" {
		t.Fatalf("GetByUserID() = %+v, want one tx-1", got)
	}
}

func TestGetAllReturnsEmptySlice(t *testing.T) {
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT id, user_id, amount, type, timestamp FROM transactions").
		WithArgs("win", 5).
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "amount", "type", "timestamp"}))

	repo := NewTransactionRepository(mock)
	got, err := repo.GetAll(context.Background(), domain.TransactionFilter{Type: "win"}, domain.PageTransaction{Limit: 5})
	if err != nil {
		t.Fatalf("GetAll() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("GetAll() len = %d, want 0", len(got))
	}
}

func TestIsMessageProcessedReturnsFlag(t *testing.T) {
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT EXISTS").
		WithArgs("idem-key").
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))

	repo := NewTransactionRepository(mock)
	processed, err := repo.IsMessageProcessed(context.Background(), "idem-key")
	if err != nil {
		t.Fatalf("IsMessageProcessed() error = %v", err)
	}
	if !processed {
		t.Fatal("IsMessageProcessed() = false, want true")
	}
}

func TestSaveBeginError(t *testing.T) {
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer mock.Close()

	mock.ExpectBegin().WillReturnError(errors.New("begin failed"))

	repo := NewTransactionRepository(mock)
	err = repo.Save(context.Background(), []*domain.Transaction{{ID: "tx-1"}}, []string{"k1"})
	if err == nil {
		t.Fatal("Save() error = nil, want begin error")
	}
}

func TestSaveCreateTempTablesError(t *testing.T) {
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer mock.Close()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("CREATE TEMP TABLE tmp_processed_messages")).WillReturnError(errors.New("exec failed"))
	mock.ExpectRollback()

	repo := NewTransactionRepository(mock)
	err = repo.Save(context.Background(), []*domain.Transaction{{ID: "tx-1"}}, []string{"k1"})
	if err == nil {
		t.Fatal("Save() error = nil, want temp table error")
	}
}

func TestSaveCopyFromProcessedMessagesError(t *testing.T) {
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer mock.Close()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("CREATE TEMP TABLE tmp_processed_messages")).WillReturnResult(pgxmock.NewResult("CREATE", 1))
	mock.ExpectCopyFrom(pgx.Identifier{"tmp_processed_messages"}, []string{"idempotency_key"}).WillReturnError(errors.New("copy failed"))
	mock.ExpectRollback()

	repo := NewTransactionRepository(mock)
	err = repo.Save(context.Background(), []*domain.Transaction{{ID: "tx-1"}}, []string{"k1"})
	if err == nil {
		t.Fatal("Save() error = nil, want copy error")
	}
}

func TestGetByUserIDQueryError(t *testing.T) {
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT id, user_id, amount, type, timestamp FROM transactions").
		WithArgs("user-1", 10).
		WillReturnError(errors.New("query failed"))

	repo := NewTransactionRepository(mock)
	_, err = repo.GetByUserID(context.Background(), domain.TransactionFilter{UserID: "user-1"}, domain.PageTransaction{Limit: 10})
	if err == nil {
		t.Fatal("GetByUserID() error = nil, want query error")
	}
}

func TestGetAllReturnsRows(t *testing.T) {
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer mock.Close()

	now := time.Now().UTC()
	rows := pgxmock.NewRows([]string{"id", "user_id", "amount", "type", "timestamp"}).
		AddRow("tx-2", "user-2", 22.5, "win", now)

	mock.ExpectQuery("SELECT id, user_id, amount, type, timestamp FROM transactions").
		WithArgs("win", now, "tx-9", 5).
		WillReturnRows(rows)

	repo := NewTransactionRepository(mock)
	got, err := repo.GetAll(
		context.Background(),
		domain.TransactionFilter{Type: "win"},
		domain.PageTransaction{Limit: 5, LastTimestamp: now, LastID: "tx-9"},
	)
	if err != nil {
		t.Fatalf("GetAll() error = %v", err)
	}
	if len(got) != 1 || got[0].ID != "tx-2" {
		t.Fatalf("GetAll() = %+v, want tx-2", got)
	}
}

func TestIsMessageProcessedQueryError(t *testing.T) {
	t.Parallel()

	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT EXISTS").
		WithArgs("idem-key").
		WillReturnError(errors.New("query failed"))

	repo := NewTransactionRepository(mock)
	_, err = repo.IsMessageProcessed(context.Background(), "idem-key")
	if err == nil {
		t.Fatal("IsMessageProcessed() error = nil, want query error")
	}
}
