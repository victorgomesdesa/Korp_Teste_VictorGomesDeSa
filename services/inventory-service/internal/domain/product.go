package domain

import "time"

type Product struct {
	ID          int64
	Code        string
	Description string
	Balance     int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
