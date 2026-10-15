// Package home is the agent-cli start screen, in the spirit of "Open
// Recent": pick a workspace, then a session, and the caller runs the
// workspace TUI for that choice. Home never starts a server or opens a
// workspace database to draw its list: everything outside the process goes
// through a [Source], and only from tea.Cmds.
package home

import (
	"cmp"
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/crush/internal/ui/styles"
)

const (
	// RecentLimit is how many recent workspaces the list shows. It only
	// limits the display: the registry keeps every entry, and searching or
	// "Show all" reaches them.
	RecentLimit = 5
	// SessionLimit caps the sessions listed for one workspace.
	SessionLimit = 50
	// probeBudget bounds one probe of all servers, one session listing and one
	// load of the workspace registry.
	probeBudget = 3 * time.Second
	// probeInterval is the pause between probes of the servers while the
	// workspace list is shown, so a server that starts or stops (or a run that
	// ends) is reflected without leaving Home.
	probeInterval = 5 * time.Second
)

// Workspace is a registered workspace of this host.
type Workspace struct {
	Path string
	// Locations are where it was opened, newest first: a subdirectory shares
	// its repository's workspace but may keep its own data directory, so
	// sessions live in every location's DataDir.
	Locations    []Location
	LastAccessed time.Time
	Pinned       bool
	Missing      bool   // The directory no longer exists.
	Branch       string // Empty when unknown.
}

// Location is a directory a workspace was opened in, and its data directory.
type Location struct {
	Cwd     string
	DataDir string
}

// ServerState is what probing a workspace's server found.
type ServerState int

const (
	ServerOffline ServerState = iota // Nothing listens: no server.
	ServerLive                       // It answered; the counts are real.
	ServerUnknown                    // A socket that did not answer in time.
)

// Live is the probed state of a workspace's server.
type Live struct {
	State            ServerState
	Running, Waiting int
}

// SessionStatus is the run state of a listed session.
type SessionStatus int

const (
	// StatusUnknown is any session not read from a live server.
	StatusUnknown SessionStatus = iota
	StatusIdle
	StatusRunning
	StatusWaiting // Waiting for a permission or question answer.
)

// Session is one row of the session list.
type Session struct {
	ID        string
	Title     string
	UpdatedAt time.Time
	Status    SessionStatus
	// Dir is where the TUI must open to find the session (its location's
	// Cwd); empty means the workspace path.
	Dir string
}

// Source is everything Home reads or changes outside the process. Home calls
// it only from tea.Cmds, never from Update or View.
type Source interface {
	// Workspaces lists this host's registry, most recent first.
	Workspaces(ctx context.Context) ([]Workspace, error)
	// Probe returns the server state of each path; missing paths are
	// offline. It must never start a server.
	Probe(ctx context.Context, paths []string) map[string]Live
	// Sessions lists up to SessionLimit sessions of ws, newest first.
	Sessions(ctx context.Context, ws Workspace) ([]Session, error)
	// Add registers an existing directory and returns its workspace.
	Add(ctx context.Context, dir string) (Workspace, error)
	// Pin and Remove change the registry only, never the workspace.
	Pin(path string, pinned bool) error
	Remove(path string) error
}

// Choice is what the user picked. The zero value means Home was quit.
type Choice struct {
	Dir       string
	SessionID string // Empty opens the TUI's landing screen.
}

type screen int

const (
	screenWorkspaces screen = iota
	screenSessions
	screenPath // Typing the path of another folder.
)

type rowKind int

const (
	rowHeader rowKind = iota // Section title or note; not selectable.
	rowWorkspace
	rowOpenCwd
	rowOpenPath
	rowShowAll
	rowNewSession
	rowSession
)

type row struct {
	kind rowKind
	text string // rowHeader only.
	ws   Workspace
	sess Session
}

func (r row) selectable() bool { return r.kind != rowHeader }

