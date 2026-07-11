package agent

import (
	"strings"
	"testing"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	agenttools "github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/memory"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

func TestRecordMemoriesUsesHiddenSummarySideCall(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	store, err := memory.Open(t.Context(), memory.Options{Directory: t.TempDir()})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })

	model := &autoReviewStreamModel{text: `{"candidates":[{"type":"feedback","scope":"global","name":"Testing preference","description":"Integration tests use real database instances","content":"Use real database instances for integration tests because mocks previously diverged. Apply this whenever database behavior is under test.","confidence":0.97,"explicit":true,"derivable":false,"replaces_id":"","source_message_id":""}]}`}
	configured := Model{
		Model: model,
		CatwalkCfg: catwalk.Model{
			ContextWindow:    30_000,
			DefaultMaxTokens: 2_000,
		},
		ModelCfg: config.SelectedModel{Provider: "fake", Model: "fake-model"},
	}
	sa := NewSessionAgent(SessionAgentOptions{
		Models: SessionAgentModels{
			Large:   configured,
			Small:   configured,
			Summary: configured,
			Review:  configured,
		},
		Sessions:       env.sessions,
		Messages:       env.messages,
		Memory:         store,
		MemoryProject:  memory.Project{ID: "project-1", Root: env.workingDir},
		MemoryRecorder: true,
	}).(*sessionAgent)

	session, err := env.sessions.Create(t.Context(), "memory test")
	require.NoError(t, err)
	user, err := env.messages.Create(t.Context(), session.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "Remember that integration tests use the real database."}},
	})
	require.NoError(t, err)
	assistant, err := env.messages.Create(t.Context(), session.ID, message.CreateMessageParams{
		Role:  message.Assistant,
		Parts: []message.ContentPart{message.TextContent{Text: "Understood."}},
	})
	require.NoError(t, err)
	assistant.AddFinish(message.FinishReasonEndTurn, "", "")
	require.NoError(t, env.messages.Update(t.Context(), assistant))

	require.NoError(t, sa.recordMemories(t.Context(), session.ID))
	records, err := store.List(t.Context(), "project-1", memory.StatusActive, 10)
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, memory.KindFeedback, records[0].Kind)
	require.Equal(t, session.ID, records[0].SourceSessionID)

	cursor, err := store.RecorderCursor(t.Context(), session.ID)
	require.NoError(t, err)
	require.Equal(t, assistant.ID, cursor.MessageID)

	storedMessages, err := env.messages.List(t.Context(), session.ID)
	require.NoError(t, err)
	require.Len(t, storedMessages, 2)
	require.Equal(t, user.ID, storedMessages[0].ID)
}

func TestRecordMemoriesSkipsExcludedSession(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	store, err := memory.Open(t.Context(), memory.Options{Directory: t.TempDir()})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	model := &autoReviewStreamModel{text: `{"candidates":[]}`}
	configured := Model{
		Model:      model,
		CatwalkCfg: catwalk.Model{ContextWindow: 30_000, DefaultMaxTokens: 2_000},
		ModelCfg:   config.SelectedModel{Provider: "fake", Model: "fake-model"},
	}
	sa := NewSessionAgent(SessionAgentOptions{
		Models:         SessionAgentModels{Large: configured, Small: configured, Summary: configured, Review: configured},
		Sessions:       env.sessions,
		Messages:       env.messages,
		Memory:         store,
		MemoryProject:  memory.Project{ID: "project-1", Root: env.workingDir},
		MemoryRecorder: true,
	}).(*sessionAgent)
	session, err := env.sessions.Create(t.Context(), "excluded recorder")
	require.NoError(t, err)
	require.NoError(t, store.SetSessionRecordingMode(t.Context(), session.ID, memory.SessionRecordingDisabled))

	require.NoError(t, sa.recordMemories(t.Context(), session.ID))
	require.Zero(t, model.streamCalls.Load())
}

