package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// TestBuildAgentReadinessSurvivesCallerCancellation is a regression test for
// the CRUSH_CLIENT_SERVER=1 "new session hangs" bug.
//
// buildAgent starts readiness goroutines that build the system prompt and the
// initial tool list. Several server entry points build an agent from a
// short-lived HTTP request context — the InitAgent/UpdateAgent handlers, and
// the sub-agent build reached through UpdateModels -> buildTools -> agentTool.
// When that request context was canceled the moment the handler returned, the
// readiness group recorded context.Canceled and every later coordinator.run
// failed at the readiness wait before emitting anything — the session hung
// with no visible LLM response. (This was made worse while the tool-list
// goroutine also blocked in mcp.WaitForInit, which kept it parked long enough
// to observe the cancellation; the readiness work no longer waits on MCP init
// — see coordinator.run — but the cancellation detachment still matters.)
//
// The fix detaches the readiness work from the caller context via
// context.WithoutCancel, so canceling the context that triggered the build no
// longer poisons the agent's readiness. Here we build an agent with a
// cancelable context, cancel it, and require that readiness still completes
// cleanly.
func TestBuildAgentReadinessSurvivesCallerCancellation(t *testing.T) {
	env := testEnv(t)

	// Minimal hermetic config: one openai-typed provider with selected large
	// and small models so buildAgentModels and the system-prompt build both
	// succeed. No MCP servers are configured, so initialization would complete
	// instantly if we let it — we arm the gate anyway to prove the readiness
	// goroutines no longer block on it.
	crushJSON := `{
  "options": {"disable_default_providers": true, "disable_provider_auto_update": true},
  "providers": {"mock": {"id": "mock", "name": "Mock", "type": "openai",
    "base_url": "http://127.0.0.1:9/v1", "api_key": "test-key",
    "models": [{"id": "mock-model", "name": "Mock", "context_window": 8192, "default_max_tokens": 128}]}},
  "models": {"large": {"provider": "mock", "model": "mock-model"},
             "small": {"provider": "mock", "model": "mock-model"}}
}`
	require.NoError(t, os.WriteFile(filepath.Join(env.workingDir, "crush.json"), []byte(crushJSON), 0o644))

	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	cfg.SetupAgents()

	coord := &coordinator{
		cfg:         cfg,
		sessions:    env.sessions,
		messages:    env.messages,
		permissions: env.permissions,
		history:     env.history,
		filetracker: *env.filetracker,
	}

	// Arm the MCP init gate. We never complete init; the readiness goroutines
	// must not care, since they build the tool list from the registry as it
	// stands rather than waiting for initialization to finish.
	mcp.ArmInit()
	t.Cleanup(mcp.DisarmInit)

	p, err := coderPrompt(prompt.WithWorkingDir(env.workingDir))
	require.NoError(t, err)
	agentCfg := cfg.Config().Agents[config.AgentCoder]

	ctx, cancel := context.WithCancel(context.Background())
	_, ready, err := coord.buildAgent(ctx, p, agentCfg, false)
	require.NoError(t, err)

	// The caller goes away, mirroring an HTTP handler returning and canceling
	// its request context.
	cancel()

	waitCtx, waitCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer waitCancel()
	err = ready.wait(waitCtx)

	// context.Canceled is the regression: the caller's cancellation leaked
	// into the readiness work and poisoned it. context.DeadlineExceeded means
	// the readiness goroutines never finished, which is the MCP-init variant
	// of the same hang.
	require.NotErrorIs(t, err, context.Canceled,
		"readiness was poisoned by caller cancellation (client/server new-session hang regression)")
	require.NotErrorIs(t, err, context.DeadlineExceeded,
		"readiness did not complete; the readiness goroutines must not block on MCP init")
	require.NoError(t, err, "unexpected buildAgent readiness error")
}

