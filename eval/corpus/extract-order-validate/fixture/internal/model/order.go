// Package model defines the domain types shared across the shop.
package model

// Item is a single line in an order.
type Item struct {
	SKU   string
	Qty   int
	Price int // cents
}

// Order is a customer purchase.
type Order struct {
	ID     string
	Email  string
	Items  []Item
	Total  int // cents
	Status string
}

// Return is a request to send items back.
type Return struct {
	OrderID string
	Email   string
	Items   []Item
	Reason  string
}

// Exchange is a request to swap items for others.
type Exchange struct {
	OrderID   string
	Email     string
	OldItems  []Item
	NewItems  []Item
	NewTotal  int // cents
	Reason    string
}
