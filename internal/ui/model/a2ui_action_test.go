package model

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/charmbracelet/crush/internal/ui/textarea"

	"github.com/charmbracelet/crush/internal/agent/tools"
	mcptools "github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/joestump-agent/a2tea/event"
	"github.com/stretchr/testify/require"
	a2ui "github.com/tmc/a2ui"
)

// a2uiActionWorkspace stubs the workspace calls the A2UI action round-trip
// touches, recording a2ui_action / a2ui_error calls and serving canned
// responses.
type a2uiActionWorkspace struct {
	workspace.Workspace

	// toolCalls records (name, toolName) pairs in call order.
	toolCalls [][2]string
	// lastArgs records the args of the most recent call.
	lastArgs map[string]any
	// response is the canned CallMCPTool result.
	response workspace.MCPToolCallResult
	// err is returned from CallMCPTool when set.
	err error
	// lastPrompt records the content of the most recent AgentRun, so the
	// fallback path's submission prompt can be asserted on.
	lastPrompt string
	// a2uiTools maps an MCP server name to the a2ui_* tools it exposes,
	// mirroring what a real workspace publishes in ClientInfo.A2UITools.
	a2uiTools map[string][]string
}

// serveA2UITools makes the workspace report that mcpName exposes the given
// a2ui_* tools, the way a connected server's published state would.
func (w *a2uiActionWorkspace) serveA2UITools(mcpName string, toolNames ...string) {
	if w.a2uiTools == nil {
		w.a2uiTools = map[string][]string{}
	}
	w.a2uiTools[mcpName] = toolNames
}

func (w *a2uiActionWorkspace) MCPGetStates() map[string]mcptools.ClientInfo {
	out := make(map[string]mcptools.ClientInfo, len(w.a2uiTools))
	for name, toolNames := range w.a2uiTools {
		out[name] = mcptools.ClientInfo{
			Name:      name,
			State:     mcptools.StateConnected,
			A2UITools: toolNames,
		}
	}
	return out
}

func (w *a2uiActionWorkspace) AgentIsReady() bool     { return true }
func (w *a2uiActionWorkspace) AgentReadyErr() error   { return nil }
func (w *a2uiActionWorkspace) Config() *config.Config { return nil }

func (w *a2uiActionWorkspace) AgentRun(_ context.Context, _, content string, _ ...message.Attachment) error {
	w.lastPrompt = content
	return nil
}

func (w *a2uiActionWorkspace) CallMCPTool(_ context.Context, name, toolName string, args map[string]any) (workspace.MCPToolCallResult, error) {
	w.toolCalls = append(w.toolCalls, [2]string{name, toolName})
	w.lastArgs = args
	return w.response, w.err
}

// a2uiActionForm is an MCP-served surface with a submit button carrying a
// server-side action, plus a TextField whose value feeds the action context.
const a2uiActionForm = `{"version":"v0.9","updateComponents":{"surfaceId":"booking","components":[` +
	`{"component":"Card","id":"root","child":"col"},` +
	`{"component":"Column","id":"col","children":["start","btn-confirm"]},` +
	`{"component":"TextField","id":"start","label":"Start","value":"2026-03-20"},` +
	`{"component":"Button","id":"btn-confirm","child":"btn-confirm-t","action":{"event":{"name":"confirm_booking"}}},` +
	`{"component":"Text","id":"btn-confirm-t","text":"Confirm"}` +
	`]}}`

// newA2UIActionUI builds a UI whose chat holds a tool message item carrying
// an MCP-served A2UI surface with provenance registered to mcpName.
func newA2UIActionUI(t *testing.T, ws *a2uiActionWorkspace, mcpName string) *UI {
	t.Helper()
	m, _ := newA2UIActionUIItem(t, ws, mcpName)
	return m
}

// newA2UIActionUIItem is newA2UIActionUI, also handing back the tool item so
// a test can assert on what the surface actually renders.
func newA2UIActionUIItem(t *testing.T, ws *a2uiActionWorkspace, mcpName string) (*UI, chat.ToolMessageItem) {
	t.Helper()
	com := common.DefaultCommon(ws)
	m := &UI{
		com:      com,
		status:   NewStatus(com, nil),
		chat:     NewChat(com, config.ScrollbarDefault),
		textarea: textarea.New(),
		state:    uiChat,
		focus:    uiFocusMain,
		width:    140,
		height:   45,
		session:  &session.Session{ID: "sess-1"},
	}

	meta, err := json.Marshal(tools.ReadMCPResourceResponseMetadata{
		A2UISurfaces:         []string{"<a2ui-json>" + a2uiActionForm + "</a2ui-json>"},
		MCPSurfaceProvenance: []string{mcpName},
	})
	require.NoError(t, err)

	item := chat.NewMCPToolMessageItem(com.Styles, message.ToolCall{
		ID:       "call-1",
		Name:     "mcp_" + mcpName + "_get_form",
		Input:    `{}`,
		Finished: true,
	}, &message.ToolResult{
		ToolCallID: "call-1",
		Content:    "form",
		Metadata:   string(meta),
	}, false)
	m.chat.AppendMessages(item)
	// Render once so the item builds its live surface models and registers
	// provenance.
	_ = item.RawRender(100)
	return m, item
}

