package service

import "testing"

func TestCatalogRoundTrip(t *testing.T) {
	c := NewCatalog()
	c.Put("sku-1", "widget")
	if got := c.Get("sku-1"); got != "widget" {
		t.Fatalf("Get = %q, want widget", got)
	}
	c.Drop("sku-1")
	if got := c.Get("sku-1"); got != "unknown(sku-1)" {
		t.Fatalf("Get after drop = %q", got)
	}
}

func TestSearchLookup(t *testing.T) {
	s := NewSearch()
	s.Index("k1", "doc one")
	if got := s.Lookup("k1"); got != "doc one" {
		t.Fatalf("Lookup = %q", got)
	}
}