// isItem reports whether r is a workspace or session, as opposed to an action
// row: only items are worth following across rebuilds.
func (r row) isItem() bool { return r.kind == rowWorkspace || r.kind == rowSession }

// same reports whether a and b show the same thing, across rebuilds.
func (r row) same(o row) bool {
	return r.kind == o.kind && r.ws.Path == o.ws.Path && r.sess.ID == o.sess.ID
}

type (
	workspacesMsg struct {
		list []Workspace
		err  error
	}
	liveMsg     struct{ live map[string]Live }
	sessionsMsg struct {
		path string
		list []Session
		err  error
	}
	addedMsg struct {
		ws  Workspace
		err error
	}
	changedMsg struct{ err error }
	// probeTickMsg means it is time to probe the servers again.
	probeTickMsg struct{}
)

// Model is the Home program.
type Model struct {
	src           Source
	sty           *styles.Styles
	cwd           string
	width, height int

	screen screen
	input  textinput.Model // Search, or the path on screenPath.
	cursor int
	// anchored keeps the cursor on the item the user put it on across
	// rebuilds; otherwise it goes to the default row.
	anchored bool
	showAll  bool
	rows     []row
	note     string // Last error or hint.

	workspaces []Workspace
	loaded     bool
	probing    bool            // A probe is in flight.
	tickArmed  bool            // A probeTickMsg timer is outstanding.
	probeEvery time.Duration   // Pause between probes; 0 probes once per registry load.
	budget     time.Duration   // Bound of one registry load.
	live       map[string]Live // nil until the probe returns.

	ws             Workspace // The workspace whose sessions are listed.
	sessions       []Session
	sessionsLoaded bool

	choice   Choice
	quitting bool

	// frame caches the rendered screen; "" means the state changed since.
	frame     string
	cursorPos *tea.Cursor
	renders   int
}

// New returns Home for the registry of src; cwd backs "Open current folder".
func New(src Source, sty *styles.Styles, cwd string) *Model {
	in := textinput.New()
	in.SetVirtualCursor(false)
	in.SetStyles(sty.TextInput)
	in.Focus()
	m := &Model{src: src, sty: sty, cwd: cwd, input: in, probeEvery: probeInterval, budget: probeBudget}
	m.setSize(80, 24)
	m.setScreen(screenWorkspaces)
	m.rebuild()
	return m
}

// Run shows Home until the user picks something or quits.
func Run(ctx context.Context, src Source, sty *styles.Styles, cwd string) (Choice, error) {
	m := New(src, sty, cwd)
	if _, err := tea.NewProgram(m, tea.WithContext(ctx)).Run(); err != nil {
		return Choice{}, err
	}
	return m.Choice(), nil
}

// Choice returns the pick, or the zero Choice when Home was quit.
func (m *Model) Choice() Choice { return m.choice }

func (m *Model) Init() tea.Cmd { return m.loadWorkspaces }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.setSize(msg.Width, msg.Height)
	case workspacesMsg:
		if msg.err != nil {
			m.note = "Cannot read the workspace list: " + msg.err.Error()
			// A failed reload (a timeout, say) keeps the list on screen; only a
			// first load that failed shows the empty state.
			if !m.loaded {
				m.loaded, m.workspaces = true, nil
			}
		} else {
			m.loaded, m.workspaces = true, msg.list
		}
		if !m.probing {
			m.probing = true
			cmd = m.probe(m.workspaces)
		}
	case liveMsg:
		m.probing = false
		m.live = msg.live
		if m.live == nil {
			m.live = map[string]Live{}
		}
		cmd = m.armProbeTick()
	case probeTickMsg:
		// Nothing to redraw by itself: a probe's answer redraws when it lands.
		return m, m.onProbeTick()
	case sessionsMsg:
		if m.screen != screenSessions || msg.path != m.ws.Path {
			return m, nil // Stale: the user moved on.
		}
		m.sessions, m.sessionsLoaded = msg.list, true
		if msg.err != nil {
			m.note = "Cannot list sessions: " + msg.err.Error()
		}
	case addedMsg:
		if msg.err != nil {
			m.note = msg.err.Error()
			break
		}
		cmd = tea.Batch(m.openWorkspace(msg.ws), m.loadWorkspaces)
	case changedMsg:
		if msg.err != nil {
			m.note = msg.err.Error()
		}
		cmd = m.loadWorkspaces
	case tea.KeyPressMsg:
		cmd = m.handleKey(msg)
	case tea.PasteMsg:
		// Nội dung dán (bracketed paste) tới dưới dạng PasteMsg chứ không
		// phải phím: đưa vào ô nhập như gõ tay.
		m.note = ""
		m.updateInput(msg)
	default:
		return m, nil // No state change: keep the rendered frame.
	}
	m.rebuild()
	m.frame = ""
	return m, cmd
}

