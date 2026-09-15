package main

import (
	"fmt"

	"shop/internal/model"
	"shop/internal/service"
)

func main() {
	svc := service.New()

	o := model.Order{
		ID:    "o-1",
		Email: "customer@example.com",
		Items: []model.Item{{SKU: "w-1", Qty: 2, Price: 999}},
		Total: 1998,
	}
	id, err := svc.Orders().CreateOrder(o)
	fmt.Println("created:", id, "err:", err)
}
