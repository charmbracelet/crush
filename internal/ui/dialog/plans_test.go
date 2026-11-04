package dialog

import (
	"context"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/plan"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/ui/util"
	"github.com/charmbracelet/crush/internal/workspace"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

type plansWorkspace struct {
	workspace.Workspace
	dir string
}

func (w *plansWorkspace) WorkingDir() string     { return w.dir }
func (w *plansWorkspace) Config() *config.Config { return &config.Config{} }

func (w *plansWorkspace) ListPlans(ctx context.Context) ([]string, error) {
	return plan.List(ctx, w.dir)
}

func (w *plansWorkspace) ReadPlan(ctx context.Context, name string) (string, error) {
	return plan.Read(ctx, w.dir, name)
}

func plansCommon(t *testing.T) *common.Common {
	t.Helper()
	sty := styles.CharmtonePantera()
	return &common.Common{Workspace: &plansWorkspace{dir: t.TempDir()}, Styles: &sty}
}

func writeTestPlan(t *testing.T, com *common.Common, name string) string {
	t.Helper()
	dir := filepath.Join(com.Workspace.WorkingDir(), ".crush", "plans")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte("plan"), 0o600))
	return path
}

func TestPlansMenuAvailabilityRefresh(t *testing.T) {
	t.Parallel()
	com := plansCommon(t)
	newCommands := func() *Commands {
		c, err := NewCommands(com, "", false, false, false, false, nil, nil)
		require.NoError(t, err)
		available := false
		c.dockerMCPAvailable = &available
		return c
	}
	c := newCommands()
	cmd := c.InitialCmd()
	path := writeTestPlan(t, com, "plan.txt")
	// Discovery happens when the command executes, not during construction.
	c.HandleMsg(cmd())
	require.True(t, c.hasPlans)
	c.input.SetValue("my plans")
	c.list.SetFilter("my plans")
	c.HandleMsg(c.InitialCmd()())
	c.HandleMsg(dockerMCPAvailabilityCheckedMsg{available: false})
	require.Equal(t, "my plans", c.input.Value())
	require.Len(t, c.list.FilteredItems(), 1)
	require.Equal(t, ActionOpenDialog{PlansID}, c.list.SelectedItem().(*CommandItem).Action())

	require.NoError(t, os.Remove(path))
	for _, state := range []string{"empty", "directories", "missing"} {
		t.Run(state, func(t *testing.T) {
			dir := filepath.Dir(path)
			if state == "directories" {
				require.NoError(t, os.Mkdir(filepath.Join(dir, "nested"), 0o700))
			}
			if state == "missing" {
				require.NoError(t, os.RemoveAll(dir))
			}
			fresh := newCommands()
			fresh.HandleMsg(fresh.InitialCmd()())
			require.False(t, fresh.hasPlans)
			for _, item := range fresh.defaultCommands() {
				require.NotEqual(t, "show_my_plans", item.ID())
			}
		})
	}
	// Results from an older menu cannot populate a newly opened menu.
	fresh := newCommands()
	fresh.HandleMsg(plansAvailabilityMsg{owner: c, files: []string{path}})
	require.False(t, fresh.hasPlans)
}