func (m *Model) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	m.note = ""
	switch msg.String() {
	case "ctrl+c":
		return m.quit(Choice{})
	case "esc":
		return m.back()
	case "up":
		m.move(-1)
		return nil
	case "down":
		m.move(1)
		return nil
	case "enter":
		return m.activate()
	case "ctrl+p":
		if r, ok := m.selected(); ok && r.kind == rowWorkspace {
			m.anchored = true // Follow the workspace into its new section.
			src, ws := m.src, r.ws
			return func() tea.Msg { return changedMsg{src.Pin(ws.Path, !ws.Pinned)} }
		}
		return nil
	case "ctrl+x":
		r, ok := m.selected()
		switch {
		case !ok || r.kind != rowWorkspace:
		case !r.ws.Missing:
			m.note = "Only a folder that no longer exists can be removed from the list."
		default:
			src, path := m.src, r.ws.Path
			return func() tea.Msg { return changedMsg{src.Remove(path)} }
		}
		return nil
	}
	m.updateInput(msg)
	return nil
}

// updateInput đưa phím hoặc nội dung dán vào ô nhập đang dùng (tìm kiếm
// hoặc đường dẫn).
func (m *Model) updateInput(msg tea.Msg) {
	prev := m.input.Value()
	m.input, _ = m.input.Update(msg)
	if m.input.Value() != prev {
		m.anchored = false // New results: start from the first one.
	}
}

func (m *Model) back() tea.Cmd {
	switch {
	case m.screen != screenWorkspaces:
		from := m.ws.Path
		m.setScreen(screenWorkspaces)
		m.rows = m.buildRows()
		m.cursor = slices.IndexFunc(m.rows, func(r row) bool { return r.kind == rowWorkspace && r.ws.Path == from })
		m.anchored = m.cursor >= 0
	case m.input.Value() != "" || m.showAll:
		m.input.Reset()
		m.showAll = false
		m.anchored = false
	default:
		return m.quit(Choice{})
	}
	return nil
}

func (m *Model) activate() tea.Cmd {
	if m.screen == screenPath {
		if dir := strings.TrimSpace(m.input.Value()); dir != "" {
			return m.add(dir)
		}
		return nil
	}
	r, ok := m.selected()
	if !ok {
		return nil
	}
	switch r.kind {
	case rowWorkspace:
		if r.ws.Missing {
			m.note = "Folder not found: " + r.ws.Path + " (ctrl+x removes it from the list)"
			return nil
		}
		return m.openWorkspace(r.ws)
	case rowOpenCwd:
		return m.add(m.cwd)
	case rowOpenPath:
		m.setScreen(screenPath)
	case rowShowAll:
		m.showAll = true
		m.anchored = false
	case rowNewSession:
		return m.quit(Choice{Dir: m.ws.Path})
	case rowSession:
		return m.quit(Choice{Dir: cmp.Or(r.sess.Dir, m.ws.Path), SessionID: r.sess.ID})
	}
	return nil
}

func (m *Model) quit(c Choice) tea.Cmd {
	m.choice, m.quitting = c, true
	return tea.Quit
}

