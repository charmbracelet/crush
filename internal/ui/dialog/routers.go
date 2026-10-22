package dialog

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// RoutersID is the identifier for the routers dialog.
const RoutersID = "routers"

const routersDialogWidth = 72

// Routers lists the configured model routers.
type Routers struct {
	com      *common.Common
	names    []string
	cursor   int
	deleting bool

	help   help.Model
	keyMap struct {
		Up, Down, Activate, New, Edit, Delete, Confirm, Close key.Binding
	}
}

var _ Dialog = (*Routers)(nil)

// NewRouters creates the routers dialog.
func NewRouters(com *common.Common) *Routers {
	r := &Routers{com: com}
	r.help = help.New()
	r.help.Styles = com.Styles.DialogHelpStyles()
	r.keyMap.Up = key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/↓", "move"))
	r.keyMap.Down = key.NewBinding(key.WithKeys("down", "j"))
	r.keyMap.Activate = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "activate"))
	r.keyMap.New = key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new"))
	r.keyMap.Edit = key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit"))
	r.keyMap.Delete = key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete"))
	r.keyMap.Confirm = key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "confirm delete"))
	r.keyMap.Close = CloseKey
	r.Refresh()
	return r
}

// ID implements Dialog.
func (r *Routers) ID() string {
	return RoutersID
}

// Refresh reloads the router list from config.
func (r *Routers) Refresh() {
	r.names = nil
	if cfg := r.com.Config(); cfg != nil {
		r.names = slices.Sorted(maps.Keys(cfg.Routers))
	}
	r.cursor = max(0, min(r.cursor, len(r.names)-1))
	r.deleting = false
}

func (r *Routers) selected() (string, bool) {
	if r.cursor < 0 || r.cursor >= len(r.names) {
		return "", false
	}
	return r.names[r.cursor], true
}

// HandleMsg implements Dialog.
func (r *Routers) HandleMsg(msg tea.Msg) Action {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}
	name, hasSel := r.selected()
	if r.deleting {
		r.deleting = false
		if hasSel && key.Matches(keyMsg, r.keyMap.Confirm) {
			return ActionDeleteRouter{Name: name}
		}
		return nil
	}
	switch {
	case key.Matches(keyMsg, r.keyMap.Close):
		return ActionClose{}
	case key.Matches(keyMsg, r.keyMap.Up):
		r.cursor = max(0, r.cursor-1)
	case key.Matches(keyMsg, r.keyMap.Down):
		r.cursor = max(0, min(len(r.names)-1, r.cursor+1))
	case key.Matches(keyMsg, r.keyMap.New):
		return ActionEditRouter{}
	case hasSel && key.Matches(keyMsg, r.keyMap.Activate):
		return ActionSelectRouter{Name: name}
	case hasSel && key.Matches(keyMsg, r.keyMap.Edit):
		return ActionEditRouter{Name: name}
	case hasSel && key.Matches(keyMsg, r.keyMap.Delete):
		r.deleting = true
	}
	return nil
}

// Draw implements Dialog.
func (r *Routers) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := r.com.Styles
	width := max(0, min(routersDialogWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	innerWidth := max(0, width-t.Dialog.View.GetHorizontalFrameSize())
	rowWidth := max(0, innerWidth-2)
	cfg := r.com.Config()

	var rows []string
	if len(r.names) == 0 {
		rows = append(rows, t.Dialog.SecondaryText.Width(innerWidth).Render("No routers yet. Press n to create one."))
	}
	for i, name := range r.names {
		rc := cfg.Routers[name]
		dot := "○ "
		if name == cfg.ActiveRouter {
			dot = "● "
		}
		classifier := rc.Classifier.Model
		if m := cfg.GetModel(rc.Classifier.Provider, rc.Classifier.Model); m != nil && m.Name != "" {
			classifier = m.Name
		}
		info := fmt.Sprintf("%d routes · %s", len(rc.Routes), classifier)
		gap := max(1, rowWidth-lipgloss.Width(dot+name)-lipgloss.Width(info))
		text := ansi.Truncate(dot+name+strings.Repeat(" ", gap)+info, rowWidth, "…")
		style := t.Dialog.NormalItem
		if i == r.cursor {
			style = t.Dialog.SelectedItem
		}
		rows = append(rows, style.Width(innerWidth).Render(text))
	}
	if r.deleting {
		name, _ := r.selected()
		rows = append(rows, "", t.Dialog.OAuth.ErrorText.Padding(0, 1).Render(
			ansi.Truncate(fmt.Sprintf("Delete router %q? Press y to confirm.", name), rowWidth, "…")))
	}

	rc := NewRenderContext(t, width)
	rc.Title = "Routers"
	rc.AddPart(lipgloss.JoinVertical(lipgloss.Left, append(append([]string{""}, rows...), "")...))
	rc.Help = renderDialogHelp(t, &r.help, r, innerWidth)
	DrawCenter(scr, area, rc.Render())
	return nil
}

// ShortHelp implements help.KeyMap.
func (r *Routers) ShortHelp() []key.Binding {
	if r.deleting {
		return []key.Binding{r.keyMap.Confirm}
	}
	if len(r.names) == 0 {
		return []key.Binding{r.keyMap.New, r.keyMap.Close}
	}
	return []key.Binding{r.keyMap.Up, r.keyMap.Activate, r.keyMap.New, r.keyMap.Edit, r.keyMap.Delete, r.keyMap.Close}
}

// FullHelp implements help.KeyMap.
func (r *Routers) FullHelp() [][]key.Binding {
	return [][]key.Binding{r.ShortHelp()}
}
