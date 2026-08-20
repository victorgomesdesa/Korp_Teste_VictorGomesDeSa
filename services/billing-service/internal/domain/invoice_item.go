package domain

type InvoiceItem struct {
	ID                 int64
	InvoiceID          int64
	ProductID          int64
	ProductCode        string
	ProductDescription string
	UnitPriceInCents   int64
	Quantity           int64
}
