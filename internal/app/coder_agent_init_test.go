package app

import (
	"context"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

// initFakeCoordinator is a minimal agent.Coordinator stand-in for
// exercising initCoderAgent's create-only semantics without the full
// coordinator machinery.
type initFakeCoordinator struct {
	busy bool
}

func (c *initFakeCoordinator) SetMainAgent(string) error { return nil }
func (c *initFakeCoordinator) Run(context.Context, string, string, ...message.Attachment) (*fantasy.AgentResult, error) {
	return nil, nil
}

func (c *initFakeCoordinator) RunAccepted(context.Context, *agent.AcceptedRun, string, string, ...message.Attachment) (*fantasy.AgentResult, error) {
	return nil, nil
}
func (c *initFakeCoordinator) BeginAccepted(string) *agent.AcceptedRun { return nil }
func (c *initFakeCoordinator) Cancel(string)                           {}
func (c *initFakeCoordinator) CancelAll()                              {}
func (c *initFakeCoordinator) IsSessionBusy(string) bool               { return false }
func (c *initFakeCoordinator) IsBusy() bool                            { return c.busy }
func (c *initFakeCoordinator) QueuedPrompts(string) int                { return 0 }
func (c *initFakeCoordinator) QueuedPromptsList(string) []string       { return nil }
func (c *initFakeCoordinator) ClearQueue(string)                       {}
func (c *initFakeCoordinator) Summarize(context.Context, string) error { return nil }
func (c *initFakeCoordinator) Model() agent.Model                      { return agent.Model{} }
func (c *initFakeCoordinator) UpdateModels(context.Context) error      { return nil }
func (c *initFakeCoordinator) GenerateTitle(context.Context, string, string) {
}

// TestInitCoderAgentCreateOnly: once a coordinator exists, a second
// initialization (headless or interactive) is a no-op, so a channel push
// initializing a headless workspace cannot race a client attach into
// building a second coordinator.
func TestInitCoderAgentCreateOnly(t *testing.T) {
	t.Parallel()

	existing := &initFakeCoordinator{}
	a := &App{AgentCoordinator: existing, agentInteractive: true}

	require.NoError(t, a.initCoderAgent(context.Background(), false))
	require.NoError(t, a.initCoderAgent(context.Background(), true))
	require.Same(t, existing, a.AgentCoordinator)
}

// TestInitCoderAgentInteractiveUpgradeKeepsBusyCoordinator: an
// interactive initializer wants to upgrade a headless coordinator, but
// not while a run is in flight on it — swapping then would strand the
// run, so the existing coordinator is kept.
func TestInitCoderAgentInteractiveUpgradeKeepsBusyCoordinator(t *testing.T) {
	t.Parallel()

	existing := &initFakeCoordinator{busy: true}
	a := &App{AgentCoordinator: existing, agentInteractive: false}

	require.NoError(t, a.initCoderAgent(context.Background(), true))
	require.Same(t, existing, a.AgentCoordinator)
	require.False(t, a.agentInteractive)
}
