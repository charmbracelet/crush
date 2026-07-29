package agent

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/stretchr/testify/require"
)

// TestNewRunContexts_GenCtxCarriesRootSession is the regression test for the
// Ctrl+B sub-agent path.
//
// The root session ID used to be stamped further down in Run, onto a ctx
// variable that genCtx did not descend from. genCtx is what agent.Stream —
// and therefore every tool call — runs under, so the stamp never reached a
// tool: tools.GetRootSessionFromContext silently fell back to
// GetSessionFromContext. The whole sub-agent half of the feature was inert
// while its tests (which built the context by hand) stayed green.
//
// Asserting on genCtx rather than runCtx is the point: it is the context
// tools actually see, so stamping after the derivation fails here.
func TestNewRunContexts_GenCtxCarriesRootSession(t *testing.T) {
	t.Parallel()

	_, genCtx, cancel := newRunContexts(context.Background(), "top-level")
	defer cancel()

	require.Equal(t, "top-level", tools.GetSessionFromContext(genCtx))
	require.Equal(t, "top-level", tools.GetRootSessionFromContext(genCtx),
		"a top-level turn must stamp itself as the root on the context tools run under")
}

// TestNewRunContexts_SubAgentInheritsRootSession pins the case the feature
// exists for: a bash command inside an agent-tool sub-run must be
// registered under the session the user is looking at, not the synthetic
// child session, or Ctrl+B on the parent can never reach it.
func TestNewRunContexts_SubAgentInheritsRootSession(t *testing.T) {
	t.Parallel()

	// The parent turn. Its genCtx is what the agent tool executes under,
	// and therefore what the child run receives.
	_, parentGenCtx, cancelParent := newRunContexts(context.Background(), "parent-session")
	defer cancelParent()

	_, childGenCtx, cancelChild := newRunContexts(parentGenCtx, "child-session")
	defer cancelChild()

	require.Equal(t, "child-session", tools.GetSessionFromContext(childGenCtx),
		"the child run still owns its own session")
	require.Equal(t, "parent-session", tools.GetRootSessionFromContext(childGenCtx),
		"the sub-agent must inherit the parent turn's root session; falling back to the child session is what made Ctrl+B miss sub-agent bash waits")
}

// TestNewRunContexts_CancelPropagates pins that genCtx stays the
// cancellable one: Escape cancels a turn through this handle.
func TestNewRunContexts_CancelPropagates(t *testing.T) {
	t.Parallel()

	runCtx, genCtx, cancel := newRunContexts(context.Background(), "s1")
	cancel()

	require.Error(t, genCtx.Err(), "cancel must cancel the run context")
	require.NoError(t, runCtx.Err(), "runCtx is the uncancellable value carrier")
}
