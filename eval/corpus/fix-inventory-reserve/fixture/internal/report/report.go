// Package report renders stock summaries for operators.
package report

import (
	"fmt"
	"strings"

	"inventory/internal/inventory"
)

// Stock renders one line per known item: name, on-hand units, and
// how many remain available for new reservations.
func Stock(inv *inventory.Inventory) string {
	var b strings.Builder
	for _, item := range inv.Items() {
		fmt.Fprintf(&b, "%s: stock=%d reserved=%d available=%d\n",
			item, inv.Stock(item), inv.Reserved(item), inv.Available(item))
	}
	return b.String()
}

// LowStock returns the items whose available count is at or below
// the threshold — the reorder list.
func LowStock(inv *inventory.Inventory, threshold int) []string {
	var low []string
	for _, item := range inv.Items() {
		if inv.Available(item) <= threshold {
			low = append(low, item)
		}
	}
	return low
}
