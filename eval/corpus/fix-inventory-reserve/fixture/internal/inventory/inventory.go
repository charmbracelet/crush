// Package inventory tracks stock levels and reservations for items.
package inventory

import (
	"errors"
	"fmt"
	"sort"
)

// ErrInsufficient is returned when a reservation cannot be satisfied
// from available stock.
var ErrInsufficient = errors.New("insufficient stock")

// ErrUnknownItem is returned for operations on items never restocked.
var ErrUnknownItem = errors.New("unknown item")

// Inventory holds stock counts and active reservations per item.
type Inventory struct {
	stock    map[string]int
	reserved map[string]int
}

// New returns an empty Inventory.
func New() *Inventory {
	return &Inventory{
		stock:    make(map[string]int),
		reserved: make(map[string]int),
	}
}

// Restock adds qty units of item to stock.
func (i *Inventory) Restock(item string, qty int) {
	i.stock[item] += qty
}

// Stock returns the on-hand units for item, ignoring reservations.
func (i *Inventory) Stock(item string) int {
	return i.stock[item]
}

// Reserved returns the units currently held by reservations.
func (i *Inventory) Reserved(item string) int {
	return i.reserved[item]
}

// Available returns the units a new reservation may still claim:
// stock minus what is already reserved.
func (i *Inventory) Available(item string) int {
	return i.stock[item] - i.reserved[item]
}

// Reserve holds qty units of item for a pending order.
func (i *Inventory) Reserve(item string, qty int) error {
	if _, ok := i.stock[item]; !ok {
		return fmt.Errorf("%w: %s", ErrUnknownItem, item)
	}
	if i.stock[item] < qty {
		return fmt.Errorf("%w: %s (have %d, want %d)", ErrInsufficient, item, i.stock[item], qty)
	}
	i.reserved[item] += qty
	return nil
}

// Release drops a reservation, returning the units to availability.
func (i *Inventory) Release(item string, qty int) {
	i.reserved[item] -= qty
	if i.reserved[item] < 0 {
		i.reserved[item] = 0
	}
}

// Commit converts a reservation into shipped stock — the units leave
// both the reserved bucket and the stock count.
func (i *Inventory) Commit(item string, qty int) error {
	if i.reserved[item] < qty {
		return fmt.Errorf("%w: %s (reserved %d, want %d)", ErrInsufficient, item, i.reserved[item], qty)
	}
	i.reserved[item] -= qty
	i.stock[item] -= qty
	return nil
}

// Items returns the sorted list of known items.
func (i *Inventory) Items() []string {
	out := make([]string, 0, len(i.stock))
	for item := range i.stock {
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}
