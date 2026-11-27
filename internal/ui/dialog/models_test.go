package dialog

import (
	"fmt"
	"image"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"
)

type modelsTestWorkspace struct {
	workspace.Workspace
	cfg *config.Config
}

func (w *modelsTestWorkspace) Config() *config.Config {
	return w.cfg
}

// newModelsTestDialog builds a Models dialog without touching the
// provider catalog: groups are injected directly, so tests control both
// the provider order (Hyper first) and the preselected model.
func newModelsTestDialog(t *testing.T, isOnboarding bool) *Models {
	t.Helper()

	sty := styles.CharmtonePantera()
	cfg := &config.Config{
		Providers: csync.NewMap[string, config.ProviderConfig](),
		Models: map[config.SelectedModelType]config.SelectedModel{
			config.SelectedModelTypeLarge: {
				Provider: "openai",
				Model:    "model-00",
			},
		},
	}
	m := &Models{
		com: &common.Common{
			Workspace: &modelsTestWorkspace{cfg: cfg},
			Styles:    &sty,
		},
		isOnboarding: isOnboarding,
	}
	m.help = help.New()
	m.help.Styles = sty.DialogHelpStyles()
	m.input = textinput.New()
	m.input.SetVirtualCursor(false)
	m.input.SetStyles(sty.TextInput)

	m.list = NewModelsList(&sty)
	m.list.Focus()

	hyper := catwalk.Provider{ID: "hyper", Name: "Hyper"}
	openai := catwalk.Provider{ID: "openai", Name: "OpenAI"}
	hyperGroup := NewModelGroup(&sty, "Hyper", false)
	openaiGroup := NewModelGroup(&sty, "OpenAI", true)
	for i := range 20 {
		model := catwalk.Model{ID: fmt.Sprintf("model-%02d", i), Name: fmt.Sprintf("Model %02d", i)}
		hyperGroup.AppendItems(NewModelItem(&sty, hyper, model, ModelTypeLarge, false))
		openaiGroup.AppendItems(NewModelItem(&sty, openai, model, ModelTypeLarge, false))
	}
	m.list.SetGroups(hyperGroup, openaiGroup)
	m.list.SelectFirst()

	return m
}

func TestModelsOnboardingStartsAtTopAndShowsHyper(t *testing.T) {
	t.Parallel()

	m := newModelsTestDialog(t, true)

	// Startup may have derived a default model from environment
	// credentials: simulating the preselection that setProviderItems
	// computes before selectInitialModel runs.
	m.list.SetSelectedItem("openai:model-00")
	m.list.ScrollToSelected()
	require.Equal(t, "openai:model-00", m.list.SelectedItem().(*ModelItem).ID())

	m.selectInitialModel("openai:model-00")

	item, ok := m.list.SelectedItem().(*ModelItem)
	require.True(t, ok)
	require.Equal(t, "hyper", string(item.prov.ID), "onboarding must start on Hyper, the first provider")
	require.Zero(t, m.list.Offset(), "onboarding must scroll to the top of the list")

	area := image.Rect(0, 0, 80, 30)
	scr := uv.NewScreenBuffer(80, 30)
	m.Draw(scr, area)
	require.Contains(t, m.list.Render(), "Hyper")
	require.Less(t, m.list.Offset(), 2, "Hyper must remain visible at the top after drawing")
}

func TestModelsListUsesAvailableHeight(t *testing.T) {
	t.Parallel()

	m := newModelsTestDialog(t, true)
	area := image.Rect(0, 0, 80, 50)
	scr := uv.NewScreenBuffer(80, 50)
	m.Draw(scr, area)

	require.Greater(t, m.list.Height(), defaultDialogHeight,
		"onboarding should grow past the standard dialog height when the screen is taller")
	for y := range onboardingLogoHeight + onboardingTopGap(area.Dy()) {
		require.Empty(t, strings.TrimSpace(screenRow(scr, y)),
			"the onboarding content must leave the logo and the top gap visible")
	}

	require.Contains(t, screenRow(scr, m.listArea.Min.Y), "Hyper",
		"the computed list area must point at the rendered Hyper header")

	regular := newModelsTestDialog(t, false)
	regular.Draw(scr, area)
	require.LessOrEqual(t, regular.list.Height(), defaultDialogHeight,
		"the regular models dialog must keep its standard height")
}

