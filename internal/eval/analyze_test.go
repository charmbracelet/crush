package eval

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/stretchr/testify/require"
)

// --- synthetic session-DB fixture ---

// part builders emit the stored {"type","data"} wrapper shape.
func tcPart(id, name, input string) string {
	return fmt.Sprintf(`{"type":"tool_call","data":{"id":%q,"name":%q,"input":%q,"provider_executed":false,"finished":true}}`,
		id, name, input)
}

// tcPartUnfinished emits the truncated-stream shape: OnToolInputStart
// persisted the part and OnToolCall never ran.
func tcPartUnfinished(id, name string) string {
	return fmt.Sprintf(`{"type":"tool_call","data":{"id":%q,"name":%q,"input":"","provider_executed":false,"finished":false}}`,
		id, name)
}

func trPart(callID, content string, isErr bool, meta string) string {
	return fmt.Sprintf(`{"type":"tool_result","data":{"tool_call_id":%q,"name":"x","content":%q,"data":"","mime_type":"","metadata":%q,"is_error":%v}}`,
		callID, content, meta, isErr)
}

func txtPart(s string) string {
	return fmt.Sprintf(`{"type":"text","data":{"text":%q}}`, s)
}

func insertSession(t *testing.T, conn *sql.DB, id, parentID string) {
	t.Helper()
	var parent any
	if parentID != "" {
		parent = parentID
	}
	_, err := conn.ExecContext(t.Context(),
		`INSERT INTO sessions (id, parent_session_id, title, created_at, updated_at) VALUES (?, ?, 't', 1000, 1000)`,
		id, parent)
	require.NoError(t, err)
}

func insertMsg(t *testing.T, conn *sql.DB, id, sessionID, role, parts string, createdAt int64, summary int) {
	t.Helper()
	_, err := conn.ExecContext(t.Context(),
		`INSERT INTO messages (id, session_id, role, parts, created_at, updated_at, is_summary_message)
		 VALUES (?, ?, ?, ?, ?, 1000, ?)`,
		id, sessionID, role, parts, createdAt, summary)
	require.NoError(t, err)
}

