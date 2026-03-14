package transaction

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/meindokuse/transaction-module/internal/domain"
)

type mockDB struct {
	saveCalled               bool
	saveBatch                []*domain.Transaction
	saveIdempotencyKeys      []string
	saveErr                  error
	getByUserIDResult        []*domain.Transaction
	getByUserIDErr           error
	getByUserIDFilter        domain.TransactionFilter
	getByUserIDPage          domain.PageTransaction
	getAllResult             []*domain.Transaction
	getAllErr                error
	getAllFilter             domain.TransactionFilter
	getAllPage               domain.PageTransaction
	isMessageProcessedResult map[string]bool
	isMessageProcessedErr    error
	isMessageProcessedCalls  []string
}

func (m *mockDB) Save(ctx context.Context, txBatch []*domain.Transaction, idempotencyKeys []string) error {
	m.saveCalled = true
	m.saveBatch = append([]*domain.Transaction(nil), txBatch...)
	m.saveIdempotencyKeys = append([]string(nil), idempotencyKeys...)
	return m.saveErr
}

func (m *mockDB) GetByUserID(ctx context.Context, filter domain.TransactionFilter, page domain.PageTransaction) ([]*domain.Transaction, error) {
	m.getByUserIDFilter = filter
	m.getByUserIDPage = page
	return m.getByUserIDResult, m.getByUserIDErr
}

func (m *mockDB) GetAll(ctx context.Context, filter domain.TransactionFilter, page domain.PageTransaction) ([]*domain.Transaction, error) {
	m.getAllFilter = filter
	m.getAllPage = page
	return m.getAllResult, m.getAllErr
}

func (m *mockDB) IsMessageProcessed(ctx context.Context, idempotencyKey string) (bool, error) {
	m.isMessageProcessedCalls = append(m.isMessageProcessedCalls, idempotencyKey)
	if m.isMessageProcessedErr != nil {
		return false, m.isMessageProcessedErr
	}
	return m.isMessageProcessedResult[idempotencyKey], nil
}

type mockRedis struct {
	existsResults map[string]bool
	existsErrors  map[string]error
	addedKeys     []string
	addErr        error
}

func (m *mockRedis) IsExists(ctx context.Context, idempotencyKey string) (bool, error) {
	if err := m.existsErrors[idempotencyKey]; err != nil {
		return false, err
	}
	return m.existsResults[idempotencyKey], nil
}

func (m *mockRedis) Add(ctx context.Context, idempotencyKey string) error {
	m.addedKeys = append(m.addedKeys, idempotencyKey)
	return m.addErr
}

func TestProcessBatchSkipsConfirmedDuplicate(t *testing.T) {
	t.Parallel()

	tx1 := &domain.Transaction{ID: "dup", UserID: "user-1", Amount: 10, Type: domain.TxTypeBet, Timestamp: time.Now().UTC()}
	tx2 := &domain.Transaction{ID: "new", UserID: "user-2", Amount: 20, Type: domain.TxTypeWin, Timestamp: time.Now().UTC()}

	db := &mockDB{
		isMessageProcessedResult: map[string]bool{
			"tx_idem_dup": true,
		},
	}
	redis := &mockRedis{
		existsResults: map[string]bool{
			"tx_idem_dup": true,
			"tx_idem_new": false,
		},
		existsErrors: map[string]error{},
	}

	uc := NewTransaction(db, redis)

	if err := uc.ProcessBatch(context.Background(), []*domain.Transaction{tx1, tx2}); err != nil {
		t.Fatalf("ProcessBatch() error = %v", err)
	}

	if !db.saveCalled {
		t.Fatal("Save() was not called")
	}
	if len(db.saveBatch) != 1 || db.saveBatch[0].ID != "new" {
		t.Fatalf("saved batch = %+v, want only tx 'new'", db.saveBatch)
	}
	if len(redis.addedKeys) != 1 || redis.addedKeys[0] != "tx_idem_new" {
		t.Fatalf("added keys = %+v, want [tx_idem_new]", redis.addedKeys)
	}
}

func TestProcessBatchFallsBackWhenRedisCheckFails(t *testing.T) {
	t.Parallel()

	tx := &domain.Transaction{ID: "tx-1", UserID: "user-1", Amount: 10, Type: domain.TxTypeBet, Timestamp: time.Now().UTC()}
	db := &mockDB{isMessageProcessedResult: map[string]bool{}}
	redis := &mockRedis{
		existsResults: map[string]bool{},
		existsErrors: map[string]error{
			"tx_idem_tx-1": errors.New("redis down"),
		},
	}

	uc := NewTransaction(db, redis)

	if err := uc.ProcessBatch(context.Background(), []*domain.Transaction{tx}); err != nil {
		t.Fatalf("ProcessBatch() error = %v", err)
	}
	if !db.saveCalled {
		t.Fatal("Save() was not called")
	}
	if len(db.isMessageProcessedCalls) != 0 {
		t.Fatalf("IsMessageProcessed() calls = %d, want 0", len(db.isMessageProcessedCalls))
	}
}

func TestGetTxByUserIDRequiresUserID(t *testing.T) {
	t.Parallel()

	uc := NewTransaction(&mockDB{}, &mockRedis{
		existsResults: map[string]bool{},
		existsErrors:  map[string]error{},
	})

	_, err := uc.GetTxByUserID(context.Background(), domain.TransactionFilter{}, domain.PageTransaction{})
	if err == nil {
		t.Fatal("GetTxByUserID() error = nil, want error")
	}
}

func TestGetAllPassesFilterAndPage(t *testing.T) {
	t.Parallel()

	want := []*domain.Transaction{
		{ID: "tx-1", UserID: "user-1", Amount: 10, Type: domain.TxTypeBet, Timestamp: time.Now().UTC()},
	}
	db := &mockDB{getAllResult: want, isMessageProcessedResult: map[string]bool{}}
	uc := NewTransaction(db, &mockRedis{
		existsResults: map[string]bool{},
		existsErrors:  map[string]error{},
	})

	filter := domain.TransactionFilter{Type: "bet"}
	page := domain.PageTransaction{Limit: 5}
	got, err := uc.GetAll(context.Background(), filter, page)
	if err != nil {
		t.Fatalf("GetAll() error = %v", err)
	}

	if len(got) != 1 || got[0].ID != "tx-1" {
		t.Fatalf("GetAll() = %+v, want tx-1", got)
	}
	if db.getAllFilter != filter {
		t.Fatalf("GetAll() filter = %+v, want %+v", db.getAllFilter, filter)
	}
	if db.getAllPage != page {
		t.Fatalf("GetAll() page = %+v, want %+v", db.getAllPage, page)
	}
}
