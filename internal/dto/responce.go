package dto

import (
	"time"

	"github.com/meindokuse/transaction-module/internal/domain"
)

type TransactionItem struct {
	ID        string  `json:"id"`
	UserID    string  `json:"user_id"`
	Amount    float64 `json:"amount"`
	Type      domain.TransactionType  `json:"transaction_type"`
	Timestamp string  `json:"timestamp"`
}

// ResponseMeta метаданные ответа (информация о пагинации)
type ResponseMeta struct {
	HasMore    bool   `json:"has_more"`
	NextCursor string `json:"next_cursor,omitempty"`
}

type GetTransactionsResponse struct {
	Data []TransactionItem `json:"data"`
	Meta ResponseMeta      `json:"meta"`
}

func BuildResponse(txs []*domain.Transaction, limit int) GetTransactionsResponse {
	resp := GetTransactionsResponse{
		Data: make([]TransactionItem, 0, len(txs)),
	}

	for _, tx := range txs {
		resp.Data = append(resp.Data, TransactionItem{
			ID:        tx.ID,
			UserID:    tx.UserID,
			Amount:    tx.Amount,
			Type:      tx.Type,
			Timestamp: tx.Timestamp.Format(time.RFC3339),
		})
	}

	if len(txs) == limit && len(txs) > 0 {
		lastTx := txs[len(txs)-1]
		nextCursorPayload := CursorPayload{
			LastID:        lastTx.ID,
			LastTimestamp: lastTx.Timestamp.UnixNano(),
		}

		encodedCursor, _ := EncodeCursor(nextCursorPayload)
		resp.Meta.HasMore = true
		resp.Meta.NextCursor = encodedCursor
	} else {
		resp.Meta.HasMore = false
	}

	return resp
}