// writeFixtureDB builds the canonical fixture: one parent session with
// every edge row the analyzer must classify, plus a child session whose
// messages must be ignored. Every message shares created_at=1000, so
// all ordering rests on the rowid tiebreaker.
//
// Returns the db file path.
func writeFixtureDB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	conn, err := db.Connect(context.Background(), dir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Release(dir)) })

	insertSession(t, conn, "s1", "")
	insertSession(t, conn, "s2", "s1") // Sub-agent child — out of scope.

	msgs := []struct {
		id      string
		role    string
		parts   string
		summary int
	}{
		// Turn 0 (first `crush run`).
		{"m01", "user", `[` + txtPart("turn one") + `]`, 0},
		{"m02", "assistant", `[` + tcPart("c1", "view", `{"file_path":"a.go"}`) + `]`, 0},
		{"m03", "tool", `[` + trPart("c1", "package a", false, "") + `]`, 0},
		{"m04", "assistant", `[` + tcPart("c2", "grep", `{"pattern":"needle"}`) + `]`, 0},
		{"m05", "tool", `[` + trPart("c2", "a.go:1:needle", false, "") + `]`, 0},
		// Errored read of an unseen path — attempts count: the
		// roundtrip was spent, so it is discovery (and a
		// view_directory_error); it just never joins the seen-set.
		{"m05b", "assistant", `[` + tcPart("c2b", "view", `{"file_path":"/w/missing-dir"}`) + `]`, 0},
		{"m05c", "tool", `[` + trPart("c2b", "Path is a directory, not a file: /w/missing-dir", true, "") + `]`, 0},
		// Canceled write placeholder: the name survives the input
		// rewrite, so first_write_attempt_index lands here — before
		// the first landed write — and closes the discovery window.
		{"m05d", "assistant", `[` + tcPart("c2c", "edit", `{}`) + `]`, 0},
		{"m05e", "tool", `[` + trPart("c2c", "Error: user cancelled assistant tool calling", true, "") + `]`, 0},
		// Post-attempt pre-write discovery — must NOT count.
		{"m05f", "assistant", `[` + tcPart("c2d", "grep", `{"pattern":"late"}`) + `]`, 0},
		{"m05g", "tool", `[` + trPart("c2d", "a.go:9:late", false, "") + `]`, 0},
		{"m06", "assistant", `[` + tcPart("c3", "view", `{"file_path":"/w/a.go"}`) + `]`, 0},
		{"m07", "tool", `[` + trPart("c3", "package a", false, "") + `]`, 0},
		{"m08", "assistant", `[` + tcPart("c4", "edit", `{"file_path":"/w/a.go","old_string":"x"}`) + `]`, 0},
		{"m09", "tool", `[` + trPart("c4", "old_string not found in /w/a.go", true, "") + `]`, 0},
		{"m10", "assistant", `[` + tcPart("c5", "edit", `{"file_path":"/w/a.go","old_string":"y"}`) + `]`, 0},
		{"m11", "tool", `[` + trPart("c5", "ok", false, "") + `]`, 0},
		{"m12", "assistant", `[` + tcPart("c6", "map", `{"symbol":"Config"}`) + `,` + tcPart("c7", "question", `{"question":"q?"}`) + `]`, 0},
		{"m13", "tool", `[` + trPart("c6", "Config defined at a.go:12", false, "") + `]`, 0},
		{"m14", "tool", `[` + trPart("c7", "tool not found: question", true, "") + `]`, 0},
		{"m15", "assistant", `[` + tcPart("c8", "grep", `{"pattern":"Config"}`) + `,` + tcPart("c9", "view", `{"file_path":"/w/dir"}`) + `]`, 0},
		{"m16", "tool", `[` + trPart("c8", "a.go:12:Config", false, "") + `]`, 0},
		{"m17", "tool", `[` + trPart("c9", "Path is a directory, not a file: /w/dir", true, "") + `]`, 0},
		{"m18", "assistant", `[` + tcPart("c10", "bash", `{}`) + `]`, 0},
		{"m19", "tool", `[` + trPart("c10", "Error: user cancelled assistant tool calling", true, "") + `]`, 0},
		// Repair turn: harness-authored retry prompt inside the same
		// process — must NOT advance the turn index.
		{"m20", "user", `[` + txtPart("Verification failed. The following check(s) did not pass — fix the underlying issue") + `]`, 0},
		{"m21", "assistant", `[` + tcPart("c11", "view", `{"file_path":"a.go"}`) + `]`, 0},
		{"m22", "tool", `[` + trPart("c11", "package a", false, "") + `]`, 0},
		// Turn 1 (second `crush run` on the same session).
		{"m23", "user", `[` + txtPart("turn two") + `]`, 0},
		{"m24", "assistant", `[` + tcPart("c12", "view", `{"file_path":"./a.go"}`) + `]`, 0},
		{"m25", "tool", `[` + trPart("c12", "package a", false, "") + `]`, 0},
		{"m26", "assistant", `[` + txtPart("summary") + `]`, 1}, // Summary row — not a request.
		// Canceled-turn row — the finish part marks it; not a request.
		{"m26b", "assistant", `[{"type":"finish","data":{"reason":"canceled","time":1000,"message":"User canceled request"}}]`, 0},
		{"m27", "assistant", `[` + txtPart("done") + `]`, 0},
		{"m28", "assistant", `[` + tcPart("c13", "edit", `{"file_path":"/w/b.go"}`) + `]`, 0},
		{"m29", "tool", `[` + trPart("c13", "Tool call blocked by hook. Reason: nope", true, `{"hook":{"hook_count":1,"decision":"deny"}}`) + `]`, 0},
		{"m30", "assistant", `[` + tcPart("c14", "map", `{"symbol":"Other"}`) + `]`, 0},
		{"m31", "tool", `[` + trPart("c14", "tool not found: map", true, "") + `]`, 0},
		{"m32", "assistant", `[` + tcPart("c15", "view", `{"file_path":"/w/c.go"}`) + `]`, 0},
		// c15 gets no result — the hard-kill shape.
		// Generic cleanup error on a "{}" call — interrupted, not
		// canceled: the string is written for any missing-result
		// cleanup, not only cancels.
		{"m33", "assistant", `[` + tcPart("c16", "bash", `{}`) + `]`, 0},
		{"m34", "tool", `[` + trPart("c16", "There was an error while executing the tool", true, "") + `]`, 0},
		// A real zero-arg call that errored on validation — a genuine
		// dispatched call, not a placeholder.
		{"m35", "assistant", `[` + tcPart("c17", "view", `{}`) + `]`, 0},
		{"m36", "tool", `[` + trPart("c17", "file_path is required", true, "") + `]`, 0},
		// map call whose result never landed — gave the model no
		// pointer, so the following grep must NOT count as a
		// wrong-pointer event.
		{"m37", "assistant", `[` + tcPart("c18", "map", `{"symbol":"Zebra"}`) + `,` + tcPart("c19", "grep", `{"pattern":"Zebra"}`) + `]`, 0},
		{"m38", "tool", `[` + trPart("c19", "z.go:1:Zebra", false, "") + `]`, 0},
		// map{} skeleton is a REAL zero-arg call — same input shape as
		// the placeholders, discriminated by the successful result.
		{"m39", "assistant", `[` + tcPart("c20", "map", `{}`) + `]`, 0},
		{"m40", "tool", `[` + trPart("c20", "map skeleton output", false, "") + `]`, 0},
		// Truncated stream: finished=false, input="" — labeled
		// truncated, not canceled.
		{"m41", "assistant", `[` + tcPartUnfinished("c21", "view") + `]`, 0},
		// Mid-stream cancel: a real billed request that completed a
		// call, then got Finish{canceled} — the request counts and the
		// call takes this request's index.
		{"m42", "assistant", `[` + tcPart("c22", "grep", `{"pattern":"late"}`) + `,{"type":"finish","data":{"reason":"canceled","time":1000,"message":"User canceled request"}}]`, 0},
		{"m43", "tool", `[` + trPart("c22", "b.go:2:late", false, "") + `]`, 0},
		// Paging: a re-view of a seen path with a different window is
		// legitimate paging, not a reread; the same window again IS.
		{"m44", "assistant", `[` + tcPart("c23", "view", `{"file_path":"/w/a.go","offset":500}`) + `]`, 0},
		{"m45", "tool", `[` + trPart("c23", "...500-600...", false, "") + `]`, 0},
		{"m46", "assistant", `[` + tcPart("c24", "view", `{"file_path":"/w/a.go","offset":500}`) + `]`, 0},
		{"m47", "tool", `[` + trPart("c24", "...500-600...", false, "") + `]`, 0},
		// Index-not-ready map — its own sub-count, not a generic
		// failure.
		{"m48", "assistant", `[` + tcPart("c25", "map", `{"symbol":"Late"}`) + `]`, 0},
		{"m49", "tool", `[` + trPart("c25", "project index unavailable — fall back to grep/glob", true, "") + `]`, 0},
		// Same content, different spelling: offset=0/limit=200 is the
		// tool's default window — it must alias to the bare view's
		// window, i.e. a reread of the SAME content, not paging.
		{"m50", "assistant", `[` + tcPart("c26", "view", `{"file_path":"/w/a.go","offset":0,"limit":200}`) + `]`, 0},
		{"m51", "tool", `[` + trPart("c26", "package a", false, "") + `]`, 0},
	}
	for _, m := range msgs {
		insertMsg(t, conn, m.id, "s1", m.role, m.parts, 1000, m.summary)
	}

	// One tracker row — the read_files cross-check against the
	// reconstructed seen-set.
	_, err = conn.ExecContext(t.Context(),
		`INSERT INTO read_files (session_id, path, read_at) VALUES ('s1', '/w/a.go', 1000)`)
	require.NoError(t, err)

	// Child-session rows that must never leak into parent metrics.
	insertMsg(t, conn, "k1", "s2", "user", `[`+txtPart("sub")+`]`, 1000, 0)
	insertMsg(t, conn, "k2", "s2", "assistant", `[`+tcPart("k1", "edit", `{"file_path":"/w/z.go"}`)+`]`, 1000, 0)
	insertMsg(t, conn, "k3", "s2", "tool", `[`+trPart("k1", "ok", false, "")+`]`, 1000, 0)

	return filepath.Join(dir, "crush.db")
}

