package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/projects"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/ui/home"
)

// rootFlagsCopy returns a fresh flag set with the same names, shorthands and
// types as the root command, so tests can parse without touching rootCmd.
func rootFlagsCopy(t *testing.T) *pflag.FlagSet {
	t.Helper()
	fs := pflag.NewFlagSet("crush", pflag.ContinueOnError)
	add := func(f *pflag.Flag) {
		if fs.Lookup(f.Name) != nil {
			return
		}
		switch f.Value.Type() {
		case "bool":
			fs.BoolP(f.Name, f.Shorthand, false, "")
		case "string":
			fs.StringP(f.Name, f.Shorthand, "", "")
		case "stringSlice":
			fs.StringSliceP(f.Name, f.Shorthand, nil, "")
		default:
			t.Fatalf("flag %s: unhandled type %s", f.Name, f.Value.Type())
		}
	}
	rootCmd.PersistentFlags().VisitAll(add)
	rootCmd.Flags().VisitAll(add)
	return fs
}

func TestHomeRouting(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		args []string
		tty  bool
		home bool
	}{
		{nil, true, true},
		{[]string{"--debug"}, true, true},
		{[]string{"-d"}, true, true},
		{nil, false, false}, // Not a terminal: the TUI as before.
		{[]string{"-c", "/x"}, true, false},
		{[]string{"--cwd=/x"}, true, false},
		{[]string{"-s", "abc"}, true, false},
		{[]string{"--session", "abc"}, true, false},
		{[]string{"-C"}, true, false},
		{[]string{"--continue"}, true, false},
		{[]string{"-y"}, true, false},
		{[]string{"--yolo"}, true, false},
		{[]string{"-D", "/d"}, true, false},
		{[]string{"--host", "unix:///x.sock"}, true, false},
		{[]string{"-H", "unix:///x.sock"}, true, false},
		{[]string{"--channels", "a:b"}, true, false},
	} {
		fs := rootFlagsCopy(t)
		require.NoError(t, fs.Parse(tc.args), "%v", tc.args)
		require.Equal(t, tc.home, shouldRunHome(fs, fs.Args(), tc.tty), "%v tty=%v", tc.args, tc.tty)
	}
}

// Subcommands are dispatched by cobra and never reach the root command, so
// they never show Home.
func TestSubcommandsBypassHome(t *testing.T) {
	t.Parallel()

	for _, sub := range []string{"run", "logs", "models", "login", "ps", "send", "keys", "server", "projects", "session", "stats"} {
		c, _, err := rootCmd.Find([]string{sub})
		require.NoError(t, err, sub)
		require.NotSame(t, rootCmd, c, sub)
		require.Equal(t, sub, c.Name())
	}
}

func TestHomeLoopExitCodes(t *testing.T) {
	t.Parallel()

	pick := func(choices ...home.Choice) (func() (home.Choice, error), *int) {
		n := 0
		return func() (home.Choice, error) {
			n++
			return choices[min(n, len(choices))-1], nil
		}, &n
	}
	run := func(codes ...int) (func(home.Choice) (int, error), *[]home.Choice) {
		var ran []home.Choice
		return func(c home.Choice) (int, error) {
			ran = append(ran, c)
			return codes[len(ran)-1], nil
		}, &ran
	}
	a, b := home.Choice{Dir: "/a"}, home.Choice{Dir: "/b", SessionID: "s1"}

	// Back to Home twice, then a normal exit ends agent-cli.
	p, picks := pick(a, b, a)
	r, ran := run(homeExitCode, homeExitCode, 0)
	code, err := homeLoop(p, r)
	require.NoError(t, err)
	require.Zero(t, code)
	require.Equal(t, 3, *picks, "Home is shown again after each exit code 10")
	require.Equal(t, []home.Choice{a, b, a}, *ran)

	// Any other status ends agent-cli with that status.
	p, picks = pick(a, a)
	r, _ = run(3)
	code, err = homeLoop(p, r)
	require.NoError(t, err)
	require.Equal(t, 3, code)
	require.Equal(t, 1, *picks)

	// Quitting Home runs nothing.
	p, _ = pick(home.Choice{})
	r, ran = run()
	code, err = homeLoop(p, r)
	require.NoError(t, err)
	require.Zero(t, code)
	require.Empty(t, *ran)

	// A failure to start the workspace ends agent-cli with that error.
	boom := errors.New("boom")
	p, _ = pick(a)
	code, err = homeLoop(p, func(home.Choice) (int, error) { return 1, boom })
	require.ErrorIs(t, err, boom)
	require.Equal(t, 1, code)
}