func TestPlansListRefreshFilterAndRead(t *testing.T) {
	t.Parallel()
	com := plansCommon(t)
	older := writeTestPlan(t, com, "2026-01-01-old.txt")
	newer := writeTestPlan(t, com, "2026-11-04-план with spaces.md")
	require.NoError(t, os.Mkdir(filepath.Join(filepath.Dir(newer), "nested"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(filepath.Dir(newer), "nested", "hidden.md"), nil, 0o600))
	r := NewPlans(com)
	r.HandleMsg(r.InitialCmd()())
	require.Equal(t, filepath.Base(newer), r.list.ItemAt(0).(*CommandItem).ID())
	require.Equal(t, filepath.Base(older), r.list.ItemAt(1).(*CommandItem).ID())
	require.Len(t, r.list.FilteredItems(), 2)
	item := r.list.ItemAt(0).(*CommandItem)
	rendered := item.Render(70)
	require.NotContains(t, rendered, "file://")
	require.Contains(t, ansi.Strip(rendered), filepath.Base(newer))
	cmd := r.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter}).(ActionCmd).Cmd
	read := cmd().(PlanReadMsg)
	require.NoError(t, read.Err)
	require.Equal(t, filepath.Base(newer), read.Name)
	require.Equal(t, "plan", read.Content)
	require.Same(t, r, read.Owner)
	r.HandleMsg(tea.KeyPressMsg{Code: 'o', Text: "old"})
	require.Len(t, r.list.FilteredItems(), 1)
	require.Equal(t, filepath.Base(older), r.list.SelectedItem().(*CommandItem).ID())
	require.IsType(t, ActionCmd{}, r.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter}))
	require.IsType(t, ActionClose{}, r.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEscape}))

	require.NoError(t, os.Remove(older))
	require.NoError(t, os.Remove(newer))
	fresh := NewPlans(com)
	fresh.HandleMsg(fresh.InitialCmd()())
	require.Empty(t, fresh.list.FilteredItems())
	require.Nil(t, fresh.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter}))
	scr := uv.NewScreenBuffer(80, 30)
	fresh.Draw(scr, image.Rect(0, 0, 80, 30))
	require.Contains(t, ansi.Strip(scr.Render()), "No plans found")
}

func TestPlansMouseAndNarrowLayout(t *testing.T) {
	t.Parallel()
	com := plansCommon(t)
	for i := range 25 {
		writeTestPlan(t, com, fmt.Sprintf("%02d-%s.md", i, strings.Repeat("long", 20)))
	}
	r := NewPlans(com)
	r.HandleMsg(r.InitialCmd()())
	scr := uv.NewScreenBuffer(80, 30)
	r.Draw(scr, image.Rect(0, 0, 80, 30))
	require.False(t, r.bodyArea.Empty())
	require.IsType(t, ActionCmd{}, r.HandleMsg(tea.MouseClickMsg(tea.Mouse{X: r.bodyArea.Min.X, Y: r.bodyArea.Min.Y + 1, Button: tea.MouseLeft})))
	require.Equal(t, 1, r.list.Selected())
	require.Nil(t, r.HandleMsg(tea.MouseClickMsg(tea.Mouse{X: 0, Y: 0, Button: tea.MouseLeft})))
	r.HandleMsg(common.CoalescedWheelMsg{Mouse: tea.Mouse{X: r.bodyArea.Min.X, Y: r.bodyArea.Min.Y}, DeltaY: 3})
	offset := r.list.Offset()
	require.Greater(t, offset, 0)
	r.Draw(scr, image.Rect(0, 0, 80, 30))
	require.Equal(t, offset, r.list.Offset())
	for _, size := range []image.Point{{40, 15}, {20, 8}, {8, 4}, {1, 1}} {
		require.NotPanics(t, func() { r.Draw(uv.NewScreenBuffer(size.X, size.Y), image.Rect(0, 0, size.X, size.Y)) })
	}
}

func TestPlansReadErrors(t *testing.T) {
	t.Parallel()
	com := plansCommon(t)
	path := writeTestPlan(t, com, "plan.md")
	r := NewPlans(com)
	r.HandleMsg(r.InitialCmd()())
	cmd := r.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter}).(ActionCmd).Cmd
	require.NoError(t, os.Remove(path))
	require.Error(t, cmd().(PlanReadMsg).Err)
	require.NoError(t, os.RemoveAll(filepath.Dir(path)))
	require.NoError(t, os.WriteFile(filepath.Dir(path), nil, 0o600))
	action := r.HandleMsg(r.InitialCmd()()).(ActionCmd)
	require.Contains(t, action.Cmd().(util.InfoMsg).Msg, "list plans")
}
