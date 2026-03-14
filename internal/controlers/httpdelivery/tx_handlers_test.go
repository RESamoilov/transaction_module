package httpdelivery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/meindokuse/transaction-module/internal/domain"
	usecasetransaction "github.com/meindokuse/transaction-module/internal/usecase/transaction"
	"github.com/meindokuse/transaction-module/pkg/validate"
)

type handlerDBMock struct {
	getByUserIDResult []*domain.Transaction
	getByUserIDErr    error
	getAllResult      []*domain.Transaction
	getAllErr         error
}

func (m *handlerDBMock) Save(ctx context.Context, txBatch []*domain.Transaction, idempotencyKeys []string) error {
	return nil
}

func (m *handlerDBMock) GetByUserID(ctx context.Context, filter domain.TransactionFilter, page domain.PageTransaction) ([]*domain.Transaction, error) {
	return m.getByUserIDResult, m.getByUserIDErr
}

func (m *handlerDBMock) GetAll(ctx context.Context, filter domain.TransactionFilter, page domain.PageTransaction) ([]*domain.Transaction, error) {
	return m.getAllResult, m.getAllErr
}

func (m *handlerDBMock) IsMessageProcessed(ctx context.Context, idempotencyKey string) (bool, error) {
	return false, nil
}

type handlerRedisMock struct{}

func (handlerRedisMock) IsExists(ctx context.Context, idempotencyKey string) (bool, error) { return false, nil }
func (handlerRedisMock) Add(ctx context.Context, idempotencyKey string) error              { return nil }

func newHandlerTestEcho() *echo.Echo {
	e := echo.New()
	e.Validator = NewEchoValidator(validate.Get())
	return e
}

func TestGetUserTransactionsReturnsJSONResponse(t *testing.T) {
	t.Parallel()

	db := &handlerDBMock{
		getByUserIDResult: []*domain.Transaction{
			{
				ID:        "tx-1",
				UserID:    "user-1",
				Amount:    50,
				Type:      domain.TxTypeBet,
				Timestamp: time.Date(2026, 3, 14, 12, 0, 0, 0, time.UTC),
			},
		},
	}
	uc := usecasetransaction.NewTransaction(db, handlerRedisMock{})
	handler := NewTransactionHandler(uc)
	e := newHandlerTestEcho()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/user-1/transactions?transaction_type=bet&limit=1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/users/:user_id/transactions")
	c.SetParamNames("user_id")
	c.SetParamValues("user-1")

	if err := handler.GetUserTransactions(c); err != nil {
		t.Fatalf("GetUserTransactions() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp struct {
		Data []struct {
			ID   string `json:"id"`
			Type string `json:"transaction_type"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response error = %v", err)
	}
	if len(resp.Data) != 1 || resp.Data[0].ID != "tx-1" || resp.Data[0].Type != "bet" {
		t.Fatalf("response = %+v, want one bet transaction", resp)
	}
}

func TestGetAllTransactionsInvalidCursor(t *testing.T) {
	t.Parallel()

	uc := usecasetransaction.NewTransaction(&handlerDBMock{}, handlerRedisMock{})
	handler := NewTransactionHandler(uc)
	e := newHandlerTestEcho()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions?cursor=not-base64", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.GetAllTransactions(c)
	if err == nil {
		t.Fatal("GetAllTransactions() error = nil, want HTTP error")
	}

	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("error type = %T, want *echo.HTTPError", err)
	}
	if httpErr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", httpErr.Code, http.StatusBadRequest)
	}
}

func TestGetAllTransactionsReturnsPaginatedResponse(t *testing.T) {
	t.Parallel()

	db := &handlerDBMock{
		getAllResult: []*domain.Transaction{
			{
				ID:        "tx-1",
				UserID:    "user-1",
				Amount:    15,
				Type:      domain.TxTypeWin,
				Timestamp: time.Date(2026, 3, 14, 13, 0, 0, 0, time.UTC),
			},
		},
	}
	uc := usecasetransaction.NewTransaction(db, handlerRedisMock{})
	handler := NewTransactionHandler(uc)
	e := newHandlerTestEcho()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions?transaction_type=win&limit=2", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.GetAllTransactions(c); err != nil {
		t.Fatalf("GetAllTransactions() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp struct {
		Data []struct {
			ID   string `json:"id"`
			Type string `json:"transaction_type"`
		} `json:"data"`
		Meta struct {
			HasMore bool `json:"has_more"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response error = %v", err)
	}
	if len(resp.Data) != 1 || resp.Data[0].Type != "win" {
		t.Fatalf("response = %+v, want one win transaction", resp)
	}
	if resp.Meta.HasMore {
		t.Fatal("Meta.HasMore = true, want false")
	}
}