func TestHomeChildCmd(t *testing.T) {
	t.Parallel()

	c := homeChildCmd("/opt/agent-cli", home.Choice{Dir: "/w/a", SessionID: "s1"}, true)
	require.Equal(t, "/opt/agent-cli", c.Path)
	require.Equal(t, []string{"/opt/agent-cli", "--cwd", "/w/a", "--session", "s1", "--debug"}, c.Args)
	require.Contains(t, c.Env, homeChildEnv+"=1")
	require.Same(t, os.Stdin, c.Stdin)
	require.Same(t, os.Stdout, c.Stdout)

	c = homeChildCmd("/opt/agent-cli", home.Choice{Dir: "/w/a"}, false)
	require.Equal(t, []string{"/opt/agent-cli", "--cwd", "/w/a"}, c.Args, "no session: the landing screen")
}

func TestRunChildExitCode(t *testing.T) {
	t.Parallel()

	sh := func(script string) (int, error) {
		c := homeChildCmd("/bin/sh", home.Choice{Dir: "/"}, false)
		c.Args = []string{"/bin/sh", "-c", script}
		c.Stdin, c.Stdout, c.Stderr = nil, nil, nil
		return runChild(c)
	}
	code, err := sh("exit 10")
	require.NoError(t, err)
	require.Equal(t, homeExitCode, code)
	code, err = sh("exit 0")
	require.NoError(t, err)
	require.Zero(t, code)
	code, err = sh("kill -9 $$")
	require.NoError(t, err)
	require.Equal(t, 1, code, "killed by a signal is a failure, not Home")
}

// shortTempDir returns a directory short enough for unix socket paths.
func shortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "h2")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func sockFor(dir, path string) string {
	return filepath.Join(dir, "agent-cli-"+workspaceKey(path)+".sock")
}

// fakeServer serves the read-only endpoints Home uses for one workspace, and
// counts the per-session status probes.
func fakeServer(t *testing.T, sock string, ws proto.Workspace, sessions []proto.Session, waiting map[string]bool) *atomic.Int32 {
	t.Helper()
	var infos atomic.Int32
	mux := http.NewServeMux()
	reply := func(w http.ResponseWriter, v any) { _ = json.NewEncoder(w).Encode(v) }
	mux.HandleFunc("GET /v1/health", func(http.ResponseWriter, *http.Request) {})
	mux.HandleFunc("GET /v1/workspaces", func(w http.ResponseWriter, _ *http.Request) {
		ws.ID = "w1"
		reply(w, []proto.Workspace{ws})
	})
	mux.HandleFunc("GET /v1/workspaces/w1/sessions", func(w http.ResponseWriter, _ *http.Request) { reply(w, sessions) })
	mux.HandleFunc("GET /v1/workspaces/w1/agent/sessions/{sid}", func(w http.ResponseWriter, r *http.Request) {
		infos.Add(1)
		sid := r.PathValue("sid")
		reply(w, proto.AgentSession{Session: proto.Session{ID: sid}, IsBusy: true, PendingPrompt: waiting[sid]})
	})
	l, err := net.Listen("unix", sock)
	require.NoError(t, err)
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: time.Second}
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(func() { _ = srv.Close() })
	return &infos
}

