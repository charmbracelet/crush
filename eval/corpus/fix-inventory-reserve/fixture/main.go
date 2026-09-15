package main

import (
	"fmt"

	"inventory/internal/inventory"
	"inventory/internal/order"
	"inventory/internal/report"
)

func main() {
	inv := inventory.New()
	inv.Restock("widget", 10)
	inv.Restock("gadget", 4)

	if err := order.Place(inv, "widget", 3); err != nil {
		fmt.Println("order failed:", err)
	}

	fmt.Println(report.Stock(inv))
}
