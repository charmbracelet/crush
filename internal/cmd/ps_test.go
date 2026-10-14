package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/proto"
	"github.com/stretchr/testify/require"
)

type fakePSSource struct {
	failInfo     map[string]bool
	failSessions map[string]bool
	workspaces   []proto.Workspace
	sessions     map[string][]proto.Session
	busy         map[string]bool
	waiting      map[string]bool
}

func (f fakePSSource) ListWorkspaces(context.Context) ([]proto.Workspace, error) {
	return f.workspaces, nil
}

func (f fakePSSource) ListSessions(_ context.Context, id string) ([]proto.Session, error) {
	if f.failSessions[id] {
		return nil, errors.New("boom sessions")
	}
	return f.sessions[id], nil
}

func (f fakePSSource) GetAgentSessionInfo(_ context.Context, _, sid string) (*proto.AgentSession, error) {
	if f.failInfo[sid] {
		return nil, errors.New("404")
	}
	return &proto.AgentSession{IsBusy: f.busy[sid], PendingPrompt: f.waiting[sid]}, nil
}

func newFakePSSource() fakePSSource {
	return fakePSSource{
		workspaces: []proto.Workspace{{ID: "w1", Path: "/p/one"}, {ID: "w2", Path: "/p/two"}},
		sessions: map[string][]proto.Session{
			"w1": {{ID: "a", Title: "alpha"}, {ID: "b", Title: "beta"}},
			"w2": {{ID: "c", Title: "gamma"}, {ID: "d", Title: "delta"}},
		},
		busy:    map[string]bool{"b": true, "d": true},
		waiting: map[string]bool{"d": true},
	}
}

func TestCollectPSRowsAcrossWorkspaces(t *testing.T) {
	t.Parallel()

	rows, err := collectPS(t.Context(), newFakePSSource(), 9)
	require.NoError(t, err)
	require.Equal(t, []psRow{
		{Path: "/p/one", SessionID: "a", Title: "alpha"},
		{Path: "/p/one", SessionID: "b", Title: "beta", Busy: true},
		{Path: "/p/two", SessionID: "c", Title: "gamma"},
		{Path: "/p/two", SessionID: "d", Title: "delta", Busy: true, Waiting: true},
	}, rows)

	limited, err := collectPS(t.Context(), newFakePSSource(), 1)
	require.NoError(t, err)
	require.Len(t, limited, 2)
}

func TestPSTableStatus(t *testing.T) {
	t.Parallel()

	rows, err := collectPS(t.Context(), newFakePSSource(), 9)
	require.NoError(t, err)
	var buf bytes.Buffer
	require.NoError(t, writePS(&buf, rows, false))
	out := buf.String()
	require.Regexp(t, `PROJECT\s+SESSION\s+STATUS\s+TITLE`, out)
	require.Regexp(t, `/p/one\s+a\s+IDLE\s+alpha`, out)
	require.Regexp(t, `/p/one\s+b\s+BUSY\s+beta`, out)
	// A pending prompt wins over busy.
	require.Regexp(t, `/p/two\s+d\s+WAITING\s+delta`, out)
}

func TestPSJSONOutput(t *testing.T) {
	t.Parallel()

	rows, err := collectPS(t.Context(), newFakePSSource(), 9)
	require.NoError(t, err)
	var buf bytes.Buffer
	require.NoError(t, writePS(&buf, rows, true))
	var got []map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	require.Len(t, got, 4)
	require.Equal(t, "/p/one", got[0]["path"])
	require.Equal(t, "b", got[1]["session_id"])
	require.Equal(t, true, got[1]["busy"])
	require.Equal(t, true, got[3]["waiting"])
}

func TestPSNoServerDoesNotSpawn(t *testing.T) {
	t.Parallel()

	sock := filepath.Join(t.TempDir(), "agent-cli-deadbeef.sock")
	var buf bytes.Buffer
	require.NoError(t, runPS(t.Context(), &buf, io.Discard, []string{"unix://" + sock}, false, 9))
	require.Equal(t, "No agent-cli server running (open projects with agent-cli-tmux)\n", buf.String())
	// A spawned server would have created its socket.
	require.NoFileExists(t, sock)

	buf.Reset()
	require.NoError(t, runPS(t.Context(), &buf, io.Discard, []string{"unix://" + sock}, true, 9))
	require.JSONEq(t, `[]`, buf.String())
}

func TestPSSocketDirsFindsSockets(t *testing.T) {
	rt := t.TempDir()
	t.Setenv("AGENT_CLI_RUNTIME_DIR", rt)
	t.Setenv("XDG_RUNTIME_DIR", rt) // Duplicate dir must not duplicate hosts.
	t.Setenv("HOME", t.TempDir())
	want := filepath.Join(rt, "agent-cli-12345678.sock")
	require.NoError(t, os.WriteFile(want, nil, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(rt, "other.sock"), nil, 0o600))

	require.Equal(t, []string{"unix://" + want}, discoverPSHosts())
}

func TestCollectPSSkipsFailingSession(t *testing.T) {
	t.Parallel()

	src := newFakePSSource()
	src.failInfo = map[string]bool{"a": true}
	rows, err := collectPS(t.Context(), src, 9)
	require.ErrorContains(t, err, "session a")
	require.Len(t, rows, 3)
	require.Equal(t, "b", rows[0].SessionID)
}

func TestCollectPSKeepsRowsWhenListSessionsFails(t *testing.T) {
	t.Parallel()

	src := newFakePSSource()
	src.failSessions = map[string]bool{"w1": true}
	rows, err := collectPS(t.Context(), src, 9)
	require.ErrorContains(t, err, "boom sessions")
	require.Len(t, rows, 2)
	require.Equal(t, "/p/two", rows[0].Path)
}

