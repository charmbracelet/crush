package dialog

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"
)

type routerWorkspace struct {
	workspace.Workspace
	cfg *config.Config
}

func (w *routerWorkspace) Config() *config.Config { return w.cfg }

func newRouterCommon(routers map[string]config.RouterConfig, active string) *common.Common {
	s := styles.CharmtonePantera()
	cfg := &config.Config{
		Providers:    csync.NewMap[string, config.ProviderConfig](),
		Routers:      routers,
		ActiveRouter: active,
	}
	return &common.Common{Styles: &s, Workspace: &routerWorkspace{cfg: cfg}}
}

func press(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "ctrl+s":
		return tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
	}
	r := []rune(s)[0]
	return tea.KeyPressMsg{Code: r, Text: s}
}

func typeText(e *RouterEditor, s string) {
	for _, r := range s {
		e.HandleMsg(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

func TestRouterEditorCreatesRouter(t *testing.T) {
	t.Parallel()
	e := NewRouterEditor(newRouterCommon(nil, ""), "", config.RouterConfig{}, nil)

	// Empty form fails validation and stays open.
	require.Nil(t, e.HandleMsg(press("ctrl+s")))
	require.NotEmpty(t, e.Err())

	typeText(e, "auto")
	e.HandleMsg(press("down"))
	act := e.HandleMsg(press("enter"))
	require.Equal(t, ActionPickRouterModel{Target: "classifier"}, act)
	e.SetPickedModel("classifier", config.SelectedModel{Provider: "p", Model: "cls"})

	// Move to the "+ Add route" row and add a route.
	e.HandleMsg(press("down"))
	e.HandleMsg(press("down"))
	e.HandleMsg(press("enter"))
	typeText(e, "simple")
	e.HandleMsg(press("down"))
	typeText(e, "easy stuff")
	e.HandleMsg(press("down"))
	act = e.HandleMsg(press("enter"))
	require.Equal(t, ActionPickRouterModel{Target: "route:0"}, act)
	e.SetPickedModel("route:0", config.SelectedModel{Provider: "p", Model: "m"})
	e.HandleMsg(press("esc"))

	act = e.HandleMsg(press("ctrl+s"))
	save, ok := act.(ActionSaveRouter)
	require.True(t, ok, "got %#v, err %q", act, e.Err())
	require.Equal(t, "auto", save.Name)
	require.Empty(t, save.OrigName)
	require.Equal(t, "cls", save.Router.Classifier.Model)
	require.Len(t, save.Router.Routes, 1)
	require.Equal(t, config.RouterRoute{
		Name:        "simple",
		Description: "easy stuff",
		Model:       config.SelectedModel{Provider: "p", Model: "m"},
	}, save.Router.Routes[0])
}

func TestRouterEditorRejectsDuplicateName(t *testing.T) {
	t.Parallel()
	r := config.RouterConfig{
		Classifier: config.SelectedModel{Provider: "p", Model: "c"},
		Routes:     []config.RouterRoute{{Name: "a", Description: "d", Model: config.SelectedModel{Provider: "p", Model: "m"}}},
	}
	e := NewRouterEditor(newRouterCommon(nil, ""), "", r, []string{"auto"})
	typeText(e, "auto")
	require.Nil(t, e.HandleMsg(press("ctrl+s")))
	require.Contains(t, e.Err(), "already exists")

	// Editing an existing router may keep its own name.
	e = NewRouterEditor(newRouterCommon(nil, ""), "auto", r, []string{"auto"})
	_, ok := e.HandleMsg(press("ctrl+s")).(ActionSaveRouter)
	require.True(t, ok, e.Err())
}

func TestRouterEditorDefaultAndDelete(t *testing.T) {
	t.Parallel()
	m := config.SelectedModel{Provider: "p", Model: "m"}
	r := config.RouterConfig{
		Classifier: m,
		Routes: []config.RouterRoute{
			{Name: "a", Description: "d", Model: m},
			{Name: "b", Description: "d", Model: m},
		},
	}
	e := NewRouterEditor(newRouterCommon(nil, ""), "auto", r, nil)
	for range rowRoutesStart + 1 {
		e.HandleMsg(press("down"))
	}
	e.HandleMsg(press("*"))
	_, got := e.Router()
	require.Equal(t, "b", got.Default)

	e.HandleMsg(press("d"))
	_, got = e.Router()
	require.Len(t, got.Routes, 1)
	require.Empty(t, got.Default)

	// Drawing both the main form and the route form must not panic.
	scr := uv.NewScreenBuffer(100, 30)
	e.Draw(scr, scr.Bounds())
	e.HandleMsg(press("enter"))
	e.Draw(scr, scr.Bounds())
	small := uv.NewScreenBuffer(10, 5)
	e.Draw(small, small.Bounds())
}

func TestRoutersDialog(t *testing.T) {
	t.Parallel()
	routers := map[string]config.RouterConfig{"auto": {}, "cheap": {}}
	d := NewRouters(newRouterCommon(routers, "auto"))

	require.Equal(t, ActionSelectRouter{Name: "auto"}, d.HandleMsg(press("enter")))
	d.HandleMsg(press("down"))
	require.Equal(t, ActionEditRouter{Name: "cheap"}, d.HandleMsg(press("e")))
	require.Equal(t, ActionEditRouter{}, d.HandleMsg(press("n")))

	// Delete asks for confirmation first.
	require.Nil(t, d.HandleMsg(press("d")))
	require.Equal(t, ActionDeleteRouter{Name: "cheap"}, d.HandleMsg(press("y")))
	require.Nil(t, d.HandleMsg(press("d")))
	require.Nil(t, d.HandleMsg(press("n")), "any other key cancels the delete")

	// Drawing must not panic.
	scr := uv.NewScreenBuffer(100, 30)
	d.Draw(scr, scr.Bounds())
}

func TestRouterItems(t *testing.T) {
	t.Parallel()
	s := styles.CharmtonePantera()
	item := NewRouterItem(&s, "auto", "2 routes")
	require.True(t, item.IsRouter())
	require.Equal(t, "router:auto", item.ID())
	require.Equal(t, "auto", item.Filter())
	require.Contains(t, item.Render(40), "2 routes")

	create := NewRouterCreateItem(&s)
	require.True(t, create.IsRouter())
	require.Empty(t, create.RouterName())
	require.NotEqual(t, item.ID(), create.ID())
}
