package main

import (
	"fmt"

	"catalog/internal/service"
)

func main() {
	cat := service.NewCatalog()
	cat.Put("sku-1", "widget")
	cat.Put("sku-2", "gadget")

	fmt.Println("sku-1:", cat.Get("sku-1"))

	search := service.NewSearch()
	search.Index("sku-1", "widget description")
	fmt.Println("hits:", search.Lookup("sku-1"))
}
