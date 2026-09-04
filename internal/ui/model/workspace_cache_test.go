package model

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/ui/util"
)

// modeTrackingWorkspace extends countingWorkspace to track AgentMode
// state per session and verify the yolo flag syncs correctly.
type modeTrackingWorkspace struct {
	countingWorkspace

	modeBySession map[string]agent.AgentMode
}

func newModeTracking() *modeTrackingWorkspace {
	w := &modeTrackingWorkspace{
		countingWorkspace: countingWorkspace{
			ready: true,
			yolo:  false,
		},
		modeBySession: map[string]agent.AgentMode{"s1": agent.AgentModeBuild},
	}
	return w
}

func (w *modeTrackingWorkspace) AgentMode(id string) agent.AgentMode {
	return w.modeBySession[id]
}

func (w *modeTrackingWorkspace) SetAgentMode(id string, mode agent.AgentMode) {
	w.modeBySession[id] = mode
}

func (w *modeTrackingWorkspace) PermissionSetSkipRequests(skip bool) {
	w.permSetCalls++
	w.yolo = skip
}

// TestCycleAgentModeNilSessionRegression: guards the crash where Shift+Tab
// into Plan/Build/Yolo fired before the session was wired, leaving
// m.session == nil. Both cycleAgentMode and setAgentMode must short-circuit
// without panicking.
func TestCycleAgentModeNilSessionRegression(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true}
	m := newBusyUI(ws)
	m.session = nil // regression condition

	require.NotPanics(t, func() {
		_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
		runCmds(m, cmd)
	})
	require.Equal(t, 0, ws.permSetCalls,
		"nil session must not touch PermissionSkipRequests")

	require.NotPanics(t, func() { m.setAgentMode(agent.AgentModeYolo) })
	require.Equal(t, 0, ws.permSetCalls,
		"nil session must not touch PermissionSkipRequests")
}

// TestCycleAgentModeFullLoop exercises the Build -> Yolo -> Plan -> Build
// cycle and asserts the workspace mode + permission-skip flag land in
// lockstep at every step. The permission flag follows Yolo and only Yolo.
func TestCycleAgentModeFullLoop(t *testing.T) {
	pinTTLs(t)

	ws := newModeTracking()
	m := newBusyUI(ws)
	warmCaches(m, false)
	ws.yolo = false
	m.setEditorPrompt(false)

	steps := []struct {
		from, want agent.AgentMode
		wantYolo   bool
	}{
		{agent.AgentModeBuild, agent.AgentModeYolo, true},
		{agent.AgentModeYolo, agent.AgentModePlan, false},
		{agent.AgentModePlan, agent.AgentModeBuild, false},
	}
	for _, step := range steps {
		ws.modeBySession["s1"] = step.from
		ws.yolo = step.from == agent.AgentModeYolo
		genBefore := m.busyFetchGen

		_ = m.cycleAgentMode()

		require.Equal(t, step.want, ws.modeBySession["s1"],
			"cycle from %s must land on %s", step.from, step.want)
		require.Equal(t, step.wantYolo, ws.yolo,
			"cycle from %s must sync yolo flag to %v", step.from, step.wantYolo)
		require.Greater(t, m.busyFetchGen, genBefore,
			"every cycle must bump busyFetchGen to supersede in-flight probes")
	}
}

// TestSetAgentModeSyncsYoloFlag pins the bug where cycling into Yolo
// failed to flip PermissionSkipRequests on (the Yolo icon never appeared).
// setAgentMode must toggle the flag and bump the generation.
func TestSetAgentModeSyncsYoloFlag(t *testing.T) {
	pinTTLs(t)

	ws := newModeTracking()
	m := newBusyUI(ws)
	warmCaches(m, false)
	ws.yolo = false
	m.setEditorPrompt(false)
	genBefore := m.busyFetchGen

	m.setAgentMode(agent.AgentModeYolo)
	require.True(t, ws.yolo, "setAgentMode(Yolo) must flip yolo flag on")
	require.Greater(t, m.busyFetchGen, genBefore,
		"must bump busyFetchGen")
	require.Equal(t, 1, ws.permSetCalls,
		"setAgentMode must call PermissionSetSkipRequests exactly once")

	m.setAgentMode(agent.AgentModeBuild)
	require.False(t, ws.yolo, "setAgentMode(Build) must flip yolo flag off")
	require.Equal(t, 2, ws.permSetCalls,
		"every setAgentMode must call PermissionSetSkipRequests")
}

// TestSetAgentModeDefaultsInvalidToBuild guards the entry-point contract:
// an invalid AgentMode value is coerced to Build, never written verbatim.
func TestSetAgentModeDefaultsInvalidToBuild(t *testing.T) {
	pinTTLs(t)

	ws := newModeTracking()
	m := newBusyUI(ws)
	warmCaches(m, false)

	m.setAgentMode(agent.AgentMode("bogus"))
	require.Equal(t, agent.AgentModeBuild, ws.modeBySession["s1"],
		"invalid mode must be coerced to Build")
	require.False(t, ws.yolo, "invalid mode must not turn yolo on")
}

