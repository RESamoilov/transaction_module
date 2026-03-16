package httpdelivery

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	usecasetransaction "github.com/meindokuse/transaction-module/internal/usecase/transaction"
	"github.com/meindokuse/transaction-module/pkg/validate"
)

func newHandlerEcho() *echo.Echo {
	e := echo.New()
	e.Validator = NewEchoValidator(validate.Get())
	return e
}

func TestGetAllTransactionsValidationError(t *testing.T) {
	t.Parallel()

	uc := usecasetransaction.NewTransaction(&handlerDBMock{}, handlerRedisMock{})
	handler := NewTransactionHandler(uc)
	e := newHandlerEcho()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions?limit=101", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.GetAllTransactions(c)
	if err == nil {
		t.Fatal("GetAllTransactions() error = nil, want validation error")
	}

	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("error type = %T, want *echo.HTTPError", err)
	}
	if httpErr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", httpErr.Code, http.StatusBadRequest)
	}
}

func TestGetAllTransactionsBindError(t *testing.T) {
	t.Parallel()

	uc := usecasetransaction.NewTransaction(&handlerDBMock{}, handlerRedisMock{})
	handler := NewTransactionHandler(uc)
	e := newHandlerEcho()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions?limit=bad", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.GetAllTransactions(c)
	if err == nil {
		t.Fatal("GetAllTransactions() error = nil, want bind error")
	}

	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("error type = %T, want *echo.HTTPError", err)
	}
	if httpErr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", httpErr.Code, http.StatusBadRequest)
	}
}

func TestGetUserTransactionsInvalidCursor(t *testing.T) {
	t.Parallel()

	uc := usecasetransaction.NewTransaction(&handlerDBMock{}, handlerRedisMock{})
	handler := NewTransactionHandler(uc)
	e := newHandlerEcho()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/user-1/transactions?cursor=bad", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/users/:user_id/transactions")
	c.SetParamNames("user_id")
	c.SetParamValues("user-1")

	err := handler.GetUserTransactions(c)
	if err == nil {
		t.Fatal("GetUserTransactions() error = nil, want invalid cursor error")
	}

	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("error type = %T, want *echo.HTTPError", err)
	}
	if httpErr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", httpErr.Code, http.StatusBadRequest)
	}
}

func TestGetUserTransactionsInternalError(t *testing.T) {
	t.Parallel()

	db := &handlerDBMock{getByUserIDErr: errors.New("db error")}
	uc := usecasetransaction.NewTransaction(db, handlerRedisMock{})
	handler := NewTransactionHandler(uc)
	e := newHandlerEcho()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/user-1/transactions", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/users/:user_id/transactions")
	c.SetParamNames("user_id")
	c.SetParamValues("user-1")

	err := handler.GetUserTransactions(c)
	if err == nil {
		t.Fatal("GetUserTransactions() error = nil, want internal error")
	}

	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("error type = %T, want *echo.HTTPError", err)
	}
	if httpErr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", httpErr.Code, http.StatusInternalServerError)
	}
}
