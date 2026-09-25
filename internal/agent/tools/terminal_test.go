package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/shell"
	"github.com/charmbracelet/crush/internal/terminal"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"
)

// fakeTerminalService records display requests and reports an attached UI
// unless told otherwise.
type fakeTerminalService struct {
	available bool
	shown     []terminal.Request
}

func newFakeTerminalService() *fakeTerminalService {
	return &fakeTerminalService{available: true}
}

func (f *fakeTerminalService) Subscribe(context.Context) <-chan pubsub.Event[terminal.Request] {
	return make(<-chan pubsub.Event[terminal.Request])
}

func (f *fakeTerminalService) Show(_ context.Context, req terminal.Request) (terminal.Request, error) {
	if !f.available {
		return terminal.Request{}, terminal.ErrUnavailable
	}
	f.shown = append(f.shown, req)
	return req, nil
}

func (f *fakeTerminalService) SetAvailable(available bool) { f.available = available }

func (f *fakeTerminalService) Available() bool { return f.available }

func resetTerminalManager(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		mgr := shell.GetInteractiveSessionManager()
		for _, id := range mgr.List() {
			if session, ok := mgr.Get(id); ok {
				_ = session.Kill()
				_ = session.Close()
			}
			mgr.Remove(id)
		}
	})
}

func runTerminalTool(t *testing.T, tool fantasy.AgentTool, params TerminalParams) fantasy.ToolResponse {
	t.Helper()

	input, err := json.Marshal(params)
	require.NoError(t, err)

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	resp, err := tool.Run(ctx, fantasy.ToolCall{
		ID:    "test-call",
		Name:  TerminalToolName,
		Input: string(input),
	})
	require.NoError(t, err)
	return resp
}

func newTerminalToolForTest(t *testing.T) (fantasy.AgentTool, *fakeTerminalService) {
	t.Helper()

	workingDir := t.TempDir()
	permissions := &mockBashPermissionService{Broker: pubsub.NewBroker[permission.PermissionRequest]()}
	terminals := newFakeTerminalService()

	return NewTerminalTool(permissions, terminals, workingDir, workingDir), terminals
}

// agentDriven marks a start the agent intends to drive itself, which must
// not block waiting for the user.
func agentDriven() *bool {
	noWait := false
	return &noWait
}

func TestTerminalToolStart(t *testing.T) {
	resetTerminalManager(t)

	tool, terminals := newTerminalToolForTest(t)

	resp := runTerminalTool(t, tool, TerminalParams{
		Action:  "start",
		Command: "printf 'prompt-answer\\n'",
	})

	require.False(t, resp.IsError)
	require.Contains(t, resp.Content, "Terminal session")
	require.Contains(t, resp.Content, "finished")
	require.Contains(t, resp.Content, "prompt-answer", "the transcript should be returned")
	require.Len(t, terminals.shown, 1)

	var meta TerminalResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
	require.Equal(t, "start", meta.Action)
	require.NotEmpty(t, meta.SessionID)
	require.True(t, meta.Exited)

	session, ok := shell.GetInteractiveSessionManager().Get(meta.SessionID)
	require.True(t, ok)
	t.Cleanup(func() {
		_ = session.Kill()
		_ = session.Close()
	})
	require.Same(t, session, terminals.shown[0].Session)
	require.False(t, terminals.shown[0].AgentDriven, "a waiting start hands the terminal to the user")
}

func TestTerminalStartAgentDrivenMarksRequest(t *testing.T) {
	resetTerminalManager(t)

	tool, terminals := newTerminalToolForTest(t)

	resp := runTerminalTool(t, tool, TerminalParams{
		Action:  "start",
		Command: "sleep 30",
		Wait:    agentDriven(),
	})
	require.False(t, resp.IsError)
	require.Len(t, terminals.shown, 1)
	require.True(t, terminals.shown[0].AgentDriven, "a non-waiting start gives the terminal to the agent")
	require.Contains(t, resp.Content, "read-only")
}

func TestTerminalStartWaitsForUserWork(t *testing.T) {
	resetTerminalManager(t)

	tool, _ := newTerminalToolForTest(t)

	resp := runTerminalTool(t, tool, TerminalParams{
		Action:  "start",
		Command: "sh -c 'printf \"user-work\\n\"; exit 3'",
	})
	require.False(t, resp.IsError)
	require.Contains(t, resp.Content, "user-work", "the transcript should be returned")
	require.Contains(t, resp.Content, "exit code 3")

	var meta TerminalResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
	require.True(t, meta.Exited)
	require.Equal(t, 3, meta.ExitCode)
}

