package home

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/ui/styles"
)

// fakeSource is an in-memory registry and server prober. Commands run
// synchronously in these tests, so it needs no locking.
type fakeSource struct {
	workspaces []Workspace
	live       map[string]Live
	sessions   map[string][]Session
	addErr     error

	probes   int
	listed   []string // Sessions calls, by workspace path.
	added    []string
	pins     []string // "path=true|false"
	removed  []string
	wsLoads  int
	probeArg []string
}

func (f *fakeSource) Workspaces(context.Context) ([]Workspace, error) {
	f.wsLoads++
	return slices.Clone(f.workspaces), nil
}

func (f *fakeSource) Probe(_ context.Context, paths []string) map[string]Live {
	f.probes++
	f.probeArg = paths
	return f.live
}

func (f *fakeSource) Sessions(_ context.Context, ws Workspace) ([]Session, error) {
	f.listed = append(f.listed, ws.Path)
	return f.sessions[ws.Path], nil
}

func (f *fakeSource) Add(_ context.Context, dir string) (Workspace, error) {
	f.added = append(f.added, dir)
	if f.addErr != nil {
		return Workspace{}, f.addErr
	}
	ws := Workspace{Path: dir}
	f.workspaces = append([]Workspace{ws}, f.workspaces...)
	return ws, nil
}

func (f *fakeSource) Pin(path string, pinned bool) error {
	f.pins = append(f.pins, fmt.Sprintf("%s=%t", path, pinned))
	for i := range f.workspaces {
		if f.workspaces[i].Path == path {
			f.workspaces[i].Pinned = pinned
		}
	}
	return nil
}

func (f *fakeSource) Remove(path string) error {
	f.removed = append(f.removed, path)
	f.workspaces = slices.DeleteFunc(f.workspaces, func(w Workspace) bool { return w.Path == path })
	return nil
}

// tenWorkspaces is a registry of /w/a../w/j, most recent first.
func tenWorkspaces() []Workspace {
	var ws []Workspace
	for _, n := range "abcdefghij" {
		ws = append(ws, Workspace{Path: "/w/" + string(n)})
	}
	return ws
}

// drive runs cmd and feeds every message it leads to back into m, like the
// Bubble Tea loop. It reports whether the program asked to quit.
func drive(m *Model, cmd tea.Cmd) (quit bool) {
	queue := []tea.Cmd{cmd}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		switch msg := c().(type) {
		case nil:
		case tea.BatchMsg:
			queue = append(queue, msg...)
		case tea.QuitMsg:
			quit = true
		default:
			_, next := m.Update(msg)
			queue = append(queue, next)
		}
	}
	return quit
}

func newTestModel(src *fakeSource) *Model {
	sty := styles.CharmtonePantera()
	m := New(src, &sty, "/home/u/here")
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	drive(m, m.Init())
	return m
}

func press(m *Model, k tea.KeyPressMsg) bool {
	_, cmd := m.Update(k)
	return drive(m, cmd)
}

