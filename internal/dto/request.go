package dto

import (
	"time"

	"github.com/meindokuse/transaction-module/internal/domain"
)

type GetAllTransactionsRequest struct {
	Type   string `query:"transaction_type" validate:"omitempty,oneof=bet win"`
	Limit  int    `query:"limit" validate:"omitempty,min=1,max=100"`
	Cursor string `query:"cursor" validate:"omitempty"`
}

type GetUserTransactionsRequest struct {
	UserID string `param:"user_id" validate:"required"`
	Type   string `query:"transaction_type" validate:"omitempty,oneof=bet win"`
	Limit  int    `query:"limit" validate:"omitempty,min=1,max=100"`
	Cursor string `query:"cursor" validate:"omitempty"`
}

type CursorPayload struct {
	LastID        string `json:"last_id"`
	LastTimestamp int64  `json:"last_timestamp"`
}

func BuildPageRequest(limit int, cursor *CursorPayload) domain.PageTransaction {
	page := domain.PageTransaction{Limit: limit}
	if cursor != nil {
		page.LastID = cursor.LastID
		page.LastTimestamp = time.Unix(0, cursor.LastTimestamp)
	}
	return page
}
