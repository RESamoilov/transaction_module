package httpdelivery

import (
	"log/slog"
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
	ctx := c.Request().Context()
	var req dto.GetUserTransactionsRequest
	if err := c.Bind(&req); err != nil {
		slog.WarnContext(ctx, "failed to bind get user transactions request", "err", err)
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if err := c.Validate(req); err != nil {
		slog.WarnContext(ctx, "failed to validate get user transactions request", "err", err, "user_id", req.UserID)
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if req.Limit == 0 {
		req.Limit = 20
	}

	cursorPayload, err := dto.DecodeCursor(req.Cursor)
	if err != nil {
		slog.WarnContext(ctx, "invalid user transactions cursor", "user_id", req.UserID)
		return echo.NewHTTPError(http.StatusBadRequest, "invalid cursor")
	}

	filter := domain.TransactionFilter{
		UserID: req.UserID,
		Type:   req.Type,
	}
	page := dto.BuildPageRequest(req.Limit, cursorPayload)

	txs, err := th.usecase.GetTxByUserID(ctx, filter, page)
	if err != nil {
		slog.ErrorContext(ctx, "failed to fetch user transactions", "err", err, "user_id", req.UserID, "transaction_type", req.Type)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, dto.BuildResponse(txs, req.Limit))
}

func (h *TransactionHandler) GetAllTransactions(c echo.Context) error {
	ctx := c.Request().Context()
	var req dto.GetAllTransactionsRequest

	if err := c.Bind(&req); err != nil {
		slog.WarnContext(ctx, "failed to bind get all transactions request", "err", err)
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if err := c.Validate(&req); err != nil {
		slog.WarnContext(ctx, "failed to validate get all transactions request", "err", err, "transaction_type", req.Type)
		return err
	}

	if req.Limit == 0 {
		req.Limit = 20
	}

	cursorPayload, err := dto.DecodeCursor(req.Cursor)
	if err != nil {
		slog.WarnContext(ctx, "invalid all transactions cursor")
		return echo.NewHTTPError(http.StatusBadRequest, "invalid cursor")
	}

	filter := domain.TransactionFilter{
		Type: req.Type,
	}
	page := dto.BuildPageRequest(req.Limit, cursorPayload)

	txs, err := h.usecase.GetAll(ctx, filter, page)
	if err != nil {
		slog.ErrorContext(ctx, "failed to fetch all transactions", "err", err, "transaction_type", req.Type)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}

	return c.JSON(http.StatusOK, dto.BuildResponse(txs, req.Limit))
}