func typeText(m *Model, s string) {
	for _, r := range s {
		press(m, tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

var (
	keyEnter = tea.KeyPressMsg{Code: tea.KeyEnter}
	keyEsc   = tea.KeyPressMsg{Code: tea.KeyEscape}
	keyDown  = tea.KeyPressMsg{Code: tea.KeyDown}
	keyPin   = tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl}
	keyDel   = tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl}
)

func screenText(m *Model) string {
	return ansi.Strip(m.View().Content)
}

// sections returns the workspace list as "Header: path path ..." lines.
func sections(m *Model) []string {
	var out []string
	for _, r := range m.rows {
		switch r.kind {
		case rowHeader:
			out = append(out, r.text+":")
		case rowWorkspace:
			out[len(out)-1] += " " + r.ws.Path
		}
	}
	return out
}

// selectRow moves the cursor to the first row matching ok.
func selectRow(t *testing.T, m *Model, ok func(row) bool) {
	t.Helper()
	i := slices.IndexFunc(m.rows, ok)
	require.GreaterOrEqual(t, i, 0, "row not found")
	m.cursor = i
}

func isPath(p string) func(row) bool {
	return func(r row) bool { return r.kind == rowWorkspace && r.ws.Path == p }
}

func isKind(k rowKind) func(row) bool {
	return func(r row) bool { return r.kind == k }
}

func TestSectionsActivePinnedRecent(t *testing.T) {
	t.Parallel()

	ws := tenWorkspaces()
	ws[1].Pinned = true // b
	src := &fakeSource{workspaces: ws, live: map[string]Live{
		"/w/c": {State: ServerLive, Running: 2, Waiting: 1},
		"/w/d": {State: ServerUnknown},
	}}
	m := newTestModel(src)

	require.Equal(t, []string{
		"Active: /w/c /w/d",
		"Pinned: /w/b",
		"Recent: /w/a /w/e /w/f /w/g /w/h",
	}, sections(m), "a workspace shows once; recent stops at five")
	require.Equal(t, 1, src.probes)
	require.Len(t, src.probeArg, 10, "every registered workspace is probed")
	require.Empty(t, src.listed, "no workspace database is opened to draw the list")

	txt := screenText(m)
	require.Contains(t, txt, "2 running")
	require.Contains(t, txt, "1 waiting")
	require.Contains(t, txt, "Open current folder")
	require.Contains(t, txt, "Open other folder")
	require.Contains(t, txt, "Show all")

	// The display limit is not a registry limit: show all lists every entry.
	selectRow(t, m, isKind(rowShowAll))
	press(m, keyEnter)
	require.Equal(t, []string{"All workspaces: /w/a /w/b /w/c /w/d /w/e /w/f /w/g /w/h /w/i /w/j"}, sections(m))
	require.Len(t, src.workspaces, 10)
	require.Empty(t, src.removed)
}

func TestUnreachableServerIsUnknownNotIdle(t *testing.T) {
	t.Parallel()

	src := &fakeSource{workspaces: tenWorkspaces()[:2], live: map[string]Live{"/w/a": {State: ServerUnknown}}}
	sty := styles.CharmtonePantera()
	m := New(src, &sty, "/home/u/here")
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Registry loaded, probe not back yet: nothing is called idle or offline.
	_, _ = m.Update(workspacesMsg{list: slices.Clone(src.workspaces)})
	require.Contains(t, screenText(m), "checking servers")

	_, _ = m.Update(liveMsg{live: src.live})
	txt := screenText(m)
	require.NotContains(t, txt, "checking servers")
	line := lineWith(t, txt, "/w/a")
	require.Contains(t, line, "unknown")
	require.NotContains(t, line, "idle")
}

func lineWith(t *testing.T, txt, s string) string {
	t.Helper()
	for line := range strings.SplitSeq(txt, "\n") {
		if strings.Contains(line, s) {
			return line
		}
	}
	t.Fatalf("no line with %q in:\n%s", s, txt)
	return ""
}

func TestFilterSearchesWholeRegistryByNameAndPath(t *testing.T) {
	t.Parallel()

	ws := tenWorkspaces()
	ws[9].Path = "/srv/zeta-api" // Tenth: past the five recent rows.
	src := &fakeSource{workspaces: ws}
	m := newTestModel(src)
	require.NotContains(t, screenText(m), "zeta-api")

	typeText(m, "zeta")
	require.Equal(t, []string{"All workspaces: /srv/zeta-api"}, sections(m))

	press(m, keyEsc) // Clears the search instead of quitting.
	require.Equal(t, Choice{}, m.Choice())
	require.False(t, m.quitting)
	typeText(m, "srv")
	require.Equal(t, []string{"All workspaces: /srv/zeta-api"}, sections(m), "matches the path too")
}

func TestMissingPathCanBeRemovedOnly(t *testing.T) {
	t.Parallel()

	ws := tenWorkspaces()[:3]
	ws[1].Missing = true
	src := &fakeSource{workspaces: ws}
	m := newTestModel(src)
	require.Contains(t, lineWith(t, screenText(m), "/w/b"), "missing")

	selectRow(t, m, isPath("/w/b"))
	require.False(t, press(m, keyEnter), "a missing folder does not open")
	require.Empty(t, src.listed)
	require.Contains(t, screenText(m), "not found")

	selectRow(t, m, isPath("/w/a"))
	press(m, keyDel)
	require.Empty(t, src.removed, "an existing folder stays in the list")

	selectRow(t, m, isPath("/w/b"))
	press(m, keyDel)
	require.Equal(t, []string{"/w/b"}, src.removed)
	require.Equal(t, []string{"Recent: /w/a /w/c"}, sections(m), "the list is reloaded")
}

func TestPinToggles(t *testing.T) {
	t.Parallel()

	src := &fakeSource{workspaces: tenWorkspaces()[:3]}
	m := newTestModel(src)

	selectRow(t, m, isPath("/w/c"))
	press(m, keyPin)
	require.Equal(t, []string{"/w/c=true"}, src.pins)
	require.Equal(t, []string{"Pinned: /w/c", "Recent: /w/a /w/b"}, sections(m))

	selectRow(t, m, isPath("/w/c"))
	press(m, keyPin)
	require.Equal(t, []string{"/w/c=true", "/w/c=false"}, src.pins)
	require.Equal(t, []string{"Recent: /w/a /w/b /w/c"}, sections(m))
}

func TestWorkspaceThenSession(t *testing.T) {
	t.Parallel()

	src := &fakeSource{
		workspaces: tenWorkspaces()[:2],
		live:       map[string]Live{"/w/b": {State: ServerLive, Running: 1}},
		sessions: map[string][]Session{"/w/b": {
			{ID: "s1", Title: "Fix login", Status: StatusRunning},
			{ID: "s2", Title: "Refactor db", Status: StatusWaiting},
			{ID: "s3", Title: "Old work", Status: StatusIdle},
		}},
	}
	m := newTestModel(src)

	selectRow(t, m, isPath("/w/b"))
	require.False(t, press(m, keyEnter))
	require.Equal(t, []string{"/w/b"}, src.listed, "only the chosen workspace is listed")
	txt := screenText(m)
	require.Contains(t, txt, "New session")
	require.Contains(t, lineWith(t, txt, "Fix login"), "running")
	require.Contains(t, lineWith(t, txt, "Refactor db"), "waiting")
	require.Contains(t, lineWith(t, txt, "Old work"), "idle")

	typeText(m, "refac")
	sess := slices.IndexFunc(m.rows, isKind(rowSession))
	require.Equal(t, "s2", m.rows[sess].sess.ID)
	require.Len(t, slices.DeleteFunc(slices.Clone(m.rows), func(r row) bool { return r.kind != rowSession }), 1)

	// Esc goes back to the workspace list, then quits Home.
	require.False(t, press(m, keyEsc))
	require.Contains(t, screenText(m), "Open current folder")
	require.True(t, press(m, keyEsc))
	require.Equal(t, Choice{}, m.Choice())

	// Picking a running session only opens it.
	m = newTestModel(src)
	selectRow(t, m, isPath("/w/b"))
	press(m, keyEnter)
	selectRow(t, m, func(r row) bool { return r.kind == rowSession && r.sess.ID == "s1" })
	require.True(t, press(m, keyEnter))
	require.Equal(t, Choice{Dir: "/w/b", SessionID: "s1"}, m.Choice())
}

func TestNewSessionOpensLanding(t *testing.T) {
	t.Parallel()

	src := &fakeSource{workspaces: tenWorkspaces()[:1]}
	m := newTestModel(src)
	selectRow(t, m, isPath("/w/a"))
	press(m, keyEnter)
	require.Contains(t, screenText(m), "No sessions yet")
	selectRow(t, m, isKind(rowNewSession))
	require.True(t, press(m, keyEnter))
	require.Equal(t, Choice{Dir: "/w/a"}, m.Choice(), "no session ID: the TUI opens its landing screen")
}

func TestOfflineSessionsHaveUnknownStatus(t *testing.T) {
	t.Parallel()

	src := &fakeSource{
		workspaces: tenWorkspaces()[:1],
		sessions:   map[string][]Session{"/w/a": {{ID: "s1", Title: "From the database"}}},
	}
	m := newTestModel(src)
	selectRow(t, m, isPath("/w/a"))
	press(m, keyEnter)
	line := lineWith(t, screenText(m), "From the database")
	require.Contains(t, line, "unknown")
	require.NotContains(t, line, "idle")
}

func TestOpenOtherFolder(t *testing.T) {
	t.Parallel()

	src := &fakeSource{workspaces: tenWorkspaces()[:1], addErr: errors.New("not a directory: /nope")}
	m := newTestModel(src)
	selectRow(t, m, isKind(rowOpenPath))
	press(m, keyEnter)
	typeText(m, "/nope")
	require.False(t, press(m, keyEnter))
	require.Equal(t, []string{"/nope"}, src.added)
	require.Contains(t, screenText(m), "not a directory")
	require.Equal(t, screenPath, m.screen, "stays on the path input")

	src.addErr = nil
	press(m, keyEsc)
	require.Equal(t, screenWorkspaces, m.screen)
	selectRow(t, m, isKind(rowOpenPath))
	press(m, keyEnter)
	typeText(m, "/srv/new")
	press(m, keyEnter)
	require.Equal(t, screenSessions, m.screen)
	require.Equal(t, "/srv/new", m.ws.Path)
	require.Equal(t, []string{"/srv/new"}, src.listed)
}

func TestOpenCurrentFolder(t *testing.T) {
	t.Parallel()

	src := &fakeSource{}
	m := newTestModel(src)
	require.Contains(t, screenText(m), "/home/u/here")
	selectRow(t, m, isKind(rowOpenCwd))
	press(m, keyEnter)
	require.Equal(t, []string{"/home/u/here"}, src.added)
	require.Equal(t, screenSessions, m.screen)
}

func TestRendersOnlyOnStateChange(t *testing.T) {
	t.Parallel()

	m := newTestModel(&fakeSource{workspaces: tenWorkspaces()})
	_ = m.View()
	n := m.renders
	_ = m.View()
	_, _ = m.Update(struct{}{})
	_ = m.View()
	require.Equal(t, n, m.renders, "no state change, no render")

	press(m, keyDown)
	_ = m.View()
	require.Equal(t, n+1, m.renders)
}

// A session kept in a subdirectory's data dir opens there, so the TUI finds
// the database that holds it.
func TestSessionOpensWhereItLives(t *testing.T) {
	t.Parallel()

	src := &fakeSource{
		workspaces: tenWorkspaces()[:1],
		sessions: map[string][]Session{"/w/a": {
			{ID: "s1", Title: "In sub", Dir: "/w/a/sub"},
			{ID: "s2", Title: "At root"},
		}},
	}
	m := newTestModel(src)
	selectRow(t, m, isPath("/w/a"))
	press(m, keyEnter)
	selectRow(t, m, func(r row) bool { return r.kind == rowSession && r.sess.ID == "s1" })
	require.True(t, press(m, keyEnter))
	require.Equal(t, Choice{Dir: "/w/a/sub", SessionID: "s1"}, m.Choice())

	m = newTestModel(src)
	selectRow(t, m, isPath("/w/a"))
	press(m, keyEnter)
	selectRow(t, m, func(r row) bool { return r.kind == rowSession && r.sess.ID == "s2" })
	require.True(t, press(m, keyEnter))
	require.Equal(t, Choice{Dir: "/w/a", SessionID: "s2"}, m.Choice())
}

// selectedPath is the workspace path under the cursor, "" when it is not a
// workspace row.
func selectedPath(m *Model) string {
	r, _ := m.selected()
	return r.ws.Path
}

// TestCursorStaysOnItemWhenProbeInsertsActiveRows: the probe lands after the
// user already sees (and may press Enter on) a Recent row; the Active rows
// it inserts above must not move the cursor to another workspace.
func TestCursorStaysOnItemWhenProbeInsertsActiveRows(t *testing.T) {
	t.Parallel()

	for _, moved := range []bool{false, true} {
		t.Run(fmt.Sprintf("moved=%t", moved), func(t *testing.T) {
			t.Parallel()

			src := &fakeSource{}
			m := New(src, func() *styles.Styles { s := styles.CharmtonePantera(); return &s }(), "/home/u/here")
			m.Update(workspacesMsg{list: tenWorkspaces()[:5]})
			want := "/w/a" // The default cursor: first workspace.
			if moved {
				press(m, keyDown)
				press(m, keyDown)
				want = "/w/c"
			}
			require.Equal(t, want, selectedPath(m))

			m.Update(liveMsg{live: map[string]Live{
				"/w/d": {State: ServerLive, Running: 1},
				"/w/e": {State: ServerLive, Running: 1},
			}})
			require.Equal(t, []string{"Active: /w/d /w/e", "Recent: /w/a /w/b /w/c"}, sections(m))
			require.Equal(t, want, selectedPath(m), "the cursor follows its item, not its index")
		})
	}
}

// TestCursorClampsWhenItemDisappears: the registry refresh drops the selected
// workspace; the cursor lands on the row now in its place, not at the top.
func TestCursorClampsWhenItemDisappears(t *testing.T) {
	t.Parallel()

	src := &fakeSource{workspaces: tenWorkspaces()[:4]}
	m := newTestModel(src)
	selectRow(t, m, isPath("/w/b"))
	press(m, keyDown) // Anchor on /w/c.
	require.Equal(t, "/w/c", selectedPath(m))

	m.Update(workspacesMsg{list: slices.DeleteFunc(tenWorkspaces()[:4], func(w Workspace) bool { return w.Path == "/w/c" })})
	require.Equal(t, "/w/d", selectedPath(m), "nearest valid row")
}

// TestSessionListRefreshKeepsSelectedSession: the session list is reloaded
// with a new session on top; the cursor stays on the same session ID.
func TestSessionListRefreshKeepsSelectedSession(t *testing.T) {
	t.Parallel()

	src := &fakeSource{
		workspaces: tenWorkspaces()[:1],
		sessions: map[string][]Session{"/w/a": {
			{ID: "s1", Title: "one"}, {ID: "s2", Title: "two"}, {ID: "s3", Title: "three"},
		}},
	}
	m := newTestModel(src)
	selectRow(t, m, isPath("/w/a"))
	press(m, keyEnter)
	press(m, keyDown) // The default cursor is the first session.
	require.Equal(t, "s2", m.rows[m.cursor].sess.ID)

	m.Update(sessionsMsg{path: "/w/a", list: []Session{
		{ID: "s0", Title: "zero"}, {ID: "s1", Title: "one"}, {ID: "s2", Title: "two"}, {ID: "s3", Title: "three"},
	}})
	require.Equal(t, "s2", m.rows[m.cursor].sess.ID)

	m.Update(sessionsMsg{path: "/w/a", list: []Session{{ID: "s1", Title: "one"}, {ID: "s3", Title: "three"}}})
	_, ok := m.selected()
	require.True(t, ok, "the removed session leaves the cursor on a selectable row")
	require.Equal(t, "s3", m.rows[m.cursor].sess.ID, "nearest valid row")
}

// TestFilterKeepsWorkingAcrossRefresh: a registry refresh during a search
// keeps the selected result, and typing still narrows from the first result.
func TestFilterKeepsWorkingAcrossRefresh(t *testing.T) {
	t.Parallel()

	src := &fakeSource{workspaces: tenWorkspaces()}
	m := newTestModel(src)
	typeText(m, "/w/")
	press(m, keyDown)
	press(m, keyDown)
	require.Equal(t, "/w/c", selectedPath(m))

	m.Update(workspacesMsg{list: append([]Workspace{{Path: "/w/new"}}, tenWorkspaces()...)})
	require.Equal(t, "/w/c", selectedPath(m), "the refresh keeps the selected result")

	typeText(m, "g")
	require.Equal(t, "/w/g", selectedPath(m), "typing narrows the results from the first one")
}
