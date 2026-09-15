package handlers

import (
	"errors"
	"fmt"

	"shop/internal/model"
	"shop/internal/store"
)

// ExchangeHandler processes exchange requests.
type ExchangeHandler struct {
	store *store.Store
}

// NewExchangeHandler builds an ExchangeHandler.
func NewExchangeHandler(s *store.Store) *ExchangeHandler {
	return &ExchangeHandler{store: s}
}

// RequestExchange validates and records an exchange against an order.
func (h *ExchangeHandler) RequestExchange(x model.Exchange) error {
	if _, err := h.store.GetOrder(x.OrderID); err != nil {
		return fmt.Errorf("lookup order: %w", err)
	}
	if x.Email == "" {
		return errors.New("email is required")
	}
	if len(x.NewItems) == 0 {
		return errors.New("exchange must contain at least one item")
	}
	if x.NewTotal < 0 {
		return errors.New("exchange total must not be negative")
	}
	computed := 0
	for _, it := range x.NewItems {
		if it.Qty <= 0 {
			return fmt.Errorf("item %s: quantity must be positive", it.SKU)
		}
		computed += it.Qty * it.Price
	}
	if computed != x.NewTotal {
		return fmt.Errorf("exchange total %d does not match items %d", x.NewTotal, computed)
	}
	if x.Reason == "" {
		return errors.New("exchange reason is required")
	}

	return h.store.SaveExchange(x)
}
