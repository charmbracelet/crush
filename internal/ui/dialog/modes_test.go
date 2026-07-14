package dialog

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func TestModeModelTypeLabel(t *testing.T) {
	t.Parallel()

	require.Equal(t, "Large Task", modelTypeLabel(config.SelectedModelTypeLarge))
	require.Equal(t, "Small Task", modelTypeLabel(config.SelectedModelTypeSmall))
	require.Equal(t, "Summary", modelTypeLabel(config.SelectedModelTypeSummary))
	require.Equal(t, "Review", modelTypeLabel(config.SelectedModelTypeReview))
}

func TestAdjacentModelTypeCyclesAllSlots(t *testing.T) {
	t.Parallel()

	require.Equal(t, config.SelectedModelTypeSmall, adjacentModelType(config.SelectedModelTypeLarge, 1))
	require.Equal(t, config.SelectedModelTypeSummary, adjacentModelType(config.SelectedModelTypeSmall, 1))
	require.Equal(t, config.SelectedModelTypeReview, adjacentModelType(config.SelectedModelTypeSummary, 1))
	require.Equal(t, config.SelectedModelTypeLarge, adjacentModelType(config.SelectedModelTypeReview, 1))
	require.Equal(t, config.SelectedModelTypeReview, adjacentModelType(config.SelectedModelTypeLarge, -1))
}

func TestInteractiveModesMapTaskToWritableCoder(t *testing.T) {
	t.Parallel()

	require.Equal(t, []modeDefinition{
		{ID: config.AgentCoder, Title: "Task"},
		{ID: config.AgentGoal, Title: "Goal"},
		{ID: config.AgentReview, Title: "Review"},
	}, modeDefinitions)
}

func TestModesDialogKeepsEveryModeVisible(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	heightOffset := sty.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		sty.Dialog.HelpView.GetVerticalFrameSize() + sty.Dialog.View.GetVerticalFrameSize()
	require.GreaterOrEqual(t, modesDialogMaxHeight-heightOffset, len(modeDefinitions))
}
