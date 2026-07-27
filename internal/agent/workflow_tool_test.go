package agent

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

// TestStripWorkflowTool_RemovesWorkflow asserts safety 3a: the
// workflow tool is stripped from a subagent's allowed tools.
func TestStripWorkflowTool_RemovesWorkflow(t *testing.T) {
	t.Parallel()
	agent := config.Agent{
		AllowedTools: []string{"bash", "edit", "workflow", "view", "grep"},
	}
	stripped := stripWorkflowTool(agent)
	require.NotContains(t, stripped.AllowedTools, WorkflowToolName,
		"subagent must not have the workflow tool")
	// Original must be unmodified.
	require.Contains(t, agent.AllowedTools, WorkflowToolName,
		"original agent config must not be mutated")
}

// TestStripWorkflowTool_CoderProfile verifies that a coder-profile
// config (which includes all tools including workflow) does not
// retain workflow after stripping.
func TestStripWorkflowTool_CoderProfile(t *testing.T) {
	t.Parallel()
	cfg := config.Config{
		Options: &config.Options{},
	}
	cfg.SetupAgents()
	coderCfg, ok := cfg.Agents[config.AgentCoder]
	require.True(t, ok, "coder agent must exist")
	require.Contains(t, coderCfg.AllowedTools, WorkflowToolName,
		"coder config should normally include workflow")

	stripped := stripWorkflowTool(coderCfg)
	require.NotContains(t, stripped.AllowedTools, WorkflowToolName,
		"coder subagent must NOT have workflow after stripping")
}

// TestStripWorkflowTool_TaskProfile verifies that the task profile
// (which never has workflow) is unaffected by stripping.
func TestStripWorkflowTool_TaskProfile(t *testing.T) {
	t.Parallel()
	cfg := config.Config{
		Options: &config.Options{},
	}
	cfg.SetupAgents()
	taskCfg := cfg.Agents[config.AgentTask]
	require.NotContains(t, taskCfg.AllowedTools, WorkflowToolName,
		"task profile should not have workflow to begin with")

	stripped := stripWorkflowTool(taskCfg)
	require.NotContains(t, stripped.AllowedTools, WorkflowToolName)
	require.Equal(t, taskCfg.AllowedTools, stripped.AllowedTools,
		"task profile should be unchanged")
}

// recordingPermissions captures the permission request the workflow tool
// sends and then denies it, so Run returns before any sub-agent is spawned.
type recordingPermissions struct {
	mu   sync.Mutex
	seen bool
	last permission.CreatePermissionRequest
}

func (r *recordingPermissions) Request(_ context.Context, opts permission.CreatePermissionRequest) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seen = true
	r.last = opts
	return false, nil
}

// description returns the text the user would have been shown.
func (r *recordingPermissions) description(t *testing.T) string {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	require.True(t, r.seen, "workflow tool never requested permission")
	return r.last.Description
}

func (r *recordingPermissions) Subscribe(context.Context) <-chan pubsub.Event[permission.PermissionRequest] {
	return nil
}

func (r *recordingPermissions) SubscribeNotifications(context.Context) <-chan pubsub.Event[permission.PermissionNotification] {
	return nil
}
func (r *recordingPermissions) GrantPersistent(permission.PermissionRequest) bool { return true }
func (r *recordingPermissions) Grant(permission.PermissionRequest) bool           { return true }
func (r *recordingPermissions) Deny(permission.PermissionRequest) bool            { return true }
func (r *recordingPermissions) AutoApproveSession(string)                         {}
func (r *recordingPermissions) SetSkipRequests(bool)                              {}
func (r *recordingPermissions) SkipRequests() bool                                { return false }
func (r *recordingPermissions) SetPlanMode(bool)                                  {}
func (r *recordingPermissions) PlanMode() bool                                    { return false }

// workflowConsentFor runs the real workflow tool over a script and returns the
// description string the permission dialog would render. It goes through
// c.workflowTool -> tool.Run -> permissions.Request, so it exercises the
// production call site rather than a copy of the text-building logic.
func workflowConsentFor(t *testing.T, script string) string {
	t.Helper()

	c, _, _ := newModelPinningCoordinator(t)
	rec := &recordingPermissions{}
	c.permissions = rec

	tool, err := c.workflowTool(t.Context())
	require.NoError(t, err)

	input, err := json.Marshal(WorkflowParams{
		Description: workflowTestDescription,
		Script:      script,
	})
	require.NoError(t, err)

	ctx := context.WithValue(t.Context(), tools.SessionIDContextKey, "session-1")
	ctx = context.WithValue(ctx, tools.MessageIDContextKey, "message-1")

	_, err = tool.Run(ctx, fantasy.ToolCall{
		ID:    "call-1",
		Name:  WorkflowToolName,
		Input: string(input),
	})
	require.NoError(t, err)

	return rec.description(t)
}

// workflowTestDescription deliberately contains no digits, so a test can assert
// mechanically that no count leaks back into the consent text.
const workflowTestDescription = "Refactor the parser"