// TestConcurrentRunsDoNotShareOneReadinessGroup is a regression test for the
// crash that killed the whole shared server during the runaway-session
// incident:
//
//	panic: sync: WaitGroup is reused before previous Wait has returned
//	sync.(*WaitGroup).Wait <- errgroup.(*Group).Wait <- agent.(*coordinator).run
//
// Readiness used to live in one long-lived errgroup on the coordinator. Every
// run waited on it, and every run also re-entered the agent builder through
// UpdateModels -> buildTools -> agentTool -> buildAgent, which added two
// goroutines to that same group. With two runs in one workspace, one sat in
// Wait() while the other called Go(), which is Add on a WaitGroup whose
// counter had reached zero with a waiter parked. Go panics there. The panic
// happened on a bare goroutine (backend.SendMessage) with no recover on the
// path, so the daemon died — every workspace, every session, every project.
//
// Readiness is per agent now, and every setup goroutine is started before the
// handle is published, so no group can be added to while a waiter is parked.
// The runs below fail for unrelated reasons (unknown session, closed provider
// port); what matters is that they neither panic nor fail because of another
// run's readiness.
func TestConcurrentRunsDoNotShareOneReadinessGroup(t *testing.T) {
	// The runs are interactive by default, so they do not park on the MCP
	// init gate.
	coord := newGateTestCoordinator(t)

	const (
		rounds  = 4
		perRoun = 8
	)
	for round := range rounds {
		errs := make([]error, perRoun)
		var wg sync.WaitGroup
		for i := range perRoun {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, errs[i] = coord.run(
					context.Background(),
					nil,
					fmt.Sprintf("session-%d-%d", round, i),
					"hello",
				)
			}()
		}
		wg.Wait()

		for i, err := range errs {
			if err == nil {
				continue
			}
			// A readiness failure here would mean one run's build state
			// leaked into another's: nothing in this test breaks the
			// builder, so no run may fail on setup.
			require.NotErrorIs(t, err, context.Canceled,
				"run %d/%d failed on another run's cancellation", round, i)
			require.NotContains(t, err.Error(), "task agent not configured",
				"run %d/%d inherited another build's failure", round, i)
		}
	}
}

// TestFailedAgentBuildDoesNotPoisonLaterRuns pins the second fault of the
// shared readiness group: an errgroup keeps its first error forever, so one
// transient build failure made every later run in the process fail at the
// readiness wait before emitting anything — a workspace that could no longer
// answer a single prompt until the daemon was restarted.
//
// Readiness belongs to the agent now, so a run reports its own agent's setup
// failure, and a rebuilt agent starts clean.
func TestFailedAgentBuildDoesNotPoisonLaterRuns(t *testing.T) {
	coord := newGateTestCoordinator(t)

	p, err := coderPrompt(prompt.WithWorkingDir(coord.cfg.WorkingDir()))
	require.NoError(t, err)
	agentCfg := coord.cfg.Config().Agents[config.AgentCoder]

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// An agent whose setup failed. Standing in for a transient prompt or
	// tool-list build failure, which is what poisoned the shared group.
	broken, _, err := coord.buildAgent(ctx, p, agentCfg, false)
	require.NoError(t, err)
	buildErr := errors.New("build failed")
	coord.setActiveAgent(config.AgentCoder, broken, newAgentReadiness(func() error {
		return buildErr
	}))

	_, err = coord.run(ctx, nil, "session-on-broken-agent", "hello")
	require.ErrorIs(t, err, buildErr,
		"a run must report the readiness failure of the agent it was about to use")

	// The rebuild is a different agent, so the earlier failure is gone with
	// the agent it belonged to.
	rebuilt, ready, err := coord.buildAgent(ctx, p, agentCfg, false)
	require.NoError(t, err)
	require.NoError(t, ready.wait(ctx), "the rebuilt agent's own setup must be clean")
	coord.setActiveAgent(config.AgentCoder, rebuilt, ready)

	// This run fails for unrelated reasons (unknown session, closed provider
	// port); it must not fail on the previous agent's build error.
	_, err = coord.run(ctx, nil, "session-after-failed-build", "hello")
	require.NotErrorIs(t, err, buildErr,
		"a rebuilt agent inherited the previous build's readiness error")
}