func TestAnalyzeSessionDB_Fixture(t *testing.T) {
	t.Parallel()
	dbPath := writeFixtureDB(t)

	for _, turns := range [][]string{nil, {"turn one", "turn two"}} {
		cm, err := AnalyzeSessionDB(context.Background(), dbPath, AnalyzeOptions{
			Workdir: "/w",
			Turns:   turns,
		})
		require.NoError(t, err)

		require.Equal(t, "s1", cm.SessionID)
		// Summary row and the finish-only canceled-turn row are not
		// requests; m42's mid-stream cancel IS (real parts + finish).
		require.Equal(t, 27, cm.Requests)
		// c10+c2c canceled, c16 interrupted, c21 truncated — labeled,
		// not counted; c17's real arg-validation error, c20's map{}
		// skeleton, and c22's mid-stream-cancel call ARE real calls.
		require.Equal(t, 25, cm.Calls)
		require.Len(t, cm.ToolCalls, 29)
		require.Equal(t, 2, cm.CanceledCalls) // c10 + c2c.
		require.Equal(t, 1, cm.InterruptedCalls)
		require.Equal(t, 1, cm.TruncatedCalls)

		require.Equal(t, 6, cm.FirstWriteIndex) // c4 — first landed write.
		// c2c's canceled edit keeps its name — the attempt marks the
		// model acting and closes the discovery window early.
		require.Equal(t, 3, cm.FirstWriteAttemptIndex)
		// Anchored on the ATTEMPT — c2c's request index is 3.
		require.Equal(t, 4, cm.RequestsToFirstEdit)
		// c1 view-unseen + c2 grep + c2b dir-view attempt; c2d's grep
		// lands after the write attempt and does NOT count.
		require.Equal(t, 3, cm.DiscoveryCallsBeforeWrite)
		require.Equal(t, 1, cm.FilesViewed)
		require.Equal(t, 1, cm.ReadFilesRows)

		require.Equal(t, 2, cm.EditFailures)
		require.Equal(t, 1, cm.EditFailuresHook)
		require.Equal(t, 1, cm.EditFailuresOther)

		// c6 ok + c14 hallucinated "tool not found" + c18 no result +
		// c20 skeleton ok + c25 index-unavailable.
		require.Equal(t, 5, cm.MapCalls)
		require.Equal(t, 2, cm.MapCallsOK)
		require.Equal(t, 1, cm.MapCallsIndexUnavailable)
		require.Equal(t, int64(len("Config defined at a.go:12")+len("map skeleton output")), cm.MapResultBytes)

		require.Equal(t, 1, cm.QuestionCalls)
		require.Equal(t, 1, cm.QuestionCallsErrored)

		// c3 same-turn, c11 same-turn (repair prompt stayed in-process),
		// c12 cross-turn, c24 same-window re-view same-turn, c26 the
		// explicit-default window (offset=0/limit=200 aliases to bare
		// view's window — same content, reread). c23's offset=500 view
		// is paging — a new window, not a reread.
		require.Equal(t, 5, cm.Rereads)
		require.Equal(t, 4, cm.RereadsSameTurn)
		require.Equal(t, 1, cm.RereadsCrossTurn)

		// c6 map(symbol=Config) → c8 grep Config. c18's result never
		// landed — no pointer delivered — so c19's grep Zebra is not
		// an event.
		require.Equal(t, 1, cm.WrongPointerEvents)
		require.Equal(t, 2, cm.ViewDirectoryErrors) // c2b + c9.

		var canceled, interrupted, truncated, noResult int
		canceledNames := map[string]bool{}
		for i := range cm.ToolCalls {
			rec := &cm.ToolCalls[i]
			if rec.Canceled {
				canceled++
				canceledNames[rec.Name] = true
			}
			if rec.Interrupted {
				interrupted++
			}
			if rec.Truncated {
				truncated++
				require.Equal(t, "view", rec.Name)
			}
			if rec.NoResult {
				noResult++
			}
		}
		require.Equal(t, 2, canceled)
		require.True(t, canceledNames["bash"] && canceledNames["edit"])
		require.Equal(t, 1, interrupted)
		require.Equal(t, 1, truncated)
		require.Equal(t, 3, noResult) // c15, c18, c21.

		// c22 belongs to the mid-stream-canceled request — its own
		// request index (22), not the previous request's.
		var c22 *CallRecord
		for i := range cm.ToolCalls {
			if cm.ToolCalls[i].ID == "c22" {
				c22 = &cm.ToolCalls[i]
			}
		}
		require.NotNil(t, c22)
		require.Equal(t, 22, c22.Step)
	}
}

