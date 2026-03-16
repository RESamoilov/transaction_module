package dto

type TransactionEvent struct {
	ID        string  `json:"id" validate:"required"`
	UserID    string  `json:"user_id" validate:"required"`
	Amount    float64 `json:"amount" validate:"required,gt=0"`
	Type      string  `json:"transaction_type" validate:"required,oneof=bet win"`
	Timestamp string  `json:"timestamp" validate:"required,datetime=2006-01-02T15:04:05Z07:00"`
}
