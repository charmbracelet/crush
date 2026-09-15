// Package service contains the catalog-facing services built on the
// key/value backend.
package service

import (
	"fmt"

	"catalog/internal/cache"
)

// Catalog keeps SKU → name mappings in a cache backend.
type Catalog struct {
	c *cache.Cache
}

// NewCatalog builds a Catalog on the legacy cache.
func NewCatalog() *Catalog {
	return &Catalog{c: cache.New()}
}

// Put stores a SKU mapping.
func (c *Catalog) Put(sku, name string) {
	c.c.Set(sku, name)
}

// Get resolves a SKU to its name, or "unknown" when absent.
func (c *Catalog) Get(sku string) string {
	if v, ok := c.c.Get(sku); ok {
		return v
	}
	return fmt.Sprintf("unknown(%s)", sku)
}

// Drop removes a SKU mapping.
func (c *Catalog) Drop(sku string) {
	c.c.Delete(sku)
}

// Size reports how many SKUs are held.
func (c *Catalog) Size() int {
	return c.c.Len()
}