// screenRow returns the rendered text of one screen row, used to assert
// that computed dialog geometry matches what was actually drawn.
func screenRow(scr uv.Screen, y int) string {
	var row strings.Builder
	for x := range scr.Bounds().Dx() {
		row.WriteString(scr.CellAt(x, y).Content)
	}
	return row.String()
}

func TestOnboardingTopGap(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		height int
		gap    int
	}{
		{height: 50, gap: 6},
		{height: 45, gap: 6},
		{height: 44, gap: 4},
		{height: 30, gap: 4},
		{height: 29, gap: 2},
		{height: 18, gap: 2},
	} {
		require.Equal(t, tc.gap, onboardingTopGap(tc.height), "height %d", tc.height)
	}
}

func TestModelsOnboardingKeepsListUsableOnShortTerminals(t *testing.T) {
	t.Parallel()

	for _, height := range []int{18, 20, 24, 30} {
		m := newModelsTestDialog(t, true)
		area := image.Rect(0, 0, 80, height)
		scr := uv.NewScreenBuffer(80, height)
		m.Draw(scr, area)
		require.Greater(t, m.list.Height(), 0, "height %d must still show list rows", height)
		require.Contains(t, screenRow(scr, m.listArea.Min.Y), "Hyper",
			"height %d: hit area must match the rendered list", height)
	}
}

func TestModelsMouseWheelScrollsList(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name         string
		isOnboarding bool
	}{
		{name: "onboarding", isOnboarding: true},
		{name: "model picker", isOnboarding: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			m := newModelsTestDialog(t, tc.isOnboarding)
			area := image.Rect(0, 0, 80, 25)
			scr := uv.NewScreenBuffer(80, 25)
			m.Draw(scr, area)
			require.False(t, m.listArea.Empty(), "the drawn list must expose its geometry for mouse events")

			m.HandleMsg(common.CoalescedWheelMsg{
				Mouse:  tea.Mouse{X: m.listArea.Min.X + 1, Y: m.listArea.Min.Y + 2},
				DeltaY: 3,
			})
			require.Equal(t, 3, m.list.Offset())

			// Rendering must preserve the offset chosen by the wheel event.
			m.Draw(scr, area)
			require.Equal(t, 3, m.list.Offset())

			m.HandleMsg(common.CoalescedWheelMsg{
				Mouse:  tea.Mouse{X: m.listArea.Min.X + 1, Y: m.listArea.Min.Y + 2},
				DeltaY: -1,
			})
			require.Equal(t, 2, m.list.Offset())

			// Wheel events outside the rendered list must not scroll.
			m.HandleMsg(common.CoalescedWheelMsg{
				Mouse:  tea.Mouse{X: 0, Y: 0},
				DeltaY: 5,
			})
			require.Equal(t, 2, m.list.Offset())
		})
	}
}