func TestAnalyzeSessionDB_NoSession(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := db.Connect(context.Background(), dir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Release(dir)) })

	_, err = AnalyzeSessionDB(context.Background(), filepath.Join(dir, "crush.db"), AnalyzeOptions{})
	require.Error(t, err)
}

// TestAnalyzeSessionDB_EmptySession pins the failure mode: a session ID
// that resolves to zero messages must error, not report a valid
// zero-call run.
func TestAnalyzeSessionDB_EmptySession(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	conn, err := db.Connect(context.Background(), dir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Release(dir)) })
	insertSession(t, conn, "s1", "")

	// Both the missing-session and the empty-session shapes.
	for _, id := range []string{"missing", "s1"} {
		_, err = AnalyzeSessionDB(context.Background(), filepath.Join(dir, "crush.db"),
			AnalyzeOptions{SessionID: id})
		require.Error(t, err, "session %q", id)
	}
}

// TestAnalyzeSessionDB_LegacySchema pins the pre-20250810 fallback:
// artifacts from before is_summary_message existed must analyze via
// the hasColumn backfill rather than erroring on the column.
func TestAnalyzeSessionDB_LegacySchema(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	conn, err := db.Connect(context.Background(), dir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Release(dir)) })

	_, err = conn.ExecContext(t.Context(), `ALTER TABLE messages DROP COLUMN is_summary_message`)
	require.NoError(t, err)

	insertSession(t, conn, "s1", "")
	for _, m := range []struct{ id, role, parts string }{
		{"m1", "user", `[` + txtPart("turn") + `]`},
		{"m2", "assistant", `[` + tcPart("c1", "view", `{"file_path":"/w/a.go"}`) + `]`},
		{"m3", "tool", `[` + trPart("c1", "package a", false, "") + `]`},
	} {
		_, err := conn.ExecContext(t.Context(),
			`INSERT INTO messages (id, session_id, role, parts, created_at, updated_at)
			 VALUES (?, 's1', ?, ?, 1000, 1000)`, m.id, m.role, m.parts)
		require.NoError(t, err)
	}

	cm, err := AnalyzeSessionDB(context.Background(), filepath.Join(dir, "crush.db"),
		AnalyzeOptions{Workdir: "/w"})
	require.NoError(t, err)
	require.Equal(t, 1, cm.Requests)
	require.Equal(t, 1, cm.Calls)
	require.Equal(t, 1, cm.DiscoveryCallsBeforeWrite)
}

