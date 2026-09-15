package inventory

import (
	"errors"
	"testing"
)

func TestReserveOK(t *testing.T) {
	inv := New()
	inv.Restock("widget", 10)
	if err := inv.Reserve("widget", 4); err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if got := inv.Available("widget"); got != 6 {
		t.Fatalf("Available = %d, want 6", got)
	}
}

func TestReserveBeyondAvailable(t *testing.T) {
	inv := New()
	inv.Restock("widget", 10)
	if err := inv.Reserve("widget", 8); err != nil {
		t.Fatalf("first Reserve: %v", err)
	}
	// Only 2 remain available — reserving 5 more must fail even
	// though stock (10) covers it.
	err := inv.Reserve("widget", 5)
	if !errors.Is(err, ErrInsufficient) {
		t.Fatalf("Reserve err = %v, want ErrInsufficient", err)
	}
	if got := inv.Available("widget"); got != 2 {
		t.Fatalf("Available = %d, want 2", got)
	}
}

func TestReserveUnknownItem(t *testing.T) {
	inv := New()
	err := inv.Reserve("missing", 1)
	if !errors.Is(err, ErrUnknownItem) {
		t.Fatalf("Reserve err = %v, want ErrUnknownItem", err)
	}
}

func TestCommitMovesStock(t *testing.T) {
	inv := New()
	inv.Restock("widget", 10)
	if err := inv.Reserve("widget", 4); err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if err := inv.Commit("widget", 4); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if got := inv.Stock("widget"); got != 6 {
		t.Fatalf("Stock = %d, want 6", got)
	}
	if got := inv.Reserved("widget"); got != 0 {
		t.Fatalf("Reserved = %d, want 0", got)
	}
}

func TestItemsReturnsSorted(t *testing.T) {
	inv := New()
	inv.Restock("zebra", 1)
	inv.Restock("apple", 2)
	inv.Restock("mango", 3)
	got := inv.Items()
	want := []string{"apple", "mango", "zebra"}
	if len(got) != len(want) {
		t.Fatalf("Items = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Items[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
