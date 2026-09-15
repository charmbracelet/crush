package retry

import (
	"testing"
	"time"

	"taskapi/internal/config"
)

func TestDefaultPolicyUsesTimeoutMS(t *testing.T) {
	p := DefaultPolicy(config.Config{TimeoutMS: 20})
	if p.Budget != 20*time.Second {
		t.Fatalf("Budget = %v, want 20s", p.Budget)
	}
}

func TestBackoffDoubles(t *testing.T) {
	p := Policy{BaseBackoff: 100 * time.Millisecond}
	if got := p.Backoff(1); got != 0 {
		t.Fatalf("Backoff(1) = %v, want 0", got)
	}
	if got := p.Backoff(3); got != 200*time.Millisecond {
		t.Fatalf("Backoff(3) = %v, want 200ms", got)
	}
}