func TestHandleA2UIButtonClickedMCPSurfaceRoundTrips(t *testing.T) {
	t.Parallel()

	// Surface provenance is recorded in a process-global registry; a
	// per-test server name keeps parallel tests from clobbering it.
	srv := t.Name()

	ws := &a2uiActionWorkspace{
		response: workspace.MCPToolCallResult{Content: "Booking confirmed"},
	}
	m := newA2UIActionUI(t, ws, srv)
	ws.serveA2UITools(srv, "a2ui_action")

	cmd := m.handleA2UIButtonClicked(event.ButtonClicked{
		Source: event.Source{ComponentID: "btn-confirm", SurfaceID: "booking"},
		ID:     "btn-confirm",
		Action: &a2ui.EventAction{Name: "confirm_booking"},
	})
	require.NotNil(t, cmd, "an MCP surface with a2ui_action must round-trip")
	msg := cmd()
	result, ok := msg.(a2uiActionResultMsg)
	require.True(t, ok, "the cmd must produce an a2uiActionResultMsg")
	require.NoError(t, result.err)
	require.Equal(t, "booking", result.surfaceID)

	// The server got the action name and the surface's field values as the
	// context.
	require.Len(t, ws.toolCalls, 1)
	require.Equal(t, [2]string{srv, "a2ui_action"}, ws.toolCalls[0])
	require.Equal(t, "confirm_booking", ws.lastArgs["name"])
	ctx, ok := ws.lastArgs["context"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "2026-03-20", ctx["start"])

	// The surface is NOT retired — MCP surfaces are long-lived.
	_, found := m.chat.A2UISurfaceFieldValues("booking")
	require.True(t, found, "the surface must stay live after the round-trip")
}

func TestHandleA2UIButtonClickedMCPSurfaceWithoutActionToolFallsBack(t *testing.T) {
	t.Parallel()

	// Surface provenance is recorded in a process-global registry; a
	// per-test server name keeps parallel tests from clobbering it.
	srv := t.Name()

	ws := &a2uiActionWorkspace{}
	m := newA2UIActionUI(t, ws, srv)
	// Server serves surfaces but NOT a2ui_action.
	ws.serveA2UITools(srv, "some_other_tool")

	cmd := m.handleA2UIButtonClicked(event.ButtonClicked{
		Source: event.Source{ComponentID: "btn-confirm", SurfaceID: "booking"},
		ID:     "btn-confirm",
		Action: &a2ui.EventAction{Name: "confirm_booking"},
	})
	require.NotNil(t, cmd, "fallback must start an agent turn")
	runCmdTree(cmd)

	require.Empty(t, ws.toolCalls, "no a2ui_action call without the tool")

	// The prompt must carry what the user typed. RetireA2UISurface only
	// matches assistant items, so an MCP surface's values have to come from
	// the tool item that holds them — otherwise the whole form is silently
	// dropped before the agent ever sees it.
	require.Contains(t, ws.lastPrompt, "2026-03-20",
		"the fallback submission must carry the surface's field values")
	require.Contains(t, ws.lastPrompt, "confirm_booking")
}

func TestHandleA2UIActionResultAppliesSurfacePayload(t *testing.T) {
	t.Parallel()

	// Surface provenance is recorded in a process-global registry; a
	// per-test server name keeps parallel tests from clobbering it.
	srv := t.Name()

	ws := &a2uiActionWorkspace{}
	m, item := newA2UIActionUIItem(t, ws, srv)
	ws.serveA2UITools(srv, "a2ui_action")

	require.Contains(t, item.RawRender(100), "Confirm", "precondition: the original label renders")

	// A response carrying an A2UI payload updates the surface in place.
	update := `{"version":"v0.9","updateComponents":{"surfaceId":"booking","components":[` +
		`{"component":"Text","id":"btn-confirm-t","text":"Booked!"}]}}`
	cmd := m.handleA2UIActionResult(a2uiActionResultMsg{
		surfaceID: "booking",
		mcpName:   srv,
		surfaces:  []string{update},
	})
	require.Nil(t, cmd)

	// The update actually reached the live surface — not just "no error".
	rendered := item.RawRender(100)
	require.Contains(t, rendered, "Booked!", "the payload must update the surface in place")
	require.NotContains(t, rendered, "Confirm", "the old label must be replaced")

	// The surface still exists after the in-place update, and the edited
	// field the server did not touch is preserved.
	values, found := m.chat.A2UISurfaceFieldValues("booking")
	require.True(t, found)
	require.Equal(t, "2026-03-20", values["start"], "untouched field values must survive")
}

