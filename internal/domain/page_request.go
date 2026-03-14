package domain

import "time"

type PageTransaction struct {
	Limit         int
	LastTimestamp time.Time
	LastID        string
}