// TestAnalyzeSessionDB_TurnSkip pins self-healing turn matching: when a
// turn's user message never persisted (the turn-0-kill shape), later
// turns still advance the process index rather than collapsing into
// the previous one.
func TestAnalyzeSessionDB_TurnSkip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	conn, err := db.Connect(context.Background(), dir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Release(dir)) })

	insertSession(t, conn, "s1", "")
	// Turn t2's user message never landed — the process was killed
	// between session continuation and createUserMessage.
	for _, m := range []struct{ id, role, parts string }{
		{"m1", "user", `[` + txtPart("t1") + `]`},
		{"m2", "assistant", `[` + tcPart("c1", "view", `{"file_path":"/w/a.go"}`) + `]`},
		{"m3", "tool", `[` + trPart("c1", "package a", false, "") + `]`},
		{"m4", "user", `[` + txtPart("t3") + `]`},
		{"m5", "assistant", `[` + tcPart("c2", "view", `{"file_path":"/w/a.go"}`) + `]`},
		{"m6", "tool", `[` + trPart("c2", "package a", false, "") + `]`},
	} {
		insertMsg(t, conn, m.id, "s1", m.role, m.parts, 1000, 0)
	}

	cm, err := AnalyzeSessionDB(context.Background(), filepath.Join(dir, "crush.db"),
		AnalyzeOptions{Workdir: "/w", Turns: []string{"t1", "t2", "t3"}})
	require.NoError(t, err)
	// c2 lands on trajectory index 2 — not collapsed into turn 0, and
	// the second view is a cross-turn reread, not same-turn.
	require.Equal(t, 2, cm.ToolCalls[1].Turn)
	require.Equal(t, 1, cm.Rereads)
	require.Equal(t, 0, cm.RereadsSameTurn)
	require.Equal(t, 1, cm.RereadsCrossTurn)
}