func TestProbeLiveStates(t *testing.T) {
	t.Parallel()

	dir := shortTempDir(t)
	refused, silent, live, none := "/w/refused", "/w/silent", "/w/live", "/w/none"

	// A socket file nobody listens on: the server is gone.
	l, err := net.Listen("unix", sockFor(dir, refused))
	require.NoError(t, err)
	l.(*net.UnixListener).SetUnlinkOnClose(false)
	require.NoError(t, l.Close())

	// A listener that never answers: maybe a busy server, never "idle".
	hung, err := net.Listen("unix", sockFor(dir, silent))
	require.NoError(t, err)
	t.Cleanup(func() { hung.Close() })
	go func() {
		for {
			c, err := hung.Accept()
			if err != nil {
				return
			}
			t.Cleanup(func() { c.Close() })
		}
	}()

	infos := fakeServer(t, sockFor(dir, live), proto.Workspace{Path: live}, []proto.Session{
		{ID: "s1", IsBusy: true, UpdatedAt: 3},
		{ID: "s2", IsBusy: true, UpdatedAt: 2},
		{ID: "s3", UpdatedAt: 1},
	}, map[string]bool{"s2": true})

	hosts := []string{
		"unix://" + sockFor(dir, refused),
		"unix://" + sockFor(dir, silent),
		"unix://" + sockFor(dir, live),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	start := time.Now()
	got := probeLive(ctx, hosts, []string{refused, silent, live, none})
	require.Less(t, time.Since(start), 2*time.Second, "bounded by the budget")

	require.Equal(t, home.ServerOffline, got[refused].State)
	require.Equal(t, home.ServerUnknown, got[silent].State)
	require.Equal(t, home.Live{State: home.ServerLive, Running: 1, Waiting: 1}, got[live])
	require.Equal(t, home.ServerOffline, got[none].State)
	require.Equal(t, int32(2), infos.Load(), "only busy sessions are asked whether they wait")

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 3, "no server or socket was started")
}

func TestHomeSessionsFromLiveServer(t *testing.T) {
	t.Parallel()

	dir := shortTempDir(t)
	never := filepath.Join(t.TempDir(), "never-opened")
	ws := home.Workspace{Path: "/w/live", Locations: []home.Location{{Cwd: "/w/live", DataDir: never}}}
	fakeServer(t, sockFor(dir, ws.Path), proto.Workspace{Path: ws.Path, DataDir: never}, []proto.Session{
		{ID: "s1", Title: "Fix", IsBusy: true, UpdatedAt: 300},
		{ID: "s2", Title: "Plan", IsBusy: true, UpdatedAt: 200},
		{ID: "s3", Title: "Done", UpdatedAt: 100},
	}, map[string]bool{"s2": true})

	list, err := listHomeSessions(t.Context(), []string{"unix://" + sockFor(dir, ws.Path)}, ws)
	require.NoError(t, err)
	require.Equal(t, []home.Session{
		{ID: "s1", Title: "Fix", UpdatedAt: time.Unix(300, 0), Status: home.StatusRunning, Dir: "/w/live"},
		{ID: "s2", Title: "Plan", UpdatedAt: time.Unix(200, 0), Status: home.StatusWaiting, Dir: "/w/live"},
		{ID: "s3", Title: "Done", UpdatedAt: time.Unix(100, 0), Status: home.StatusIdle, Dir: "/w/live"},
	}, list)
	require.NoDirExists(t, never, "the database the server holds is not opened")
}

type dbRow struct {
	id, parent string
	updated    int64
}

// makeSessionDB creates a migrated workspace database holding rows.
func makeSessionDB(t *testing.T, dataDir string, rows ...dbRow) {
	t.Helper()
	conn, err := db.Connect(t.Context(), dataDir)
	require.NoError(t, err)
	for _, r := range rows {
		var p any
		if r.parent != "" {
			p = r.parent
		}
		_, err := conn.ExecContext(t.Context(),
			`INSERT INTO sessions (id, parent_session_id, title, message_count, prompt_tokens, completion_tokens, cost, updated_at, created_at)
			 VALUES (?, ?, ?, 0, 0, 0, 0, ?, ?)`, r.id, p, "title "+r.id, r.updated, r.updated)
		require.NoError(t, err)
	}
	require.NoError(t, db.Release(dataDir))
}

