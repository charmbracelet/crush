package tools

import "testing"

func TestPermissionDeniedResponseStopTurn(t *testing.T) {
	t.Parallel()

	// Human denial (no recorded reason): the turn stops so the agent does
	// not hammer the same call the user just refused.
	human := NewPermissionDeniedResponse()
	if !human.StopTurn {
		t.Error("human denial must set StopTurn")
	}
	if human.Content != "User denied permission" {
		t.Errorf("human denial message = %q, want generic", human.Content)
	}

	// Classifier denial (recorded reason): the model sees the reason and
	// the turn continues so it can adapt with a safer approach. Runaway
	// retries are bounded by the auto-mode quota pause, not StopTurn.
	classifier := NewPermissionDeniedResponse("[auto-mode] Blocked (1/3 consecutive, 1/20 total): command matches a statically dangerous pattern")
	if classifier.StopTurn {
		t.Error("classifier denial must not set StopTurn; auto mode relies on the model continuing")
	}
	if classifier.Content != "Permission denied: [auto-mode] Blocked (1/3 consecutive, 1/20 total): command matches a statically dangerous pattern" {
		t.Errorf("classifier denial must surface the recorded reason, got %q", classifier.Content)
	}
}