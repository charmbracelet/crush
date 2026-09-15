// Package service wires the store to the handlers and exposes them
// as a single entry point for cmd/shop.
package service

import (
	"shop/internal/handlers"
	"shop/internal/store"
)

// Service bundles the shop's handlers.
type Service struct {
	store     *store.Store
	orders    *handlers.OrderHandler
	returns   *handlers.ReturnHandler
	exchanges *handlers.ExchangeHandler
}

// New builds a Service with fresh storage.
func New() *Service {
	s := store.New()
	return &Service{
		store:     s,
		orders:    handlers.NewOrderHandler(s),
		returns:   handlers.NewReturnHandler(s),
		exchanges: handlers.NewExchangeHandler(s),
	}
}

// Orders returns the order handler.
func (s *Service) Orders() *handlers.OrderHandler { return s.orders }

// Returns returns the return handler.
func (s *Service) Returns() *handlers.ReturnHandler { return s.returns }

// Exchanges returns the exchange handler.
func (s *Service) Exchanges() *handlers.ExchangeHandler { return s.exchanges }
