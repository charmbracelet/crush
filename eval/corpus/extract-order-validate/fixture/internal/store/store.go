// Package store is the in-memory persistence layer for the shop.
package store

import (
	"errors"
	"fmt"
	"sync"

	"shop/internal/model"
)

// ErrNotFound is returned when a record does not exist.
var ErrNotFound = errors.New("not found")

// Store keeps orders, returns, and exchanges in memory.
type Store struct {
	mu        sync.Mutex
	orders    map[string]model.Order
	returns   map[string]model.Return
	exchanges map[string]model.Exchange
}

// New returns an empty Store.
func New() *Store {
	return &Store{
		orders:    make(map[string]model.Order),
		returns:   make(map[string]model.Return),
		exchanges: make(map[string]model.Exchange),
	}
}

// SaveOrder persists an order keyed by its ID.
func (s *Store) SaveOrder(o model.Order) error {
	if o.ID == "" {
		return fmt.Errorf("order id required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.orders[o.ID] = o
	return nil
}

// GetOrder fetches an order by ID.
func (s *Store) GetOrder(id string) (model.Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.orders[id]
	if !ok {
		return model.Order{}, fmt.Errorf("%w: order %s", ErrNotFound, id)
	}
	return o, nil
}

// SaveReturn records a return keyed by order ID.
func (s *Store) SaveReturn(r model.Return) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.returns[r.OrderID] = r
	return nil
}

// SaveExchange records an exchange keyed by order ID.
func (s *Store) SaveExchange(x model.Exchange) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.exchanges[x.OrderID] = x
	return nil
}

// Counts reports how many records of each kind are stored.
func (s *Store) Counts() (orders, returns, exchanges int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.orders), len(s.returns), len(s.exchanges)
}