func (m *Model) setSize(w, h int) {
	m.width, m.height = w, h
	m.input.SetWidth(max(10, w-4))
}

func (m *Model) setScreen(s screen) {
	m.screen = s
	m.input.Reset()
	m.anchored = false
	switch s {
	case screenWorkspaces:
		m.input.Placeholder = "Search workspaces"
	case screenSessions:
		m.input.Placeholder = "Search sessions"
	case screenPath:
		m.input.Placeholder = "Path of an existing folder"
	}
}

// openWorkspace shows the session list of ws.
func (m *Model) openWorkspace(ws Workspace) tea.Cmd {
	m.setScreen(screenSessions)
	m.ws, m.sessions, m.sessionsLoaded = ws, nil, false
	src := m.src
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), probeBudget)
		defer cancel()
		list, err := src.Sessions(ctx, ws)
		return sessionsMsg{path: ws.Path, list: list, err: err}
	}
}

// loadWorkspaces reads the registry within the load budget. The Source gets a
// context with that deadline, but it may stat paths on a dead mount and never
// look at it, so the wait itself is bounded too: a late answer is dropped and
// the goroutine ends whenever the Source returns.
func (m *Model) loadWorkspaces() tea.Msg {
	budget := cmp.Or(m.budget, probeBudget)
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	type result struct {
		list []Workspace
		err  error
	}
	src, answer := m.src, make(chan result, 1)
	go func() {
		list, err := src.Workspaces(ctx)
		answer <- result{list, err}
	}()
	select {
	case r := <-answer:
		return workspacesMsg{list: r.list, err: r.err}
	case <-ctx.Done():
		return workspacesMsg{err: fmt.Errorf("reading the registry took longer than %s", budget)}
	}
}

func (m *Model) probe(list []Workspace) tea.Cmd {
	paths := make([]string, len(list))
	for i, w := range list {
		paths[i] = w.Path
	}
	src := m.src
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), probeBudget)
		defer cancel()
		return liveMsg{src.Probe(ctx, paths)}
	}
}

// armProbeTick schedules the next probe, unless probing is off or a timer is
// already outstanding.
func (m *Model) armProbeTick() tea.Cmd {
	if m.probeEvery <= 0 || m.tickArmed {
		return nil
	}
	m.tickArmed = true
	return tea.Tick(m.probeEvery, func(time.Time) tea.Msg { return probeTickMsg{} })
}

// onProbeTick probes the servers again while the workspace list is shown and no
// probe is running. Off the list (sessions, path entry) it only keeps the timer
// going; a probe in flight re-arms it when its answer lands.
func (m *Model) onProbeTick() tea.Cmd {
	m.tickArmed = false
	switch {
	case m.probing:
		return nil
	case m.screen != screenWorkspaces || !m.loaded:
		return m.armProbeTick()
	}
	m.probing = true
	return m.probe(m.workspaces)
}

func (m *Model) add(dir string) tea.Cmd {
	src := m.src
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), probeBudget)
		defer cancel()
		ws, err := src.Add(ctx, dir)
		return addedMsg{ws: ws, err: err}
	}
}

func (m *Model) selected() (row, bool) {
	if m.cursor < 0 || m.cursor >= len(m.rows) || !m.rows[m.cursor].selectable() {
		return row{}, false
	}
	return m.rows[m.cursor], true
}

// move steps the cursor to the next selectable row in direction d.
func (m *Model) move(d int) {
	for i := m.cursor + d; i >= 0 && i < len(m.rows); i += d {
		if m.rows[i].selectable() {
			m.cursor, m.anchored = i, true
			return
		}
	}
}