func TestHomeSessionsFromDatabaseReadOnly(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	rows := []dbRow{{id: "child", parent: "sA", updated: 5000}} // Newest, but a sub-agent session.
	for i := range 60 {
		rows = append(rows, dbRow{id: "s" + string(rune('A'+i)), updated: int64(1000 + i)})
	}
	makeSessionDB(t, dataDir, rows...)

	dbPath := filepath.Join(dataDir, "crush.db")
	before, err := os.ReadFile(dbPath)
	require.NoError(t, err)

	// No socket for this workspace: the database is read instead.
	list, err := listHomeSessions(t.Context(), nil, home.Workspace{Path: "/w/offline", Locations: []home.Location{{Cwd: "/w/offline", DataDir: dataDir}}})
	require.NoError(t, err)
	require.Len(t, list, home.SessionLimit)
	require.Equal(t, "s"+string(rune('A'+59)), list[0].ID, "newest first")
	require.False(t, slices.ContainsFunc(list, func(s home.Session) bool { return s.ID == "child" }))
	for _, s := range list {
		require.Equal(t, home.StatusUnknown, s.Status, "without a server the status is unknown, not idle")
	}
	bounded, err := dbSessions(t.Context(), dbPath, 7)
	require.NoError(t, err)
	require.Len(t, bounded, 7, "the read itself is bounded")
	after, err := os.ReadFile(dbPath)
	require.NoError(t, err)
	require.Equal(t, before, after, "the database is not written")

	// A workspace that never ran has no database: nothing is created.
	empty := t.TempDir()
	list, err = listHomeSessions(t.Context(), nil, home.Workspace{Path: "/w/new", Locations: []home.Location{{Cwd: "/w/new", DataDir: empty}}})
	require.NoError(t, err)
	require.Empty(t, list)
	require.NoFileExists(t, filepath.Join(empty, "crush.db"))
}

// The launcher runs every TUI in client/server mode; that path must feed the
// registry Home reads, with the absolute workspace path even for a relative
// --cwd. Not parallel: it sets the process cwd, env and the --host global.
func TestClientServerSetupRegistersWorkspace(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CRUSH_GLOBAL_DATA", filepath.Join(tmp, "data"))
	t.Setenv("CRUSH_GLOBAL_CONFIG", filepath.Join(tmp, "config"))
	t.Setenv("CRUSH_CACHE_DIR", filepath.Join(tmp, "cache"))
	t.Setenv("CRUSH_DISABLE_METRICS", "1")
	wsDir := filepath.Join(tmp, "repo")
	require.NoError(t, os.MkdirAll(wsDir, 0o755))
	dataDir := filepath.Join(wsDir, ".crush")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/v1/workspaces" {
			// An unconfigured workspace (no provider), as a fresh server sends.
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "w1", "path": wsDir, "data_dir": dataDir,
				"config": map[string]any{"options": map[string]any{"data_directory": dataDir}, "providers": map[string]any{}},
			})
		}
	}))
	t.Cleanup(srv.Close)
	old := clientHost
	clientHost = "tcp://" + srv.Listener.Addr().String()
	t.Cleanup(func() { clientHost = old })

	t.Chdir(tmp)
	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	cmd.Flags().String("cwd", "", "")
	require.NoError(t, cmd.Flags().Set("cwd", "repo")) // Relative on purpose.

	_, cleanup, err := setupClientServerWorkspace(cmd)
	require.NoError(t, err)
	cleanup()

	list, err := projects.List()
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, wsDir, list[0].Path)
	require.Equal(t, dataDir, list[0].DataDir)
}

func TestHomeWorkspaceLocations(t *testing.T) {
	t.Parallel()

	p := projects.Project{Path: "/r", DataDir: "/r/sub/.crush", Locations: []projects.Location{
		{Cwd: "/r/sub", DataDir: "/r/sub/.crush"},
		{Cwd: "/r", DataDir: "/r/.crush"},
	}}
	require.Equal(t, []home.Location{{Cwd: "/r/sub", DataDir: "/r/sub/.crush"}, {Cwd: "/r", DataDir: "/r/.crush"}},
		toHomeWorkspace(p).Locations)

	old := projects.Project{Path: "/r", DataDir: "/r/.crush"} // Written before locations existed.
	require.Equal(t, []home.Location{{Cwd: "/r", DataDir: "/r/.crush"}}, toHomeWorkspace(old).Locations)
}