func TestModelsMouseClickSelectsAndChoosesModel(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name         string
		isOnboarding bool
	}{
		{name: "onboarding", isOnboarding: true},
		{name: "model picker", isOnboarding: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			m := newModelsTestDialog(t, tc.isOnboarding)
			area := image.Rect(0, 0, 80, 25)
			scr := uv.NewScreenBuffer(80, 25)
			m.Draw(scr, area)

			// The first two rows of a group hold the header and the
			// group's first model.
			clickPoint := image.Pt(m.listArea.Min.X+1, m.listArea.Min.Y+1)
			action := m.HandleMsg(tea.MouseClickMsg(tea.Mouse{
				X:      clickPoint.X,
				Y:      clickPoint.Y,
				Button: tea.MouseLeft,
			}))

			require.Nil(t, action, "a single click only selects")
			selected, ok := m.list.SelectedItem().(*ModelItem)
			require.True(t, ok)
			require.Equal(t, "hyper:model-00", selected.ID())

			// A quick second click on the same entry chooses it.
			action = m.HandleMsg(tea.MouseClickMsg(tea.Mouse{
				X:      clickPoint.X,
				Y:      clickPoint.Y,
				Button: tea.MouseLeft,
			}))
			selectAction, ok := action.(ActionSelectModel)
			require.True(t, ok, "double click must select the model, got %T", action)
			require.Equal(t, "hyper", string(selectAction.Provider.ID))
			require.Equal(t, "model-00", selectAction.Model.Model)

			// Clicking another model must not choose it, only select.
			secondPoint := image.Pt(m.listArea.Min.X+1, m.listArea.Min.Y+2)
			action = m.HandleMsg(tea.MouseClickMsg(tea.Mouse{
				X:      secondPoint.X,
				Y:      secondPoint.Y,
				Button: tea.MouseLeft,
			}))
			require.Nil(t, action)
			selected, ok = m.list.SelectedItem().(*ModelItem)
			require.True(t, ok)
			require.Equal(t, "hyper:model-01", selected.ID())
		})
	}
}

func TestModelsMouseClickIgnoresNonModelRows(t *testing.T) {
	t.Parallel()

	m := newModelsTestDialog(t, false)
	area := image.Rect(0, 0, 80, 25)
	scr := uv.NewScreenBuffer(80, 25)
	m.Draw(scr, area)

	// Group header row.
	m.HandleMsg(tea.MouseClickMsg(tea.Mouse{
		X:      m.listArea.Min.X + 1,
		Y:      m.listArea.Min.Y,
		Button: tea.MouseLeft,
	}))
	selected, ok := m.list.SelectedItem().(*ModelItem)
	require.True(t, ok)
	require.Equal(t, "hyper:model-00", selected.ID(), "headers must not change the selection")

	// Right clicks are not part of the select/choose interaction.
	action := m.HandleMsg(tea.MouseClickMsg(tea.Mouse{
		X:      m.listArea.Min.X + 1,
		Y:      m.listArea.Min.Y + 1,
		Button: tea.MouseRight,
	}))
	require.Nil(t, action)

	// Clicking the scrollbar column must not select a model.
	m.lastClickTime = time.Time{}
	m.lastClickID = ""
	action = m.HandleMsg(tea.MouseClickMsg(tea.Mouse{
		X:      m.listArea.Min.X + 70,
		Y:      m.listArea.Min.Y + 1,
		Button: tea.MouseLeft,
	}))
	require.Nil(t, action)
	selected, ok = m.list.SelectedItem().(*ModelItem)
	require.True(t, ok)
	require.Equal(t, "hyper:model-00", selected.ID())
}

// newPickerTestModels builds a models dialog without fetching providers.
func newPickerTestModels() *Models {
	com := newPickerTestCommon()
	m := &Models{com: com, modelType: ModelTypeLarge}
	m.help = help.New()
	m.help.Styles = com.Styles.DialogHelpStyles()
	m.list = NewModelsList(com.Styles)
	m.input = textinput.New()
	m.input.SetVirtualCursor(false)
	m.input.Placeholder = m.modelType.Placeholder()
	m.input.SetStyles(com.Styles.TextInput)
	m.input.Focus()
	m.keyMap.Close = CloseKey
	return m
}

// TestModelsNarrowKeepsContext verifies that on a narrow screen the model
// picker keeps the Large/Small switch (with short labels) and ellipsizes
// the placeholder instead of cutting it mid-word.
func TestModelsNarrowKeepsContext(t *testing.T) {
	t.Parallel()
	text := strings.Join(drawFits(t, newPickerTestModels(), 40, 30), "\n")
	require.Contains(t, text, "Large")
	require.Contains(t, text, "Small")
	require.Contains(t, text, "…")

	text = strings.Join(drawFits(t, newPickerTestModels(), 100, 30), "\n")
	require.Contains(t, text, "Large Task")
	require.Contains(t, text, largeModelInputPlaceholder)
}