func TestHandleA2UIActionResultInvalidPayloadReportsToServer(t *testing.T) {
	t.Parallel()

	// Surface provenance is recorded in a process-global registry; a
	// per-test server name keeps parallel tests from clobbering it.
	srv := t.Name()

	ws := &a2uiActionWorkspace{}
	m := newA2UIActionUI(t, ws, srv)
	ws.serveA2UITools(srv, "a2ui_action", "a2ui_error")

	// A malformed payload must round-trip an a2ui_error back to the server:
	// the command reportA2UIError builds has to actually reach Bubble Tea.
	cmd := m.handleA2UIActionResult(a2uiActionResultMsg{
		surfaceID: "booking",
		mcpName:   srv,
		surfaces:  []string{`{"version":"v0.9","updateComponents":{`},
	})
	require.NotNil(t, cmd, "a malformed payload must produce an a2ui_error command")
	runCmdTree(cmd)

	require.Len(t, ws.toolCalls, 1)
	require.Equal(t, [2]string{srv, "a2ui_error"}, ws.toolCalls[0])
	// Truncated JSON scans as prose rather than failing outright, so it
	// lands as INVALID_PAYLOAD (parsed, but no server messages) rather than
	// INVALID_JSON. Either way it must not be a silent no-op.
	require.Equal(t, "INVALID_PAYLOAD", ws.lastArgs["code"])
	require.Equal(t, "booking", ws.lastArgs["surfaceId"])
}

func TestHandleA2UIActionResultHonorsPayloadSurfaceID(t *testing.T) {
	t.Parallel()

	// Surface provenance is recorded in a process-global registry; a
	// per-test server name keeps parallel tests from clobbering it.
	srv := t.Name()

	ws := &a2uiActionWorkspace{}
	m, item := newA2UIActionUIItem(t, ws, srv)
	ws.serveA2UITools(srv, "a2ui_action", "a2ui_error")

	// The server answers the "booking" click by targeting a DIFFERENT
	// surface. That payload must not be forced onto "booking".
	update := `{"version":"v0.9","updateComponents":{"surfaceId":"receipt","components":[` +
		`{"component":"Text","id":"btn-confirm-t","text":"Booked!"}]}}`
	cmd := m.handleA2UIActionResult(a2uiActionResultMsg{
		surfaceID: "booking",
		mcpName:   srv,
		surfaces:  []string{update},
	})
	require.NotNil(t, cmd, "a payload for an unheld surface must be reported back")
	runCmdTree(cmd)

	require.Contains(t, item.RawRender(100), "Confirm",
		"a payload aimed at another surface must not corrupt the clicked one")
	require.Len(t, ws.toolCalls, 1)
	require.Equal(t, [2]string{srv, "a2ui_error"}, ws.toolCalls[0])
	require.Equal(t, "SURFACE_NOT_FOUND", ws.lastArgs["code"])
	require.Equal(t, "receipt", ws.lastArgs["surfaceId"])
}

func TestHandleA2UIActionResultDeletedSurfaceGoesAway(t *testing.T) {
	t.Parallel()

	// Surface provenance is recorded in a process-global registry; a
	// per-test server name keeps parallel tests from clobbering it.
	srv := t.Name()

	ws := &a2uiActionWorkspace{}
	m := newA2UIActionUI(t, ws, srv)
	ws.serveA2UITools(srv, "a2ui_action", "a2ui_error")

	// The server dismisses the form. The surface must be released, not left
	// behind as a dead widget that still takes focus and swallows keys —
	// and a delete is not an error worth reporting back.
	cmd := m.handleA2UIActionResult(a2uiActionResultMsg{
		surfaceID: "booking",
		mcpName:   srv,
		surfaces:  []string{`{"version":"v0.9","deleteSurface":{"surfaceId":"booking"}}`},
	})
	runCmdTree(cmd)

	_, found := m.chat.A2UISurfaceFieldValues("booking")
	require.False(t, found, "a deleted surface must no longer be live")
	require.Empty(t, ws.toolCalls, "deleting a surface is not an error to report")
}