// TestWorkflowConsentTextStatesCapability is the regression test for the
// consent prompt being derived from a substring scan of a Turing-complete Lua
// script. countCoderAgents looked for `agent = "coder"` and reported the number
// of matches to the user; all five scripts below spawn a write-capable
// sub-agent while that scan finds nothing, so the prompt claimed no coder agent
// was requested and the user approved blanket file-write and shell access
// anyway. Three of the five are ordinary Lua, not evasion.
//
// The fix states the capability the approval grants instead of describing the
// script, so the text is necessarily invariant across every script. This test
// asserts that invariance: all cases, bypasses and control alike, must produce
// byte-identical consent text that names the capability.
func TestWorkflowConsentTextStatesCapability(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		script string
	}{
		// Bypass 1: the canonical form the old scan did match.
		{"canonical", `agent("do", {agent = "coder"})`},
		// Bypass 2: two spaces around `=`. Ordinary Lua; scan reported 0.
		{"double space", `agent("do", {agent  =  "coder"})`},
		// Bypass 3: bracketed key. Ordinary Lua; scan reported 0.
		{"bracket key", `agent("do", {["agent"]="coder"})`},
		// Bypass 4: newline before the value. Ordinary Lua; scan reported 0.
		{"newline before value", "agent(\"do\", {agent =\n\t\"coder\"})"},
		// Bypass 5: the profile name is computed at run time, so no
		// static scan of the source can ever see it.
		{"computed at runtime", `local p = "cod" .. "er"
agent("do", {agent = p})`},
		// Control: a script that really does spawn nothing writable.
		// The capability statement is unconditional, so this must read
		// exactly the same -- the user is told what approval grants,
		// not what this particular script happens to do.
		{"no coder at all", `agent("summarise the README")`},
	}

	// Deliberately not subtests: the invariance assertion below compares the
	// cases against each other, so they have to be collected in one goroutine
	// before anything is compared.
	texts := make([]string, len(cases))
	for i, tc := range cases {
		desc := workflowConsentFor(t, tc.script)

		require.Contains(t, desc, workflowConsentNotice,
			"case %q: consent text must state the capability granted", tc.name)
		require.Contains(t, desc, "file-write",
			"case %q: consent text must name file-write access", tc.name)
		require.Contains(t, desc, "shell",
			"case %q: consent text must name shell access", tc.name)
		require.Contains(t, desc, workflowTestDescription,
			"case %q: consent text must keep the workflow's own description", tc.name)
		require.NotRegexp(t, `\d`, desc,
			"case %q: consent text must not report a count derived from the script", tc.name)

		texts[i] = desc
	}

	for i := range texts {
		require.Equal(t, texts[0], texts[i],
			"consent text must be identical for every script; case %q differs", cases[i].name)
	}
}

// TestWorkflowPermissionDescriptionAppendsNotice covers the text builder
// directly, including the empty-description edge the tool itself rejects.
func TestWorkflowPermissionDescriptionAppendsNotice(t *testing.T) {
	t.Parallel()

	require.Equal(t, "Do a thing\n\n"+workflowConsentNotice,
		workflowPermissionDescription("Do a thing"))
	require.Equal(t, workflowConsentNotice,
		workflowPermissionDescription(""))
}

// TestWorkflowConsentNoticeMatchesRealGrantScope is the docs-vs-behaviour test
// for the one qualifier in workflowConsentNotice that could understate the
// grant: its scope.
//
// The notice tells the user that an approved workflow's sub-agents get
// file-write and shell access "on any path this process can reach". That is a
// claim about permission.Service, so it is asserted against the real service
// rather than restated: an auto-approved session is granted a write far outside
// the working directory, with no prompt. If a future change ever bounds the
// grant to the working directory, this test fails and the sentence must be
// narrowed with it -- and, more importantly, if someone re-narrows the sentence
// to "in this directory" while the grant stays unbounded, the scope assertion
// below fails.
func TestWorkflowConsentNoticeMatchesRealGrantScope(t *testing.T) {
	t.Parallel()

	// The notice must not imply a directory bound that is not enforced.
	for _, claim := range []string{"in this directory", "within this directory", "in the working directory"} {
		require.NotContains(t, workflowConsentNotice, claim,
			"consent notice must not claim a directory scope the grant does not have")
	}
	require.Contains(t, workflowConsentNotice, "on any path this process can reach",
		"consent notice must state that the grant is not path-scoped")

	// ...because the grant really is not path-scoped.
	const workingDir = "/home/user/project"
	svc := permission.NewPermissionService(workingDir, false, nil)
	svc.AutoApproveSession("workflow-subagent-session")

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	granted, err := svc.Request(ctx, permission.CreatePermissionRequest{
		SessionID:  "workflow-subagent-session",
		ToolCallID: "call-scope-1",
		ToolName:   "write",
		Action:     "write",
		Path:       "/etc/cron.d/anywhere-but-the-working-dir",
	})
	require.NoError(t, err)
	require.True(t, granted,
		"an auto-approved workflow sub-agent is granted writes outside %s without prompting, "+
			"so the consent notice must not promise a directory bound", workingDir)
}