func TestTerminalStartUnblocksWhenUserClosesTerminal(t *testing.T) {
	resetTerminalManager(t)

	tool, _ := newTerminalToolForTest(t)

	type result struct {
		resp fantasy.ToolResponse
		err  error
	}
	done := make(chan result, 1)
	go func() {
		input, _ := json.Marshal(TerminalParams{Action: "start", Command: "sleep 60"})
		ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
		resp, err := tool.Run(ctx, fantasy.ToolCall{
			ID:    "test-call",
			Name:  TerminalToolName,
			Input: string(input),
		})
		done <- result{resp, err}
	}()

	// Wait for the session to register, then end it the way the UI does
	// when the user quits the docked terminal.
	var session *shell.InteractiveSession
	require.Eventually(t, func() bool {
		for _, id := range shell.GetInteractiveSessionManager().List() {
			if s, ok := shell.GetInteractiveSessionManager().Get(id); ok {
				session = s
				return true
			}
		}
		return false
	}, 10*time.Second, 20*time.Millisecond)

	require.NoError(t, session.Kill())
	_ = session.Close()

	select {
	case r := <-done:
		require.NoError(t, r.err)
		require.False(t, r.resp.IsError)
		require.Contains(t, r.resp.Content, "finished")
	case <-time.After(15 * time.Second):
		t.Fatal("a waiting start did not unblock when the session was killed")
	}
}

func TestTerminalStartWaitBudgetElapses(t *testing.T) {
	resetTerminalManager(t)

	old := terminalStartWaitBudget
	terminalStartWaitBudget = 100 * time.Millisecond
	t.Cleanup(func() { terminalStartWaitBudget = old })

	tool, _ := newTerminalToolForTest(t)

	resp := runTerminalTool(t, tool, TerminalParams{Action: "start", Command: "sleep 60"})
	require.False(t, resp.IsError)
	require.Contains(t, resp.Content, "still running")

	var meta TerminalResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
	require.False(t, meta.Exited)
}

func TestTerminalToolStartRequiresCommand(t *testing.T) {
	resetTerminalManager(t)

	tool, _ := newTerminalToolForTest(t)
	resp := runTerminalTool(t, tool, TerminalParams{Action: "start"})
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "missing command")
}

func TestTerminalToolStartBusy(t *testing.T) {
	resetTerminalManager(t)

	tool, _ := newTerminalToolForTest(t)

	first := runTerminalTool(t, tool, TerminalParams{Action: "start", Command: "sleep 30", Wait: agentDriven()})
	require.False(t, first.IsError)

	second := runTerminalTool(t, tool, TerminalParams{Action: "start", Command: "sleep 30", Wait: agentDriven()})
	require.True(t, second.IsError)
	require.Contains(t, second.Content, "already running")
}

func TestTerminalToolStartBlocked(t *testing.T) {
	resetTerminalManager(t)

	tool, terminals := newTerminalToolForTest(t)
	resp := runTerminalTool(t, tool, TerminalParams{Action: "start", Command: "sudo shutdown"})
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "blocked")
	require.Empty(t, terminals.shown)
}

func TestTerminalToolUnavailable(t *testing.T) {
	resetTerminalManager(t)

	tool, terminals := newTerminalToolForTest(t)
	terminals.SetAvailable(false)

	resp := runTerminalTool(t, tool, TerminalParams{Action: "start", Command: "printf hi"})
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "unavailable")
}

func TestTerminalToolUnknownAction(t *testing.T) {
	resetTerminalManager(t)

	tool, _ := newTerminalToolForTest(t)
	resp := runTerminalTool(t, tool, TerminalParams{Action: "restart"})
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "unknown action")
}