func TestExternalContextGuardPollutesOnlyConfiguredSessions(t *testing.T) {
	t.Parallel()

	store, err := memory.Open(t.Context(), memory.Options{Directory: t.TempDir()})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	sa := NewSessionAgent(SessionAgentOptions{
		Memory:                         store,
		MemoryDisableOnExternalContext: true,
	}).(*sessionAgent)

	require.NoError(t, sa.markMemoryExternalContext(t.Context(), "session-1", agenttools.WebSearchToolName))
	mode, err := store.SessionRecordingMode(t.Context(), "session-1")
	require.NoError(t, err)
	require.Equal(t, memory.SessionRecordingPolluted, mode)

	require.NoError(t, sa.markMemoryExternalContext(t.Context(), "session-2", agenttools.ViewToolName))
	mode, err = store.SessionRecordingMode(t.Context(), "session-2")
	require.NoError(t, err)
	require.Equal(t, memory.SessionRecordingEnabled, mode)
}

func TestRecallMemoriesSelectsDetailsWithoutPersistingInjectedPrompt(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	store, err := memory.Open(t.Context(), memory.Options{Directory: t.TempDir(), MaxRecall: 2})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	project := memory.Project{ID: "project-1", Root: env.workingDir}
	record, err := store.SaveObservation(t.Context(), project, memory.Observation{
		Scope:       memory.ScopeGlobal,
		Kind:        memory.KindFeedback,
		Name:        "Testing preference",
		Description: "Integration tests use real database instances",
		Content:     "Use real database instances for integration tests.",
		Confidence:  0.95,
		Explicit:    true,
	})
	require.NoError(t, err)

	selector := &autoReviewStreamModel{text: `{"selected_ids":["` + record.ID + `"]}`}
	configured := Model{
		Model:      selector,
		CatwalkCfg: catwalk.Model{ContextWindow: 30_000, DefaultMaxTokens: 2_000},
		ModelCfg:   config.SelectedModel{Provider: "fake", Model: "fake-model"},
	}
	sa := NewSessionAgent(SessionAgentOptions{
		Models:        SessionAgentModels{Large: configured, Small: configured, Summary: configured, Review: configured},
		Sessions:      env.sessions,
		Messages:      env.messages,
		Memory:        store,
		MemoryProject: project,
		MemoryRecall:  true,
	}).(*sessionAgent)

	prompt := sa.recallMemories(t.Context(), "session-1", "How should I write the database integration test?")
	require.Contains(t, prompt, "<memory-context>")
	require.Contains(t, prompt, "Use real database instances")
	require.Contains(t, prompt, "<current-user-request>")
	require.True(t, strings.HasSuffix(prompt, "</current-user-request>"))
}

func TestMemoryTranscriptHonorsCursorAndBoundsContent(t *testing.T) {
	t.Parallel()

	now := time.Now().UnixMilli()
	messages := []message.Message{
		{ID: "one", Role: message.User, CreatedAt: now, Parts: []message.ContentPart{message.TextContent{Text: "old"}}},
		{ID: "two", Role: message.Assistant, CreatedAt: now + 1, Parts: []message.ContentPart{message.TextContent{Text: strings.Repeat("x", 3000)}}},
		{ID: "three", Role: message.Tool, CreatedAt: now + 2},
	}
	entries, cursor := memoryTranscript(messages, memory.Cursor{CreatedAt: now, MessageID: "one"})
	require.Len(t, entries, 1)
	require.Equal(t, "two", entries[0].ID)
	require.LessOrEqual(t, len([]rune(entries[0].Text)), 2403)
	require.Equal(t, "three", cursor.MessageID)
}

func TestDecodeMemoryJSONStripsReasoningTags(t *testing.T) {
	t.Parallel()

	var selection memorySelection
	err := decodeMemoryJSON("<think>private reasoning</think>\n```json\n{\"selected_ids\":[\"abc\"]}\n```", &selection)
	require.NoError(t, err)
	require.Equal(t, []string{"abc"}, selection.SelectedIDs)
}

