package httpdelivery

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/meindokuse/transaction-module/internal/domain"
	"github.com/meindokuse/transaction-module/internal/dto"
	"github.com/meindokuse/transaction-module/internal/usecase/transaction"
)

type TransactionHandler struct {
	usecase *transaction.Transaction
}

func NewTransactionHandler(uc *transaction.Transaction) *TransactionHandler {
	return &TransactionHandler{usecase: uc}
}

func (th *TransactionHandler) GetUserTransactions(c echo.Context) error {
	var req dto.GetUserTransactionsRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if err := c.Validate(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if req.Limit == 0 {
		req.Limit = 20
	}

	cursorPayload, err := dto.DecodeCursor(req.Cursor)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid cursor")
	}

	filter := domain.TransactionFilter{
		UserID: req.UserID,
		Type:   req.Type,
	}

	page := dto.BuildPageRequest(req.Limit, cursorPayload)

	txs, err := th.usecase.GetTxByUserID(c.Request().Context(), filter, page)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, dto.BuildResponse(txs, req.Limit))

}

func (h *TransactionHandler) GetAllTransactions(c echo.Context) error {
	var req dto.GetAllTransactionsRequest

	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if err := c.Validate(&req); err != nil {
		return err
	}

	if req.Limit == 0 {
		req.Limit = 20
	}

	cursorPayload, err := dto.DecodeCursor(req.Cursor)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid cursor")
	}

	// Здесь filter.UserID остается пустым
	filter := domain.TransactionFilter{
		Type: req.Type,
	}

	page := dto.BuildPageRequest(req.Limit, cursorPayload)

	// Вызов другого метода UseCase
	txs, err := h.usecase.GetAll(c.Request().Context(), filter, page)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}

	return c.JSON(http.StatusOK, dto.BuildResponse(txs, req.Limit))
}
