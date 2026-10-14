package home

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/dustin/go-humanize"

	homedir "github.com/charmbracelet/crush/internal/home"
)

// View returns the cached frame; it is only re-rendered after Update
// changed something.
func (m *Model) View() tea.View {
	if m.frame == "" {
		m.frame, m.cursorPos = m.render()
		m.renders++
	}
	v := tea.NewView(m.frame)
	v.AltScreen = true
	v.BackgroundColor = m.sty.Background
	v.WindowTitle = "agent-cli"
	if m.cursorPos != nil {
		c := *m.cursorPos
		v.Cursor = &c
	}
	return v
}

// segment is a piece of a row with its style.
type segment struct {
	text  string
	style lipgloss.Style
}

func (m *Model) render() (string, *tea.Cursor) {
	t := m.sty
	w := max(20, m.width)
	var b strings.Builder

	title := "agent-cli · Home"
	switch m.screen {
	case screenSessions:
		title = filepath.Base(m.ws.Path) + "  " + homedir.Short(m.ws.Path) + "  · " + m.serverNote()
	case screenPath:
		title = "Open other folder"
	}
	b.WriteString(ansi.Truncate(t.Dialog.Title.Render(title), w, "…") + "\n")
	b.WriteString(" " + ansi.Truncate(m.input.View(), w-1, "") + "\n\n")

	// Title, input, blank, then the list, a blank, the note and the help.
	listH := max(3, m.height-6)
	start := 0
	if m.cursor >= listH {
		start = m.cursor - listH + 1
	}
	for i := start; i < len(m.rows) && i < start+listH; i++ {
		b.WriteString(m.renderRow(m.rows[i], i == m.cursor, w) + "\n")
	}
	b.WriteString("\n")
	if m.note != "" {
		b.WriteString(ansi.Truncate(t.Dialog.TitleError.Render(" "+m.note), w, "…"))
	}
	b.WriteString("\n" + ansi.Truncate(t.Section.Title.Render(" "+m.help()), w, "…"))

	var cur *tea.Cursor
	if c := m.input.Cursor(); c != nil {
		c.X++ // The input is indented by one column.
		c.Y = 1
		cur = c
	}
	return b.String(), cur
}

func (m *Model) help() string {
	switch m.screen {
	case screenSessions:
		return "type to search · ↑↓ choose · enter open · esc back"
	case screenPath:
		return "enter open · esc back"
	}
	esc := "esc quit"
	if m.input.Value() != "" || m.showAll {
		esc = "esc clear"
	}
	return "type to search · ↑↓ choose · enter open · ctrl+p pin/unpin · ctrl+x remove missing · " + esc
}

// serverNote says where the session statuses come from.
func (m *Model) serverNote() string {
	switch m.live[m.ws.Path].State {
	case ServerLive:
		return "server running"
	case ServerUnknown:
		return "server not responding: status unknown"
	}
	return "no server: status unknown"
}

func (m *Model) renderRow(r row, selected bool, w int) string {
	t := m.sty
	if r.kind == rowHeader {
		return ansi.Truncate(t.Section.Title.Render(" "+r.text), w, "…")
	}
	segs := m.rowSegments(r)
	if selected {
		// Plain text inside one style: inner resets would drop the highlight.
		var plain strings.Builder
		for _, s := range segs {
			plain.WriteString(s.text)
		}
		return t.Dialog.SelectedItem.Width(w).Render(ansi.Truncate(plain.String(), w-2, "…"))
	}
	var out strings.Builder
	out.WriteString(" ")
	for _, s := range segs {
		out.WriteString(s.style.Render(s.text))
	}
	return ansi.Truncate(out.String(), w, "…")
}

func (m *Model) rowSegments(r row) []segment {
	t := m.sty
	base := t.Dialog.NormalItem.UnsetPadding()
	muted := t.Header.WorkingDir
	switch r.kind {
	case rowOpenCwd:
		return []segment{{"→ Open current folder  ", base}, {homedir.Short(m.cwd), muted}}
	case rowOpenPath:
		return []segment{{"→ Open other folder…", base}}
	case rowShowAll:
		return []segment{{fmt.Sprintf("→ Show all / search (%d)", len(m.workspaces)), base}}
	case rowNewSession:
		return []segment{{"→ New session", base}}
	case rowSession:
		title := r.sess.Title
		if title == "" {
			title = "Untitled session"
		}
		segs := []segment{{"  " + title, base}}
		if !r.sess.UpdatedAt.IsZero() {
			segs = append(segs, segment{"  " + humanize.Time(r.sess.UpdatedAt), muted})
		}
		return append(segs, m.statusSegment(r.sess.Status))
	}

	// The path goes last: it is the longest part and the first to be cut.
	ws := r.ws
	segs := []segment{{"  " + filepath.Base(ws.Path), base}}
	if ws.Branch != "" {
		segs = append(segs, segment{"  " + ws.Branch, t.Header.GitBranch})
	}
	if ws.Pinned {
		segs = append(segs, segment{"  pinned", muted})
	}
	switch live := m.live[ws.Path]; {
	case ws.Missing:
		segs = append(segs, segment{"  missing", t.Dialog.TitleError})
	case live.State == ServerUnknown:
		segs = append(segs, segment{"  server not responding (unknown)", t.Section.Title})
	case live.State == ServerLive && live.Running+live.Waiting == 0:
		segs = append(segs, segment{"  idle", muted})
	case live.State == ServerLive:
		segs = append(segs, segment{fmt.Sprintf("  %d running · %d waiting", live.Running, live.Waiting), m.statusStyle(StatusRunning)})
	case !ws.LastAccessed.IsZero():
		segs = append(segs, segment{"  " + humanize.Time(ws.LastAccessed), muted})
	}
	return append(segs, segment{"  " + homedir.Short(ws.Path), muted})
}

func (m *Model) statusSegment(s SessionStatus) segment {
	text := map[SessionStatus]string{
		StatusUnknown: "unknown",
		StatusIdle:    "idle",
		StatusRunning: "running",
		StatusWaiting: "waiting",
	}[s]
	return segment{"  " + text, m.statusStyle(s)}
}

func (m *Model) statusStyle(s SessionStatus) lipgloss.Style {
	t := m.sty
	switch s {
	case StatusRunning:
		return lipgloss.NewStyle().Foreground(t.Resource.BusyIcon.GetForeground())
	case StatusWaiting:
		return lipgloss.NewStyle().Foreground(t.Status.WarnIndicator.GetBackground())
	}
	return t.Header.WorkingDir
}
