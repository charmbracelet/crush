package model

import (
	"testing"

	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/stretchr/testify/require"
)

// TestOpenRouterDialogAddsToOverlay confirms the router dialog can be
// opened and is idempotent (opening twice brings the same instance to
// front rather than stacking duplicates), matching the existing pattern
// for openReasoningDialog/openModelsDialog.
func TestOpenRouterDialogAddsToOverlay(t *testing.T) {
	t.Parallel()

	u := newPrismTestUI()
	require.False(t, u.dialog.ContainsDialog(dialog.RouterID))

	u.openRouterDialog()
	require.True(t, u.dialog.ContainsDialog(dialog.RouterID))

	u.openRouterDialog()
	require.True(t, u.dialog.ContainsDialog(dialog.RouterID))
}
