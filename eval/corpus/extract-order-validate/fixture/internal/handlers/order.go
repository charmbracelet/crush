// Package handlers contains the entry points for customer-facing
// operations. Each handler currently validates its input inline.
package handlers

import (
	"errors"
	"fmt"

	"shop/internal/model"
	"shop/internal/store"
)

// OrderHandler creates orders.
type OrderHandler struct {
	store *store.Store
}

// NewOrderHandler builds an OrderHandler.
func NewOrderHandler(s *store.Store) *OrderHandler {
	return &OrderHandler{store: s}
}

// CreateOrder validates and persists a new order.
func (h *OrderHandler) CreateOrder(o model.Order) (string, error) {
	if o.Email == "" {
		return "", errors.New("email is required")
	}
	if len(o.Items) == 0 {
		return "", errors.New("order must contain at least one item")
	}
	if o.Total <= 0 {
		return "", errors.New("order total must be positive")
	}
	computed := 0
	for _, it := range o.Items {
		if it.Qty <= 0 {
			return "", fmt.Errorf("item %s: quantity must be positive", it.SKU)
		}
		computed += it.Qty * it.Price
	}
	if computed != o.Total {
		return "", fmt.Errorf("order total %d does not match items %d", o.Total, computed)
	}

	if o.Status == "" {
		o.Status = "pending"
	}
	if err := h.store.SaveOrder(o); err != nil {
		return "", fmt.Errorf("save order: %w", err)
	}
	return o.ID, nil
}
