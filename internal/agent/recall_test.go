package agent

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/memory"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

func TestBuildAutoRecallBlockIncludesMemoryAndHistory(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	sess, err := env.sessions.Create(t.Context(), "recall")
	require.NoError(t, err)

	require.NoError(t, env.memory.Store(t.Context(), memory.StoreParams{Key: "project/goal", Value: "Ship MVP search flow", Scope: "project", Category: "product", Type: "goal", Tags: []string{"search", "launch"}}))
	_, err = env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "Remember the search implementation details"}},
	})
	require.NoError(t, err)

	block := buildAutoRecallBlock(context.Background(), env.history, env.memory, nil, sess.ID, "search")
	require.Contains(t, block, "Relevant long-term memory:")
	require.Contains(t, block, "project/goal")
	require.Contains(t, block, "product/goal")
	require.Contains(t, block, "#launch #search")
	require.Contains(t, block, "Relevant session history:")
	require.Contains(t, block, "search implementation details")
}

func TestBuildAutoRecallBlockSkipsEmptyResults(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	sess, err := env.sessions.Create(t.Context(), "recall-empty")
	require.NoError(t, err)

	block := buildAutoRecallBlock(context.Background(), env.history, env.memory, nil, sess.ID, "unmatched query")
	require.Empty(t, block)
}

func TestBuildAutoRecallBlockFiltersMemoryByAgentPolicy(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	sess, err := env.sessions.Create(t.Context(), "recall-policy")
	require.NoError(t, err)

	require.NoError(t, env.memory.Store(t.Context(), memory.StoreParams{Key: "scope/project", Value: "Project memory", Scope: "project"}))
	require.NoError(t, env.memory.Store(t.Context(), memory.StoreParams{Key: "scope/session", Value: "Session memory", Scope: "session"}))

	ctx := context.WithValue(context.Background(), tools.AgentMemoryContextKey, "isolated")
	block := buildAutoRecallBlock(ctx, env.history, env.memory, nil, sess.ID, "memory")
	require.Contains(t, block, "scope/session")
	require.NotContains(t, block, "scope/project")

	ephemeralCtx := context.WithValue(context.Background(), tools.AgentMemoryContextKey, "ephemeral")
	ephemeralBlock := buildAutoRecallBlock(ephemeralCtx, env.history, env.memory, nil, sess.ID, "memory")
	require.NotContains(t, ephemeralBlock, "Relevant long-term memory:")
}

func TestAutoRecallMemoryScopeRespectsIsolationHints(t *testing.T) {
	t.Parallel()

	scope, include := autoRecallMemoryScope(context.WithValue(context.Background(), tools.AgentIsolationContextKey, "workspace"))
	require.True(t, include)
	require.Equal(t, "project", scope)

	scope, include = autoRecallMemoryScope(context.WithValue(context.Background(), tools.AgentIsolationContextKey, "process"))
	require.True(t, include)
	require.Equal(t, "session", scope)
}

func TestFormatAutoRecallHistoryUsesSessionScopedResults(t *testing.T) {
	t.Parallel()

	results := []history.MessageSearchResult{{Role: message.User, Text: "alpha"}, {Role: message.Assistant, Text: "beta"}}
	formatted := formatAutoRecallHistory(results)
	require.Contains(t, formatted, "[user] alpha")
	require.Contains(t, formatted, "[assistant] beta")
}
