package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"charm.land/fantasy"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

func sessionModelID(t *testing.T, c *coordinator, sessionID string) string {
	t.Helper()
	m, err := c.modelForSession(sessionID)
	require.NoError(t, err)
	return m.ModelCfg.Model
}

func setDefaultLarge(t *testing.T, c *coordinator, sel config.SelectedModel) {
	t.Helper()
	c.cfg.Config().Models[config.SelectedModelTypeLarge] = sel
	require.NoError(t, c.rebuildModels(t.Context()))
}

// A session keeps the model it first ran on after the default changes, and
// a new session gets the new default.
func TestModelForSessionKeepsPinnedModelAcrossDefaultSwitch(t *testing.T) {
	t.Parallel()
	c, _ := newOverrideTestCoordinator(t)

	require.Equal(t, "mock-reasoner", sessionModelID(t, c, "session-a"))
	setDefaultLarge(t, c, config.SelectedModel{Provider: "other", Model: "other-model"})

	require.Equal(t, "mock-reasoner", sessionModelID(t, c, "session-a"))
	require.Equal(t, "other-model", sessionModelID(t, c, "session-b"))
}

// A model picked in one session reaches only that session.
func TestSetSessionModelAppliesOnlyToThatSession(t *testing.T) {
	t.Parallel()
	c, _ := newOverrideTestCoordinator(t)

	require.NoError(t, c.SetSessionModel(t.Context(), "session-a", config.SelectedModel{Provider: "mock", Model: "mock-thinker"}))

	require.Equal(t, "mock-thinker", sessionModelID(t, c, "session-a"))
	require.Equal(t, "mock-reasoner", sessionModelID(t, c, "session-b"))
}

// A model that cannot serve is refused rather than pinned.
func TestSetSessionModelRejectsUnbuildableSelection(t *testing.T) {
	t.Parallel()
	c, _ := newOverrideTestCoordinator(t)

	err := c.SetSessionModel(t.Context(), "session-a", config.SelectedModel{Provider: "mock", Model: "no-such-model"})
	require.ErrorContains(t, err, "no-such-model")
}

// Settings changed on the default, a reasoning effort say, stay out of a
// session already pinned to the same model; the session changes only when
// they are applied to it.
func TestSessionSettingsDoNotLeakAcrossSessions(t *testing.T) {
	t.Parallel()
	c, _ := newOverrideTestCoordinator(t)
	effort := func(id string) string {
		m, err := c.modelForSession(id)
		require.NoError(t, err)
		return m.ModelCfg.ReasoningEffort
	}

	require.Equal(t, "low", effort("session-a"))
	high := config.SelectedModel{Provider: "mock", Model: "mock-reasoner", ReasoningEffort: "high"}
	setDefaultLarge(t, c, high)
	require.Equal(t, "low", effort("session-a"))

	require.NoError(t, c.SetSessionModel(t.Context(), "session-a", high))
	require.Equal(t, "high", effort("session-a"))
}

// Asking which model a session runs on does not pin it: until its first
// turn it follows the default.
func TestSessionModelDoesNotPin(t *testing.T) {
	t.Parallel()
	c, _ := newOverrideTestCoordinator(t)

	require.Equal(t, "mock-reasoner", c.SessionModel("session-a").ModelCfg.Model)
	setDefaultLarge(t, c, config.SelectedModel{Provider: "other", Model: "other-model"})
	require.Equal(t, "other-model", c.SessionModel("session-a").ModelCfg.Model)

	require.NoError(t, c.SetSessionModel(t.Context(), "session-b", config.SelectedModel{Provider: "mock", Model: "mock-thinker"}))
	require.Equal(t, "mock-thinker", c.SessionModel("session-b").ModelCfg.Model)
}

// A pinned session's build refreshes with the shared models, and a retry
// mid-turn picks up the refreshed build.
func TestPinnedSessionModelServesLatestBuild(t *testing.T) {
	t.Parallel()
	c, _ := newOverrideTestCoordinator(t)
	require.NoError(t, c.SetSessionModel(t.Context(), "s", config.SelectedModel{Provider: "other", Model: "other-model"}))

	started, err := c.modelForSession("s")
	require.NoError(t, err)
	require.NoError(t, c.rebuildModels(t.Context()))

	a := &sessionAgent{models: c.mainModels}
	require.NotEqual(t, fmt.Sprintf("%p", started.Model), fmt.Sprintf("%p", a.currentBuildOf(started)))
}

// A dispatch without a model override runs on the parent session's model.
func TestDispatchDefaultsToParentSessionModel(t *testing.T) {
	t.Parallel()
	c, env := newOverrideTestCoordinator(t)
	ctx := t.Context()

	tool, err := c.agentDispatchTool(ctx, config.AgentCoder)
	require.NoError(t, err)

	parent, err := env.sessions.Create(ctx, "parent")
	require.NoError(t, err)
	require.NoError(t, c.SetSessionModel(ctx, parent.ID, config.SelectedModel{Provider: "other", Model: "other-model"}))

	assistant, err := env.messages.Create(ctx, parent.ID, message.CreateMessageParams{
		Role:  message.Assistant,
		Parts: []message.ContentPart{message.TextContent{Text: "dispatching"}},
	})
	require.NoError(t, err)
	callCtx := context.WithValue(ctx, tools.SessionIDContextKey, parent.ID)
	callCtx = context.WithValue(callCtx, tools.MessageIDContextKey, assistant.ID)
	resp, err := tool.Run(callCtx, fantasy.ToolCall{ID: "c1", Name: AgentDispatchToolName, Input: `{"prompt":"do","label":"pinned"}`})
	require.NoError(t, err)
	require.False(t, resp.IsError, resp.Content)

	var meta AgentDispatchResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
	require.Empty(t, meta.Model, "an inherited model is not recorded as an override")
	entry, ok := c.dispatched.Get(dispatchKey(parent.ID, meta.Label))
	require.True(t, ok)
	require.Equal(t, "other-model", entry.agent.Model().ModelCfg.Model)
	entry.stop("test done")
}
