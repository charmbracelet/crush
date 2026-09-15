// Package order places orders against inventory reservations.
package order

import (
	"fmt"

	"inventory/internal/inventory"
)

// Place reserves qty units of item for a new order. A later Ship
// call commits the reservation.
func Place(inv *inventory.Inventory, item string, qty int) error {
	if err := inv.Reserve(item, qty); err != nil {
		return fmt.Errorf("place order: %w", err)
	}
	return nil
}

// Ship commits a previously placed reservation — the units are gone
// from stock for good.
func Ship(inv *inventory.Inventory, item string, qty int) error {
	if err := inv.Commit(item, qty); err != nil {
		return fmt.Errorf("ship order: %w", err)
	}
	return nil
}

// Cancel releases a placed-but-unshipped reservation.
func Cancel(inv *inventory.Inventory, item string, qty int) {
	inv.Release(item, qty)
}