// TestTerminalPasteUsesBracketedPaste checks the paste action end to end:
// the child enables bracketed paste, and the pasted text arrives wrapped in
// the paste markers so multi-line input is inserted, not executed.
func TestTerminalPasteUsesBracketedPaste(t *testing.T) {
	resetTerminalManager(t)

	tool, _ := newTerminalToolForTest(t)
	resp := runTerminalTool(t, tool, TerminalParams{
		Action:  "start",
		Command: `sh -c 'stty raw -echo; printf "\033[?2004hready"; dd bs=1 count=19 2>/dev/null | od -An -tx1; stty sane'`,
		Wait:    agentDriven(),
	})

	var meta TerminalResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))

	// Paste only once the mode is on; the ready marker proves it.
	require.Eventually(t, func() bool {
		read := runTerminalTool(t, tool, TerminalParams{Action: "read", SessionID: meta.SessionID})
		return strings.Contains(read.Content, "ready")
	}, 15*time.Second, 50*time.Millisecond)

	paste := runTerminalTool(t, tool, TerminalParams{
		Action:    "paste",
		SessionID: meta.SessionID,
		Text:      "one\ntwo",
	})
	require.False(t, paste.IsError)

	var pasteMeta TerminalResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(paste.Metadata), &pasteMeta))
	require.Equal(t, "paste", pasteMeta.Action)

	require.Eventually(t, func() bool {
		read := runTerminalTool(t, tool, TerminalParams{Action: "read", SessionID: meta.SessionID})
		screen := strings.Join(strings.Fields(read.Content), " ")
		return strings.Contains(screen, "1b 5b 32 30 30 7e") && strings.Contains(screen, "1b 5b 32 30 31 7e")
	}, 15*time.Second, 50*time.Millisecond, "the pasted bytes carry the bracketed paste markers")
}

func TestTerminalPasteNeedsText(t *testing.T) {
	resetTerminalManager(t)

	tool, _ := newTerminalToolForTest(t)
	resp := runTerminalTool(t, tool, TerminalParams{
		Action:  "start",
		Command: "sleep 30",
		Wait:    agentDriven(),
	})

	var meta TerminalResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))

	paste := runTerminalTool(t, tool, TerminalParams{Action: "paste", SessionID: meta.SessionID})
	require.True(t, paste.IsError)
	require.Contains(t, paste.Content, "nothing to paste")
}

func TestTerminalRead(t *testing.T) {
	resetTerminalManager(t)

	tool, _ := newTerminalToolForTest(t)

	resp := runTerminalTool(t, tool, TerminalParams{Action: "start", Command: "printf 'read-me\\n'"})
	require.False(t, resp.IsError)

	var meta TerminalResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))

	read := runTerminalTool(t, tool, TerminalParams{Action: "read", SessionID: meta.SessionID, WaitSeconds: 10})
	require.False(t, read.IsError)
	require.Contains(t, read.Content, "read-me")
	require.Contains(t, read.Content, "exited")
	require.Contains(t, read.Content, "Screen is", "the logical size is reported")
}

// TestTerminalReadReportsHiddenCursor checks that a read reports the cursor
// position even when the program hid its cursor: apps move the hidden
// cursor while drawing, so the position still says where the program is
// focused.
func TestTerminalReadReportsHiddenCursor(t *testing.T) {
	resetTerminalManager(t)

	tool, _ := newTerminalToolForTest(t)
	resp := runTerminalTool(t, tool, TerminalParams{
		Action:  "start",
		Command: "sh -c 'printf \"\\033[?25l\"; sleep 30'",
		Wait:    agentDriven(),
	})

	var meta TerminalResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))

	// The child needs a moment to emit the escape sequence.
	require.Eventually(t, func() bool {
		read := runTerminalTool(t, tool, TerminalParams{Action: "read", SessionID: meta.SessionID})
		return !read.IsError && strings.Contains(read.Content, "Cursor at row") && strings.Contains(read.Content, "hid its cursor")
	}, 10*time.Second, 100*time.Millisecond)
}

// TestTerminalWriteReportsGeometry checks that a write result carries the
// session geometry so the agent knows where its typing landed.
func TestTerminalWriteReportsGeometry(t *testing.T) {
	resetTerminalManager(t)

	tool, _ := newTerminalToolForTest(t)
	resp := runTerminalTool(t, tool, TerminalParams{
		Action:  "start",
		Command: "sleep 30",
		Wait:    agentDriven(),
	})

	var meta TerminalResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))

	write := runTerminalTool(t, tool, TerminalParams{Action: "write", SessionID: meta.SessionID, Text: "hi\r"})
	require.False(t, write.IsError)
	require.Contains(t, write.Content, "Cursor at row", "write results report the cursor")
	require.Contains(t, write.Content, "Screen is", "write results report the size")
}

func TestTerminalReadWaitForExit(t *testing.T) {
	resetTerminalManager(t)

	tool, _ := newTerminalToolForTest(t)
	resp := runTerminalTool(t, tool, TerminalParams{Action: "start", Command: `sh -c 'sleep 1; echo slow-done'`})
	require.False(t, resp.IsError)

	var meta TerminalResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))

	read := runTerminalTool(t, tool, TerminalParams{Action: "read", SessionID: meta.SessionID, WaitSeconds: 10})
	require.False(t, read.IsError)
	require.Contains(t, read.Content, "slow-done")
}

