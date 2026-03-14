package domain

import (
	"time"
)

type TransactionType string

const (
	TxTypeBet TransactionType = "bet"
	TxTypeWin TransactionType = "win"
)

type Transaction struct {
	ID        string          `json:"id"`
	UserID    string          `json:"user_id"`
	Type      TransactionType `json:"transaction_type"`
	Amount    float64         `json:"amount"`
	Timestamp time.Time       `json:"timestamp"`
}