// Launching from repo/sub and from repo leaves sessions in two databases of
// one workspace: Home lists both, newest first, 50 at most, each tagged with
// where to open it; a database a live server holds is read through it.
func TestHomeSessionsFromEveryDataDir(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	rootData, subData := filepath.Join(root, ".crush"), filepath.Join(sub, ".crush")
	var rootRows, subRows []dbRow
	for i := range 30 {
		rootRows = append(rootRows, dbRow{id: fmt.Sprintf("r%02d", i), updated: int64(100 + 2*i)})
		subRows = append(subRows, dbRow{id: fmt.Sprintf("s%02d", i), updated: int64(101 + 2*i)})
	}
	subRows = append(subRows, dbRow{id: "r29", updated: 500}) // A copy, newer: listed once.
	makeSessionDB(t, rootData, rootRows...)
	makeSessionDB(t, subData, subRows...)
	ws := home.Workspace{Path: root, Locations: []home.Location{{Cwd: sub, DataDir: subData}, {Cwd: root, DataDir: rootData}}}

	list, err := listHomeSessions(t.Context(), nil, ws)
	require.NoError(t, err)
	require.Len(t, list, home.SessionLimit, "the limit is over all locations")
	byID := map[string]home.Session{}
	for _, s := range list {
		require.NotContains(t, byID, s.ID, "listed once")
		byID[s.ID] = s
	}
	require.Equal(t, home.Session{ID: "r29", Title: "title r29", UpdatedAt: time.Unix(500, 0), Dir: sub}, list[0], "the newest copy")
	require.Equal(t, sub, byID["s29"].Dir)
	require.Equal(t, root, byID["r28"].Dir, "each session opens where its database is")
	for i := 1; i < len(list); i++ {
		require.False(t, list[i].UpdatedAt.After(list[i-1].UpdatedAt), "newest first")
	}

	// The root location is open on a live server: its sessions come from
	// there with real statuses; the sub database is still read.
	dir := shortTempDir(t)
	fakeServer(t, sockFor(dir, root), proto.Workspace{Path: root, DataDir: rootData}, []proto.Session{
		{ID: "r29", Title: "live", IsBusy: true, UpdatedAt: 1000},
	}, nil)
	list, err = listHomeSessions(t.Context(), []string{"unix://" + sockFor(dir, root)}, ws)
	require.NoError(t, err)
	require.Equal(t, home.Session{ID: "r29", Title: "live", UpdatedAt: time.Unix(1000, 0), Status: home.StatusRunning, Dir: root}, list[0])
	require.Equal(t, "s29", list[1].ID, "the sub database is still read; its copy of r29 is dropped")
	require.Equal(t, home.StatusUnknown, list[1].Status)
	require.False(t, slices.ContainsFunc(list, func(s home.Session) bool { return s.ID == "r28" }),
		"the root database is not read while its server is live")
}

// Add only registers a directory that exists, as its workspace root, and
// keeps an existing entry (and its data dir) as is. Not parallel: it sets
// the registry location.
func TestHomeSourceAdd(t *testing.T) {
	t.Setenv("CRUSH_GLOBAL_DATA", t.TempDir())
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	require.NoError(t, os.MkdirAll(sub, 0o755))
	file := filepath.Join(root, "f.txt")
	require.NoError(t, os.WriteFile(file, nil, 0o644))

	_, err := homeSource{}.Add(t.Context(), filepath.Join(root, "missing"))
	require.ErrorContains(t, err, "not an existing folder")
	_, err = homeSource{}.Add(t.Context(), file)
	require.ErrorContains(t, err, "not an existing folder")
	list, err := projects.List()
	require.NoError(t, err)
	require.Empty(t, list, "nothing registered for a bad path")

	ws, err := homeSource{}.Add(t.Context(), root)
	require.NoError(t, err)
	require.Equal(t, root, ws.Path)
	require.NoDirExists(t, filepath.Join(root, ".git"), "no git init")

	require.NoError(t, projects.Register(root, filepath.Join(sub, ".crush")))
	ws, err = homeSource{}.Add(t.Context(), root)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(sub, ".crush"), ws.Locations[0].DataDir, "an existing entry keeps its data dir")
	list, err = projects.List()
	require.NoError(t, err)
	require.Len(t, list, 1)
}