func TestScheduledMemoryRecorderRunsOnlyWhenSessionIsIdle(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	store, err := memory.Open(t.Context(), memory.Options{Directory: t.TempDir()})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	candidateJSON := "{\"candidates\":[{\"type\":\"feedback\",\"scope\":\"global\",\"name\":\"Focused verification\",\"description\":\"Run focused checks before broad checks\",\"content\":\"Run focused checks before broad checks when validating a narrow change.\",\"confidence\":0.98,\"explicit\":true,\"derivable\":false,\"replaces_id\":\"\",\"source_message_id\":\"\"}]}"
	model := &autoReviewStreamModel{text: candidateJSON}
	configured := Model{
		Model:      model,
		CatwalkCfg: catwalk.Model{ContextWindow: 30_000, DefaultMaxTokens: 2_000},
		ModelCfg:   config.SelectedModel{Provider: "fake", Model: "fake-model"},
	}
	sa := NewSessionAgent(SessionAgentOptions{
		Models:         SessionAgentModels{Large: configured, Small: configured, Summary: configured, Review: configured},
		Sessions:       env.sessions,
		Messages:       env.messages,
		Memory:         store,
		MemoryProject:  memory.Project{ID: "project-1", Root: env.workingDir},
		MemoryRecorder: true,
	}).(*sessionAgent)
	session, err := env.sessions.Create(t.Context(), "idle recorder")
	require.NoError(t, err)
	_, err = env.messages.Create(t.Context(), session.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "Remember to run focused checks first."}},
	})
	require.NoError(t, err)
	assistant, err := env.messages.Create(t.Context(), session.ID, message.CreateMessageParams{
		Role:  message.Assistant,
		Parts: []message.ContentPart{message.TextContent{Text: "Understood."}},
	})
	require.NoError(t, err)
	assistant.AddFinish(message.FinishReasonEndTurn, "", "")
	require.NoError(t, env.messages.Update(t.Context(), assistant))

	sa.activeRequests.Set(session.ID, func() {})
	sa.scheduleMemoryRecording(t.Context(), session.ID)
	sa.waitForMemoryJobs(3 * time.Second)
	records, err := store.List(t.Context(), "project-1", memory.StatusActive, 10)
	require.NoError(t, err)
	require.Empty(t, records)

	sa.activeRequests.Del(session.ID)
	sa.scheduleMemoryRecording(t.Context(), session.ID)
	require.False(t, sa.IsSessionBusy(session.ID), "passive recorder must not make the session visibly busy")
	sa.waitForMemoryJobs(3 * time.Second)
	records, err = store.List(t.Context(), "project-1", memory.StatusActive, 10)
	require.NoError(t, err)
	require.Len(t, records, 1)
}

func TestCancelMemoryJobPreemptsRecorderDebounce(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	store, err := memory.Open(t.Context(), memory.Options{Directory: t.TempDir()})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	model := &autoReviewStreamModel{text: "{\"candidates\":[]}"}
	configured := Model{
		Model:      model,
		CatwalkCfg: catwalk.Model{ContextWindow: 30_000, DefaultMaxTokens: 2_000},
		ModelCfg:   config.SelectedModel{Provider: "fake", Model: "fake-model"},
	}
	sa := NewSessionAgent(SessionAgentOptions{
		Models:         SessionAgentModels{Large: configured, Small: configured, Summary: configured, Review: configured},
		Sessions:       env.sessions,
		Messages:       env.messages,
		Memory:         store,
		MemoryProject:  memory.Project{ID: "project-1", Root: env.workingDir},
		MemoryRecorder: true,
	}).(*sessionAgent)
	session, err := env.sessions.Create(t.Context(), "cancel recorder")
	require.NoError(t, err)
	_, err = env.messages.Create(t.Context(), session.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "A completed turn."}},
	})
	require.NoError(t, err)

	sa.scheduleMemoryRecording(t.Context(), session.ID)
	sa.cancelMemoryJob(session.ID)
	sa.waitForMemoryJobs(3 * time.Second)
	cursor, err := store.RecorderCursor(t.Context(), session.ID)
	require.NoError(t, err)
	require.Empty(t, cursor.MessageID)
	require.Zero(t, model.streamCalls.Load())
}