func TestPSTitleSanitized(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, writePS(&buf, []psRow{{Path: "/p", SessionID: "x", Title: "a\tb\nc"}}, false))
	require.Contains(t, buf.String(), "a b c")
	require.Len(t, bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n")), 2)
}

func TestQueryPSHostTimeout(t *testing.T) {
	t.Parallel()

	sock := filepath.Join(t.TempDir(), "hung.sock")
	ln, err := net.Listen("unix", sock)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			defer conn.Close() // Accept and never answer.
		}
	}()

	start := time.Now()
	_, alive, err := queryPSHost(t.Context(), "unix://"+sock, 500*time.Millisecond, 9)
	require.Error(t, err)
	require.False(t, alive)
	require.Less(t, time.Since(start), 4*time.Second)
}

func TestPSLiveServerFailureIsNotNoServer(t *testing.T) {
	t.Parallel()

	sock := filepath.Join(t.TempDir(), "agent-cli-live.sock")
	ln, err := net.Listen("unix", sock)
	require.NoError(t, err)
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/health" {
			http.Error(w, "boom", http.StatusInternalServerError)
		}
	})}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	var out, errOut bytes.Buffer
	host := "unix://" + sock
	require.NoError(t, runPS(t.Context(), &out, &errOut, []string{host}, false, 9))
	require.NotContains(t, out.String(), "No agent-cli server running")
	require.Contains(t, out.String(), "PROJECT")
	require.Contains(t, errOut.String(), "Skipping "+host+": ")
}

func TestPSAllIncludesOldestBusySession(t *testing.T) {
	t.Parallel()

	src := fakePSSource{
		workspaces: []proto.Workspace{{ID: "w", Path: "/p"}},
		sessions:   map[string][]proto.Session{"w": nil},
		busy:       map[string]bool{"s11": true}, // Oldest, listed last.
	}
	for i := range 12 {
		src.sessions["w"] = append(src.sessions["w"], proto.Session{ID: fmt.Sprintf("s%d", i)})
	}

	rows, err := collectPS(t.Context(), src, psSessionsPerWorkspace)
	require.NoError(t, err)
	require.Len(t, rows, 9)
	for _, r := range rows {
		require.NotEqual(t, "s11", r.SessionID)
	}

	rows, err = collectPS(t.Context(), src, math.MaxInt)
	require.NoError(t, err)
	require.Len(t, rows, 12)
	require.Equal(t, psRow{Path: "/p", SessionID: "s11", Busy: true}, rows[11])
}

func TestPSAllFlagRegistered(t *testing.T) {
	t.Parallel()

	require.NotNil(t, psCmd.Flags().Lookup("all"))
}

func TestPSHungServerIsNotedNotNoServer(t *testing.T) {
	t.Parallel()

	sock := filepath.Join(t.TempDir(), "agent-cli-hung.sock")
	ln, err := net.Listen("unix", sock)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			defer conn.Close() // Accept and never answer.
		}
	}()

	// Shorter than psServerTimeout is not injectable via runPS, so use the
	// real timeout; the test takes about 3s.
	var out, errOut bytes.Buffer
	host := "unix://" + sock
	require.NoError(t, runPS(t.Context(), &out, &errOut, []string{host}, false, 9))
	require.NotContains(t, out.String(), "No agent-cli server running")
	require.Contains(t, errOut.String(), "Skipping "+host+": not responding (")
}

func TestPSDeadServersStaySilent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// A stale socket file: bound then closed without unlinking.
	stale := filepath.Join(dir, "agent-cli-stale.sock")
	ln, err := net.Listen("unix", stale)
	require.NoError(t, err)
	ln.(*net.UnixListener).SetUnlinkOnClose(false)
	require.NoError(t, ln.Close())

	var out, errOut bytes.Buffer
	hosts := []string{"unix://" + stale, "unix://" + filepath.Join(dir, "missing.sock")}
	require.NoError(t, runPS(t.Context(), &out, &errOut, hosts, false, 9))
	require.Empty(t, errOut.String())
	require.Contains(t, out.String(), "No agent-cli server running")
}

// Sessions whose status cannot be read are counted on stderr ("not counted
// = not seen"); stdout keeps the JSON of the rows that were read.
func TestPSCountsSkippedSessionsOnStderr(t *testing.T) {
	t.Parallel()

	sock := filepath.Join(t.TempDir(), "agent-cli-skip.sock")
	ln, err := net.Listen("unix", sock)
	require.NoError(t, err)
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/health":
		case "/v1/workspaces":
			_ = json.NewEncoder(w).Encode([]proto.Workspace{{ID: "w", Path: "/p"}})
		case "/v1/workspaces/w/sessions":
			_ = json.NewEncoder(w).Encode([]proto.Session{{ID: "a"}, {ID: "b"}, {ID: "c"}})
		case "/v1/workspaces/w/agent/sessions/c":
			_ = json.NewEncoder(w).Encode(proto.AgentSession{IsBusy: true})
		default:
			http.Error(w, "gone", http.StatusNotFound)
		}
	})}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	var out, errOut bytes.Buffer
	host := "unix://" + sock
	require.NoError(t, runPS(t.Context(), &out, &errOut, []string{host}, true, 9))
	var rows []psRow
	require.NoError(t, json.Unmarshal(out.Bytes(), &rows))
	require.Equal(t, []psRow{{Path: "/p", SessionID: "c", Busy: true}}, rows)
	require.Equal(t, 1, strings.Count(errOut.String(), "\n"), errOut.String())
	require.Contains(t, errOut.String(), "Skipping "+host+": 2 sessions skipped (session a: ")
}