// rebuild recomputes the rows. An anchored cursor stays on its item (by
// workspace path or session ID) while that is still listed, so a late probe
// or refresh never makes Enter pick another row; when the item is gone the
// cursor clamps to the nearest selectable row. Otherwise it goes to the
// latest session (resuming is the usual pick) or the first selectable row,
// and anchors there if that is an item.
func (m *Model) rebuild() {
	sel, ok := m.selected()
	prev := m.cursor
	m.rows = m.buildRows()
	if m.anchored && ok {
		if i := slices.IndexFunc(m.rows, sel.same); i >= 0 {
			m.cursor = i
			return
		}
		if i := m.nearestSelectable(prev); i >= 0 {
			m.cursor = i
			m.anchored = m.rows[i].isItem()
			return
		}
	}
	m.cursor = slices.IndexFunc(m.rows, func(r row) bool { return r.kind == rowSession })
	if m.cursor < 0 {
		m.cursor = slices.IndexFunc(m.rows, row.selectable)
	}
	m.anchored = m.cursor >= 0 && m.rows[m.cursor].isItem()
}

// nearestSelectable returns the selectable row closest to index at, the one
// above on a tie, or -1 when there is none.
func (m *Model) nearestSelectable(at int) int {
	at = min(max(at, 0), len(m.rows)-1)
	for d := 0; d < len(m.rows); d++ {
		for _, i := range [2]int{at - d, at + d} {
			if i >= 0 && i < len(m.rows) && m.rows[i].selectable() {
				return i
			}
		}
	}
	return -1
}

func (m *Model) buildRows() []row {
	switch m.screen {
	case screenSessions:
		return m.sessionRows()
	case screenPath:
		return nil
	}
	return m.workspaceRows()
}

func (m *Model) workspaceRows() []row {
	var rows []row
	section := func(title string, list []Workspace) {
		if len(list) == 0 {
			return
		}
		rows = append(rows, row{kind: rowHeader, text: title})
		for _, w := range list {
			rows = append(rows, row{kind: rowWorkspace, ws: w})
		}
	}
	q := query(m.input.Value())
	switch {
	case !m.loaded:
		rows = append(rows, row{kind: rowHeader, text: "Loading workspaces…"})
	case q != "" || m.showAll:
		var all []Workspace
		for _, w := range m.workspaces {
			if matches(q, filepath.Base(w.Path), w.Path) {
				all = append(all, w)
			}
		}
		section("All workspaces", all)
	case len(m.workspaces) == 0:
		rows = append(rows, row{kind: rowHeader, text: "No workspaces yet"})
	default:
		// Each workspace shows once, in the first section it belongs to.
		var active, pinned, recent []Workspace
		for _, w := range m.workspaces {
			switch {
			case m.live[w.Path].State != ServerOffline:
				active = append(active, w)
			case w.Pinned:
				pinned = append(pinned, w)
			default:
				recent = append(recent, w)
			}
		}
		if m.live == nil {
			rows = append(rows, row{kind: rowHeader, text: "Active · checking servers…"})
		}
		section("Active", active)
		section("Pinned", pinned)
		section("Recent", recent[:min(len(recent), RecentLimit)])
	}
	rows = append(rows, row{kind: rowOpenCwd}, row{kind: rowOpenPath})
	if q == "" && !m.showAll {
		rows = append(rows, row{kind: rowShowAll})
	}
	return rows
}

func (m *Model) sessionRows() []row {
	rows := []row{{kind: rowNewSession}}
	if !m.sessionsLoaded {
		return append(rows, row{kind: rowHeader, text: "Loading sessions…"})
	}
	if len(m.sessions) == 0 {
		return append(rows, row{kind: rowHeader, text: "No sessions yet"})
	}
	q := query(m.input.Value())
	for _, s := range m.sessions {
		if matches(q, s.Title, s.ID) {
			rows = append(rows, row{kind: rowSession, sess: s})
		}
	}
	return rows
}

func query(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// matches reports whether any field contains q, ignoring case.
func matches(q string, fields ...string) bool {
	return q == "" || slices.ContainsFunc(fields, func(f string) bool {
		return strings.Contains(strings.ToLower(f), q)
	})
}