func TestHandleA2UIActionResultErrorReports(t *testing.T) {
	t.Parallel()

	// Surface provenance is recorded in a process-global registry; a
	// per-test server name keeps parallel tests from clobbering it.
	srv := t.Name()

	ws := &a2uiActionWorkspace{}
	m := newA2UIActionUI(t, ws, srv)

	cmd := m.handleA2UIActionResult(a2uiActionResultMsg{
		surfaceID: "booking",
		mcpName:   srv,
		err:       context.DeadlineExceeded,
	})
	require.NotNil(t, cmd, "an error must surface as a note")
}

func TestReportA2UIErrorCallsErrorTool(t *testing.T) {
	t.Parallel()

	// Surface provenance is recorded in a process-global registry; a
	// per-test server name keeps parallel tests from clobbering it.
	srv := t.Name()

	ws := &a2uiActionWorkspace{}
	m := newA2UIActionUI(t, ws, srv)
	ws.serveA2UITools(srv, "a2ui_error")

	// An empty server name exercises the surface-owner resolution path.
	cmd := m.reportA2UIError("", "booking", "INVALID_JSON", "bad payload")
	require.NotNil(t, cmd)
	msg := cmd()
	require.Nil(t, msg, "a successful a2ui_error call produces no message")

	require.Len(t, ws.toolCalls, 1)
	require.Equal(t, [2]string{srv, "a2ui_error"}, ws.toolCalls[0])
	require.Equal(t, "INVALID_JSON", ws.lastArgs["code"])
	require.Equal(t, "booking", ws.lastArgs["surfaceId"])
}

// assistantCollidingForm is an assistant-authored surface that happens to use
// the same surface ID as the MCP-served form above. Surface IDs are only
// unique within whatever authored them, and both the assistant and MCP
// servers habitually reuse a small set of names ("default", "form"), so the
// collision is ordinary rather than contrived.
const assistantCollidingForm = `<a2ui-json>{"version":"v0.9","updateComponents":{"surfaceId":"booking","components":[` +
	`{"component":"Card","id":"root","child":"col"},` +
	`{"component":"Column","id":"col","children":["who","btn-go"]},` +
	`{"component":"TextField","id":"who","label":"Who","value":"Joe"},` +
	`{"component":"Button","id":"btn-go","child":"btn-go-t","action":{"event":{"name":"go"}}},` +
	`{"component":"Text","id":"btn-go-t","text":"Go"}` +
	`]}}</a2ui-json>`

// TestHandleA2UIButtonClickedAssistantSurfaceWinsOverMCPProvenance pins that
// an assistant-authored surface is never routed to an MCP server, even when
// its ID collides with one that server owns.
//
// Assistant items do not implement chat.A2UISurfaceItem and never appear in
// the global surfaceID->server registry, so both provenance lookups used to
// resolve the collision in the MCP server's favour: the user's form values
// were shipped to that server's a2ui_action tool, the surface was never
// retired, and no agent turn ever started — the button just looked dead.
func TestHandleA2UIButtonClickedAssistantSurfaceWinsOverMCPProvenance(t *testing.T) {
	t.Parallel()

	// Surface provenance is recorded in a process-global registry; a
	// per-test server name keeps parallel tests from clobbering it.
	srv := t.Name()

	ws := &a2uiActionWorkspace{
		response: workspace.MCPToolCallResult{Content: "should never be called"},
	}
	// Registers "booking" -> srv in both the item and the global registry.
	m := newA2UIActionUI(t, ws, srv)
	ws.serveA2UITools(srv, "a2ui_action")

	// The assistant now emits its own form reusing the same surface ID.
	msg := &message.Message{
		ID:   "assistant-form",
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: "Confirm:\n\n" + assistantCollidingForm},
		},
	}
	item, ok := chat.NewAssistantMessageItem(m.com.Styles, msg).(*chat.AssistantMessageItem)
	require.True(t, ok)
	m.chat.AppendMessages(item)
	_ = item.RawRender(80)
	require.True(t, m.chat.HasAssistantA2UISurface("booking"),
		"the assistant item must hold a live surface with the colliding ID")

	cmd := m.handleA2UIButtonClicked(event.ButtonClicked{
		Source: event.Source{ComponentID: "btn-go", SurfaceID: "booking"},
		ID:     "btn-go",
		Action: &a2ui.EventAction{Name: "go"},
	})
	require.NotNil(t, cmd, "the submission must go somewhere")
	runCmdTree(cmd)

	require.Empty(t, ws.toolCalls,
		"an assistant surface must never round-trip to an MCP server")
	require.NotEmpty(t, ws.lastPrompt, "the submission must start an agent turn")
}
