package dialog

import (
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func newTestModelItem(sty *styles.Styles, providerID, modelID string) *ModelItem {
	return NewModelItem(
		sty,
		catwalk.Provider{
			Name: providerID,
			ID:   catwalk.InferenceProvider(providerID),
		},
		catwalk.Model{ID: modelID, Name: modelID},
		ModelTypeLarge,
		false,
	)
}

// Build two groups of two models each, with the following flat layout.
//
//	[0]  "Group A" header
//	[1]  a1
//	[2]  a2
//	[3]  spacer
//	[4]  "Group B" header
//	[5]  b1
//	[6]  b2
//	[7]  spacer
func newTestModelsList(t *testing.T) *ModelsList {
	t.Helper()

	sty := styles.CharmtonePantera()
	f := NewModelsList(&sty)
	f.SetGroups(
		NewModelGroup(&sty, "Group A", false,
			newTestModelItem(&sty, "provider-a", "a1"),
			newTestModelItem(&sty, "provider-a", "a2"),
		),
		NewModelGroup(&sty, "Group B", false,
			newTestModelItem(&sty, "provider-b", "b1"),
			newTestModelItem(&sty, "provider-b", "b2"),
		),
	)
	f.SetSize(40, 3)
	return f
}

func TestModelsListScrollToSelectedKeepsFirstGroupHeaderVisible(t *testing.T) {
	t.Parallel()

	// A single-row viewport exercises the header adjustment when the base
	// list scrolls the selected item to the top.
	f := newTestModelsList(t)

	// Simulate wrapping from the last model to the first. The first group
	// header should become visible again.
	f.SelectLast()
	f.ScrollToSelected()
	f.SelectFirst()
	f.ScrollToSelected()

	offsetIdx, offsetLine := f.ScrollPosition()
	require.Zero(t, offsetIdx)
	require.Zero(t, offsetLine)
	require.Contains(t, f.Render(), "Group A")
}

func TestModelsListScrollToSelectedKeepsHeaderAndModelVisibleOnNavigateUp(t *testing.T) {
	t.Parallel()

	// A multi-row viewport should show both the header and the first model
	// after navigating back from the bottom.
	f := newTestModelsList(t)
	f.SelectLast()
	f.ScrollToSelected()
	for !f.IsSelectedFirst() {
		require.True(t, f.SelectPrev(), "first model was not reached")
		f.ScrollToSelected()
	}

	offsetIdx, offsetLine := f.ScrollPosition()
	require.Zero(t, offsetIdx)
	require.Zero(t, offsetLine)
	rendered := f.Render()
	require.Contains(t, rendered, "Group A")
	require.Contains(t, rendered, "a1")
}
