package inventory

import "time"

type Product struct {
	ID          int64     `json:"id"`
	Code        string    `json:"code"`
	Description string    `json:"description"`
	Balance     int64     `json:"balance"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type errorResponse struct {
	Code string `json:"code"`
}
