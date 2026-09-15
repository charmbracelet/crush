package service

import (
	"catalog/internal/cache"
)

// Search is a tiny lookup index over product descriptions, backed by
// the same cache type as Catalog.
type Search struct {
	c *cache.Cache
}

// NewSearch builds a Search on the legacy cache.
func NewSearch() *Search {
	return &Search{c: cache.New()}
}

// Index stores a document under its key.
func (s *Search) Index(key, doc string) {
	s.c.Set(key, doc)
}

// Lookup returns the document for key, or "" when absent.
func (s *Search) Lookup(key string) string {
	v, _ := s.c.Get(key)
	return v
}

// Drop removes a document.
func (s *Search) Drop(key string) {
	s.c.Delete(key)
}