func TestTerminalReadNoSession(t *testing.T) {
	resetTerminalManager(t)

	tool, _ := newTerminalToolForTest(t)
	resp := runTerminalTool(t, tool, TerminalParams{Action: "read"})
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "no terminal session is running")
}

func TestTerminalReadNotFound(t *testing.T) {
	resetTerminalManager(t)

	tool, _ := newTerminalToolForTest(t)
	resp := runTerminalTool(t, tool, TerminalParams{Action: "read", SessionID: "0FF"})
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "not found")
}

func TestTerminalWrite(t *testing.T) {
	resetTerminalManager(t)

	tool, _ := newTerminalToolForTest(t)

	resp := runTerminalTool(t, tool, TerminalParams{Action: "start", Command: `read line; echo "got:$line"`, Wait: agentDriven()})
	require.False(t, resp.IsError)

	var meta TerminalResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))

	write := runTerminalTool(t, tool, TerminalParams{Action: "write", SessionID: meta.SessionID, Text: "hello-agent\r"})
	require.False(t, write.IsError)
	require.Contains(t, write.Content, "Sent")
	require.Contains(t, write.Content, "<screen>", "writes return the settled screen")

	read := runTerminalTool(t, tool, TerminalParams{Action: "read", SessionID: meta.SessionID, WaitSeconds: 10})
	require.False(t, read.IsError)
	require.Contains(t, read.Content, "got:hello-agent")
}

func TestTerminalWriteExited(t *testing.T) {
	resetTerminalManager(t)

	tool, _ := newTerminalToolForTest(t)
	resp := runTerminalTool(t, tool, TerminalParams{Action: "start", Command: "true"})
	require.False(t, resp.IsError)

	var meta TerminalResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))

	session, ok := shell.GetInteractiveSessionManager().Get(meta.SessionID)
	require.True(t, ok)
	select {
	case <-session.Done():
	case <-time.After(15 * time.Second):
		t.Fatal("session did not exit")
	}

	write := runTerminalTool(t, tool, TerminalParams{Action: "write", SessionID: meta.SessionID, Text: "nope\r"})
	require.True(t, write.IsError)
	require.Contains(t, write.Content, "already exited")
}

func TestTerminalKill(t *testing.T) {
	resetTerminalManager(t)

	tool, _ := newTerminalToolForTest(t)
	resp := runTerminalTool(t, tool, TerminalParams{Action: "start", Command: `sh -c 'echo kill-me; sleep 60'`, Wait: agentDriven()})
	require.False(t, resp.IsError)

	var meta TerminalResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))

	session, ok := shell.GetInteractiveSessionManager().Get(meta.SessionID)
	require.True(t, ok)

	require.Eventually(t, func() bool {
		return strings.Contains(session.CaptureText(), "kill-me")
	}, 10*time.Second, 50*time.Millisecond)

	kill := runTerminalTool(t, tool, TerminalParams{Action: "kill", SessionID: meta.SessionID})
	require.False(t, kill.IsError)
	require.Contains(t, kill.Content, "Session terminated")
	require.Contains(t, kill.Content, "kill-me")
	require.True(t, session.Exited())

	// The session stays registered so the transcript remains readable.
	_, ok = shell.GetInteractiveSessionManager().Get(meta.SessionID)
	require.True(t, ok)
}

func TestTerminalWriteKeysNavigateTUIs(t *testing.T) {
	resetTerminalManager(t)

	tool, _ := newTerminalToolForTest(t)

	// The child enables application cursor keys mode and reads three raw
	// bytes; a plain write would send the wrong escape sequence, while a
	// semantic key is encoded for the mode the child asked for.
	resp := runTerminalTool(t, tool, TerminalParams{
		Action:  "start",
		Command: "sh -c 'stty raw -echo; printf \"\\033[?1h\"; dd bs=1 count=3 2>/dev/null | od -An -tx1; printf \"\\033[?1l\"; stty sane'",
		Wait:    agentDriven(),
	})
	require.False(t, resp.IsError)

	var meta TerminalResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))

	// The child must have reported its mode to the emulator before the
	// key is sent; give it a moment to write the sequence.
	time.Sleep(300 * time.Millisecond)

	write := runTerminalTool(t, tool, TerminalParams{Action: "write", SessionID: meta.SessionID, Keys: []string{"up"}})
	require.False(t, write.IsError)

	// od pads with extra spaces; normalize before matching.
	normalized := strings.Join(strings.Fields(write.Content), " ")
	require.Contains(t, normalized, "1b 4f 41", "application cursor keys encoding: %s", write.Content)
}