// TestApplyAgentModeNoopWhenYoloAlreadyCorrect checks that when the
// requested mode matches the current permission-skip state, the call
// does not re-issue PermissionSetSkipRequests. Repeated cycles must not
// hammer the coordinator's HTTP endpoint.
func TestApplyAgentModeNoopWhenYoloAlreadyCorrect(t *testing.T) {
	pinTTLs(t)

	ws := newModeTracking()
	m := newBusyUI(ws)
	warmCaches(m, false)
	ws.yolo = true // workspace already in yolo
	genBefore := m.busyFetchGen
	ws.permSetCalls = 0

	m.applyAgentMode(agent.AgentModeYolo)
	require.Equal(t, 0, ws.permSetCalls,
		"applyAgentMode must skip PermissionSetSkipRequests when flag already matches")
	require.Greater(t, m.busyFetchGen, genBefore,
		"generation bump must still happen to supersede probes")
}

// TestApplyAgentModeRefreshesYoloCache verifies every mode change writes the
// yolo cache through immediately, so prompt rendering uses current mode.
func TestApplyAgentModeRefreshesYoloCache(t *testing.T) {
	pinTTLs(t)

	ws := newModeTracking()
	m := newBusyUI(ws)
	warmCaches(m, false)

	m.applyAgentMode(agent.AgentModeYolo)
	require.True(t, m.yoloModeCached())

	m.applyAgentMode(agent.AgentModeBuild)
	require.False(t, m.yoloModeCached())
}

// TestCycleAgentModeReturnsInfoMessage pins the user-facing feedback: the
// cycle command produces an info toast naming the new mode.
func TestCycleAgentModeReturnsInfoMessage(t *testing.T) {
	pinTTLs(t)

	ws := newModeTracking()
	m := newBusyUI(ws)
	warmCaches(m, false)
	ws.modeBySession["s1"] = agent.AgentModeBuild
	ws.yolo = false

	cmd := m.cycleAgentMode()
	require.NotNil(t, cmd, "cycleAgentMode must return non-nil command (info toast)")
	msg := cmd()
	info, ok := msg.(util.InfoMsg)
	require.True(t, ok, "message must be util.InfoMsg, got %T", msg)
	require.Contains(t, info.Msg, "yolo",
		"info toast must name the destination mode")

	cmd = m.setAgentMode(agent.AgentModePlan)
	require.NotNil(t, cmd)
	msg = cmd()
	info, ok = msg.(util.InfoMsg)
	require.True(t, ok)
	require.Contains(t, info.Msg, "plan",
		"info toast must name the destination mode")
}

// TestCycleModeKeyBinding verifies Shift+Tab reaches cycleAgentMode through
// UI key handling.
func TestCycleModeKeyBinding(t *testing.T) {
	pinTTLs(t)

	ws := newModeTracking()
	m := newBusyUI(ws)
	warmCaches(m, false)

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	runCmds(m, cmd)
	require.Equal(t, agent.AgentModeYolo, ws.modeBySession["s1"])
}

// TestCommandPaletteSetAgentMode pins mode values used by command-palette
// actions, which call setAgentMode after dialog dispatch.
func TestCommandPaletteSetAgentMode(t *testing.T) {
	pinTTLs(t)

	ws := newModeTracking()
	m := newBusyUI(ws)
	warmCaches(m, false)

	for _, mode := range []agent.AgentMode{
		agent.AgentModePlan,
		agent.AgentModeBuild,
		agent.AgentModeYolo,
	} {
		ws.modeBySession["s1"] = agent.AgentModeBuild
		ws.yolo = false
		ws.permSetCalls = 0

		m.setAgentMode(mode)

		require.Equal(t, mode, ws.modeBySession["s1"])
		require.Equal(t, mode == agent.AgentModeYolo, ws.yolo)
		if mode == agent.AgentModeYolo {
			require.Equal(t, 1, ws.permSetCalls)
		} else {
			require.Zero(t, ws.permSetCalls)
		}
	}
}

// TestApplyAgentModeBumpsBusyFetchGen ensures applyAgentMode always bumps
// busyFetchGen so any in-flight busy/yolo probe carrying the old
// generation is superseded and cannot clobber the fresh state.
func TestApplyAgentModeBumpsBusyFetchGen(t *testing.T) {
	pinTTLs(t)

	ws := newModeTracking()
	m := newBusyUI(ws)
	warmCaches(m, false)
	ws.yolo = false

	// Simulate an in-flight probe at generation N
	m.busyFetchInFlight = true
	oldGen := m.busyFetchGen

	m.applyAgentMode(agent.AgentModeYolo)

	require.NotEqual(t, oldGen, m.busyFetchGen,
		"applyAgentMode must advance busyFetchGen")
	require.True(t, m.busyFetchInFlight,
		"in-flight flag must remain true (probe superseded, not cancelled)")

	// The stale probe (oldGen) must not overwrite the new value
	m.applyBusyState(busyStateMsg{gen: oldGen, yolo: false})
	require.True(t, m.yoloModeCached(),
		"stale probe must not clobber fresh yolo state")
	require.NotEmpty(t, m.applyBusyState(busyStateMsg{gen: oldGen, yolo: false}),
		"stale probe must re-dispatch authoritative refresh")
}

// TestWorkspaceCacheInitialState verifies a fresh UI starts with a valid
// session mode default and correct yolo flag.
func TestWorkspaceCacheInitialState(t *testing.T) {
	pinTTLs(t)

	ws := newModeTracking()
	m := newBusyUI(ws)
	warmCaches(m, false)

	// Before any cycle, the workspace reports Build and yolo=false
	require.Equal(t, agent.AgentModeBuild, ws.modeBySession["s1"])
	require.False(t, ws.yolo)
	require.False(t, m.yoloModeCached())
	require.Zero(t, m.busyFetchGen)
}