// TestPreserveSessionDB_CapturesWALTail pins the fix at the source: the
// run's DB is WAL-mode, so a bare crush.db copy drops committed rows
// still in the -wal tail — the derailment tail on WaitDelay hard-kills.
// VACUUM INTO snapshots through it.
func TestPreserveSessionDB_CapturesWALTail(t *testing.T) {
	t.Parallel()
	workParent := t.TempDir()
	workdir := filepath.Join(workParent, "w")
	dataDir := DataDirFor(workdir)

	conn, err := db.Connect(context.Background(), dataDir)
	require.NoError(t, err)
	// The connection stays open: committed rows sit in the
	// un-checkpointed WAL — the shape a hard-killed run leaves behind.
	insertSession(t, conn, "s1", "")
	insertMsg(t, conn, "m1", "s1", "assistant", `[`+tcPart("c1", "edit", `{"file_path":"/w/a.go"}`)+`]`, 1000, 0)

	r := &Runner{EvalDir: t.TempDir()}
	dst, walSafe, err := r.preserveSessionDB(context.Background(), "exp", "traj", "control", "inv", 0, workdir)
	require.NoError(t, err)
	require.True(t, walSafe)
	require.NoError(t, db.Release(dataDir))

	ro, err := db.ConnectReadOnly(context.Background(), filepath.Join(r.EvalDir, dst))
	require.NoError(t, err)
	defer ro.Close()
	var n int
	require.NoError(t, ro.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM messages`).Scan(&n))
	require.Equal(t, 1, n, "artifact must carry the un-checkpointed WAL tail")
}

// TestPreserveSessionDB_IncompleteFlag: when VACUUM INTO fails the raw
// copy may lack the WAL tail — the caller must learn that via
// walSafe=false so the record can stamp session_db_incomplete.
// VACUUM INTO refuses an existing target file, which is the seam.
func TestPreserveSessionDB_IncompleteFlag(t *testing.T) {
	t.Parallel()
	workParent := t.TempDir()
	workdir := filepath.Join(workParent, "w")
	dataDir := DataDirFor(workdir)

	conn, err := db.Connect(context.Background(), dataDir)
	require.NoError(t, err)
	insertSession(t, conn, "s1", "")
	require.NoError(t, db.Release(dataDir))

	r := &Runner{EvalDir: t.TempDir()}
	dstDir := filepath.Join(r.EvalDir, "results", "exp", "artifacts")
	require.NoError(t, os.MkdirAll(dstDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dstDir, "traj-control-inv-0.db"), []byte("exists"), 0o644))

	dst, walSafe, err := r.preserveSessionDB(
		context.Background(), "exp", "traj", "control", "inv", 0, workdir)
	require.NoError(t, err)
	require.False(t, walSafe)
	require.NotEmpty(t, dst)
}

// TestExecuteRun_PreserveFailure pins the failure stamps: a run that
// left no session DB records WHY the metrics are absent instead of
// silently reporting none — and still carries workdir for forensics.
func TestExecuteRun_PreserveFailure(t *testing.T) {
	t.Parallel()
	root := newEvalDir(t)
	trajDir := writeTrajectory(t, filepath.Join(root, "corpus"), "t1", nil)
	traj, err := LoadTrajectory(trajDir)
	require.NoError(t, err)
	exp := &Experiment{
		Name: "e1", Model: "mock/m", Temperature: ptr(0.0),
		Arms: map[string]Arm{ArmControl: {}},
	}
	r := &Runner{EvalDir: root, Driver: noDBDriver{}, WorkParent: t.TempDir()}

	rec, err := r.ExecuteRun(context.Background(), exp, traj, trajDir,
		ArmControl, Arm{}, &FlagsManifest{Defaults: map[string]any{}}, 1, "inv1")
	require.NoError(t, err)
	require.Empty(t, rec.SessionDB)
	require.Nil(t, rec.CallMetrics)
	require.Contains(t, rec.CallMetricsError, "session db not preserved")
	require.NotEmpty(t, rec.Workdir)
}

// TestExecuteRun_AnalyzerFailure pins the stamp for a preserved-but-
// unanalyzable artifact: telemetry reported a session the DB lacks.
func TestExecuteRun_AnalyzerFailure(t *testing.T) {
	t.Parallel()
	root := newEvalDir(t)
	trajDir := writeTrajectory(t, filepath.Join(root, "corpus"), "t1", nil)
	traj, err := LoadTrajectory(trajDir)
	require.NoError(t, err)
	exp := &Experiment{
		Name: "e1", Model: "mock/m", Temperature: ptr(0.0),
		Arms: map[string]Arm{ArmControl: {}},
	}
	r := &Runner{EvalDir: root, Driver: dbDriver{sessionID: "missing"}, WorkParent: t.TempDir()}

	rec, err := r.ExecuteRun(context.Background(), exp, traj, trajDir,
		ArmControl, Arm{}, &FlagsManifest{Defaults: map[string]any{}}, 1, "inv1")
	require.NoError(t, err)
	require.NotEmpty(t, rec.SessionDB) // Preserved — it's the analysis that failed.
	require.Nil(t, rec.CallMetrics)
	require.Contains(t, rec.CallMetricsError, "no messages")
}

// dbDriver is a fake AgentRunner that leaves a real session DB behind —
// ExecuteRun's preserve+analyze path end-to-end, no provider.
type dbDriver struct{ sessionID string }

func (d dbDriver) Run(_ context.Context, workdir string, _ []string, _ Budget) RunResult {
	dataDir := DataDirFor(workdir)
	conn, err := db.Connect(context.Background(), dataDir)
	if err != nil {
		return RunResult{Err: err}
	}
	defer db.Release(dataDir)
	_, err = conn.ExecContext(context.Background(), `INSERT INTO sessions (id, parent_session_id, title, created_at, updated_at) VALUES ('s1', NULL, 't', 1000, 1000)`)
	if err != nil {
		return RunResult{Err: err}
	}
	for _, q := range []string{
		`INSERT INTO messages (id, session_id, role, parts, created_at, updated_at) VALUES ('u1', 's1', 'user', '[{"type":"text","data":{"text":"fix it"}}]', 1000, 1000)`,
		`INSERT INTO messages (id, session_id, role, parts, created_at, updated_at) VALUES ('a1', 's1', 'assistant', '[{"type":"tool_call","data":{"id":"c1","name":"view","input":"{\"file_path\":\"x.txt\"}","provider_executed":false,"finished":true}}]', 1000, 1000)`,
		`INSERT INTO messages (id, session_id, role, parts, created_at, updated_at) VALUES ('t1', 's1', 'tool', '[{"type":"tool_result","data":{"tool_call_id":"c1","name":"view","content":"x","data":"","mime_type":"","metadata":"","is_error":false}}]', 1000, 1000)`,
	} {
		if _, err := conn.ExecContext(context.Background(), q); err != nil {
			return RunResult{Err: err}
		}
	}
	_ = os.WriteFile(filepath.Join(workdir, "fixed.marker"), []byte("x"), 0o644)
	return RunResult{Steps: 1, ModelResolved: "mock/m", SessionID: d.sessionID}
}

// noDBDriver leaves no session DB — the preserve-failure shape.
type noDBDriver struct{}

func (noDBDriver) Run(_ context.Context, workdir string, _ []string, _ Budget) RunResult {
	_ = os.WriteFile(filepath.Join(workdir, "fixed.marker"), []byte("x"), 0o644)
	return RunResult{Steps: 1, ModelResolved: "mock/m"}
}

func TestExecuteRun_AttachesCallMetrics(t *testing.T) {
	t.Parallel()
	root := newEvalDir(t)
	trajDir := writeTrajectory(t, filepath.Join(root, "corpus"), "t1", map[string]any{
		// The coverage predicate reads call_metrics off the record —
		// this exercises the "before CoverageMet" ordering too.
		"coverage": map[string]any{"min_call_metrics.requests": 1},
	})
	traj, err := LoadTrajectory(trajDir)
	require.NoError(t, err)
	exp := &Experiment{
		Name: "e1", Model: "mock/m", Temperature: ptr(0.0),
		Arms: map[string]Arm{ArmControl: {}},
	}
	manifest := &FlagsManifest{Defaults: map[string]any{}}
	r := &Runner{EvalDir: root, Driver: dbDriver{}, WorkParent: t.TempDir()}

	rec, err := r.ExecuteRun(context.Background(), exp, traj, trajDir, ArmControl, Arm{}, manifest, 1, "inv1")
	require.NoError(t, err)
	require.Equal(t, OutcomePass, rec.Outcome)
	require.NotEmpty(t, rec.SessionDB)
	require.Empty(t, rec.CallMetricsError)
	require.NotNil(t, rec.CallMetrics)
	require.Equal(t, 1, rec.CallMetrics.Requests)
	require.Equal(t, 1, rec.CallMetrics.Calls)
	require.Equal(t, 1, rec.CallMetrics.DiscoveryCallsBeforeWrite)
}

// TestCallMetricsCoveragePins the flag-invariant registration rule:
// call_metrics fields derivable in both arms are predicates; map_* and
// question_* can never be.
func TestCallMetricsCoverage(t *testing.T) {
	t.Parallel()
	op, field, err := ParseCoverageKey("min_call_metrics.requests")
	require.NoError(t, err)
	require.Equal(t, "min", op)
	require.Equal(t, "call_metrics.requests", field)

	for _, key := range []string{
		"min_call_metrics.map_calls",
		"max_call_metrics.question_calls",
		"min_call_metrics.wrong_pointer_events",
	} {
		_, _, err := ParseCoverageKey(key)
		require.Error(t, err, "%s must not be a coverage predicate", key)
	}

	// Absent analysis starves predicates in BOTH directions — min_*
	// and max_* are both inconclusive, never pass-on-missing.
	for _, cov := range []Coverage{
		{"min_call_metrics.requests": 1},
		{"max_call_metrics.rereads": 3},
	} {
		met, err := CoverageMet(cov, &RunRecord{})
		require.NoError(t, err)
		require.False(t, met)
	}
}

func TestAnalyzeSessionDB_SessionSelect(t *testing.T) {
	t.Parallel()
	dbPath := writeFixtureDB(t)
	// An explicit SessionID scopes analysis — here to the child session
	// the default picker would skip.
	cm, err := AnalyzeSessionDB(context.Background(), dbPath, AnalyzeOptions{
		SessionID: "s2",
		Workdir:   "/w",
	})
	require.NoError(t, err)
	require.Equal(t, "s2", cm.SessionID)
	require.Equal(t, 1, cm.Requests)
	require.Equal(t, 1, cm.Calls)
	require.Equal(t, 0, cm.FirstWriteIndex) // The child's edit is seq 0.
}
