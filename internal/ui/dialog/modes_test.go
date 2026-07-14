package dialog

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

type modesTestWorkspace struct {
	workspace.Workspace
	mode string
}

func (w modesTestWorkspace) AgentMode() string {
	return w.mode
}

func (modesTestWorkspace) Config() *config.Config {
	return nil
}

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

func TestModesDialogStartsAtTopForNonFirstActiveMode(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	dialog := NewModes(&common.Common{
		Workspace: modesTestWorkspace{mode: config.AgentGoal},
		Styles:    &sty,
	})

	require.Equal(t, 1, dialog.modeList.Selected())
	require.Equal(t, 0, dialog.modeList.Offset())
}
