package handlers

import (
	"errors"
	"fmt"

	"shop/internal/model"
	"shop/internal/store"
)

// ReturnHandler processes return requests.
type ReturnHandler struct {
	store *store.Store
}

// NewReturnHandler builds a ReturnHandler.
func NewReturnHandler(s *store.Store) *ReturnHandler {
	return &ReturnHandler{store: s}
}

// RequestReturn validates and records a return against an order.
func (h *ReturnHandler) RequestReturn(r model.Return) error {
	if _, err := h.store.GetOrder(r.OrderID); err != nil {
		return fmt.Errorf("lookup order: %w", err)
	}
	if r.Email == "" {
		return errors.New("email is required")
	}
	if len(r.Items) == 0 {
		return errors.New("return must contain at least one item")
	}
	for _, it := range r.Items {
		if it.Qty <= 0 {
			return fmt.Errorf("item %s: quantity must be positive", it.SKU)
		}
	}
	if r.Reason == "" {
		return errors.New("return reason is required")
	}

	return h.store.SaveReturn(r)
}