func TestTerminalWriteCtrlCInterrupts(t *testing.T) {
	resetTerminalManager(t)

	tool, _ := newTerminalToolForTest(t)
	resp := runTerminalTool(t, tool, TerminalParams{Action: "start", Command: "sleep 60", Wait: agentDriven()})
	require.False(t, resp.IsError)

	var meta TerminalResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))

	session, ok := shell.GetInteractiveSessionManager().Get(meta.SessionID)
	require.True(t, ok)

	write := runTerminalTool(t, tool, TerminalParams{Action: "write", SessionID: meta.SessionID, Keys: []string{"ctrl+c"}})
	require.False(t, write.IsError)

	select {
	case <-session.Done():
	case <-time.After(15 * time.Second):
		t.Fatal("session did not exit after ctrl+c")
	}
}

func TestTerminalWriteUnknownKey(t *testing.T) {
	resetTerminalManager(t)

	tool, _ := newTerminalToolForTest(t)
	resp := runTerminalTool(t, tool, TerminalParams{Action: "start", Command: "sleep 30", Wait: agentDriven()})
	require.False(t, resp.IsError)

	var meta TerminalResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))

	write := runTerminalTool(t, tool, TerminalParams{Action: "write", SessionID: meta.SessionID, Keys: []string{"banana"}})
	require.True(t, write.IsError)
	require.Contains(t, write.Content, "unknown key")
}

func TestTerminalWriteNothingToSend(t *testing.T) {
	resetTerminalManager(t)

	tool, _ := newTerminalToolForTest(t)
	resp := runTerminalTool(t, tool, TerminalParams{Action: "start", Command: "sleep 30", Wait: agentDriven()})
	require.False(t, resp.IsError)

	var meta TerminalResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))

	write := runTerminalTool(t, tool, TerminalParams{Action: "write", SessionID: meta.SessionID})
	require.True(t, write.IsError)
	require.Contains(t, write.Content, "nothing to send")
}

func TestTerminalWriteMouseReachesTUIs(t *testing.T) {
	resetTerminalManager(t)

	tool, _ := newTerminalToolForTest(t)

	// The child enables SGR mouse tracking and reads a click sequence.
	resp := runTerminalTool(t, tool, TerminalParams{
		Action:  "start",
		Command: "sh -c 'stty raw -echo; printf \"\\033[?1000h\\033[?1006h\"; dd bs=1 count=9 2>/dev/null | od -An -tx1; stty sane'",
		Wait:    agentDriven(),
	})
	require.False(t, resp.IsError)

	var meta TerminalResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))

	time.Sleep(300 * time.Millisecond)

	write := runTerminalTool(t, tool, TerminalParams{
		Action:    "write",
		SessionID: meta.SessionID,
		Mouse:     &TerminalMouse{Action: "click", Button: "left", X: 1, Y: 1},
	})
	require.False(t, write.IsError)

	// SGR left click at 1,1: ESC [ < 0 ; 1 ; 1 M
	normalized := strings.Join(strings.Fields(write.Content), " ")
	require.Contains(t, normalized, "1b 5b 3c 30 3b 31 3b 31 4d", "mouse click encoding: %s", write.Content)
}

func TestTerminalMouseValidation(t *testing.T) {
	t.Parallel()

	_, err := terminalMouseEvent(TerminalMouse{Action: "click"})
	require.Error(t, err)

	_, err = terminalMouseEvent(TerminalMouse{Action: "wheel", Button: "left"})
	require.Error(t, err)

	_, err = terminalMouseEvent(TerminalMouse{Action: "wiggle", Button: "left"})
	require.Error(t, err)

	_, err = terminalMouseEvent(TerminalMouse{Action: "", Button: "left"})
	require.Error(t, err)

	_, err = terminalMouseEvent(TerminalMouse{Action: "click", Button: "banana"})
	require.Error(t, err)

	event, err := terminalMouseEvent(TerminalMouse{Action: "click", Button: "left", X: 2, Y: 3})
	require.NoError(t, err)
	click, ok := event.(uv.MouseClickEvent)
	require.True(t, ok)
	require.Equal(t, 1, click.X, "1-based input becomes 0-based emulator coordinates")
	require.Equal(t, 2, click.Y)
	require.Equal(t, uv.MouseLeft, click.Button)
}
