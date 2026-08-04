package agent

import (
	"context"
	"errors"
	"math"
	"sync/atomic"
	"testing"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

// usageStreamModel is a fantasy.LanguageModel that reports a fixed token usage
// on its finish part, optionally after emitting one tool call. It exists so a
// real sessionAgent.Run can be driven to the production OnStepFinish path
// without a provider cassette, which is the only way to prove that the step
// usage a `session show --by-model` breakdown reads is actually WRITTEN.
type usageStreamModel struct {
	usage fantasy.Usage
	// toolName, when set, makes the first TOOL-BEARING step emit a call to
	// that tool and finish with FinishReasonToolCalls. Title generation shares
	// the model instance but runs a tool-free agent, so gating on
	// len(call.Tools) keeps the two from racing over this counter.
	toolName  string
	toolSteps atomic.Int32
}

func (m *usageStreamModel) Provider() string { return "fake" }
func (m *usageStreamModel) Model() string    { return "fake-model" }

func (m *usageStreamModel) Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
	return nil, errors.New("not implemented")
}

func (m *usageStreamModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	emitTool := m.toolName != "" && len(call.Tools) > 0 && m.toolSteps.Add(1) == 1
	return func(yield func(fantasy.StreamPart) bool) {
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "1"}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "1", Delta: "working"}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "1"}) {
			return
		}
		if emitTool {
			if !yield(fantasy.StreamPart{
				Type:          fantasy.StreamPartTypeToolInputStart,
				ID:            "call-1",
				ToolCallName:  m.toolName,
				ToolCallInput: "{}",
			}) {
				return
			}
			if !yield(fantasy.StreamPart{
				Type:          fantasy.StreamPartTypeToolCall,
				ID:            "call-1",
				ToolCallName:  m.toolName,
				ToolCallInput: "{}",
			}) {
				return
			}
			yield(fantasy.StreamPart{
				Type:         fantasy.StreamPartTypeFinish,
				FinishReason: fantasy.FinishReasonToolCalls,
				Usage:        m.usage,
			})
			return
		}
		yield(fantasy.StreamPart{
			Type:         fantasy.StreamPartTypeFinish,
			FinishReason: fantasy.FinishReasonStop,
			Usage:        m.usage,
		})
	}, nil
}

func (m *usageStreamModel) GenerateObject(ctx context.Context, call fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *usageStreamModel) StreamObject(ctx context.Context, call fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

// pricedModel wraps a language model in a catwalk config with real per-token
// prices so a step produces a non-zero cost.
func pricedModel(m fantasy.LanguageModel) Model {
	return Model{
		Model: m,
		CatwalkCfg: catwalk.Model{
			ContextWindow:      200000,
			DefaultMaxTokens:   10000,
			CostPer1MIn:        3,
			CostPer1MOut:       15,
			CostPer1MInCached:  3.75,
			CostPer1MOutCached: 0.3,
		},
	}
}

func newPricedAgent(t *testing.T, m fantasy.LanguageModel, tools ...fantasy.AgentTool) (*sessionAgent, fakeEnv) {
	t.Helper()
	return newPricedAgentOpts(t, m, true, tools...)
}

func newPricedAgentOpts(t *testing.T, m fantasy.LanguageModel, disableAutoSummarize bool, tools ...fantasy.AgentTool) (*sessionAgent, fakeEnv) {
	t.Helper()
	env := testEnv(t)
	priced := pricedModel(m)
	sa := NewSessionAgent(SessionAgentOptions{
		LargeModel:           priced,
		SmallModel:           priced,
		SystemPrompt:         "system",
		IsYolo:               true,
		Sessions:             env.sessions,
		Messages:             env.messages,
		Tools:                tools,
		DisableAutoSummarize: disableAutoSummarize,
	}).(*sessionAgent)
	return sa, env
}

// seedUserPrompt writes a user text message so Run's
// `if !hasUserTextMessage(msgs)` guard skips title generation. Title generation
// runs concurrently on the same model instance and adds its own cost to the
// session row, which would make single-step cost assertions non-deterministic.
// TestGenerateTitleUsageIsAttributable covers that path on purpose instead.
func seedUserPrompt(t *testing.T, env fakeEnv, sessionID string) {
	t.Helper()
	_, err := env.messages.Create(t.Context(), sessionID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "earlier"}},
	})
	require.NoError(t, err)
}

func assistantMessages(t *testing.T, env fakeEnv, sessionID string) []message.Message {
	t.Helper()
	require.NoError(t, env.messages.FlushAll(t.Context()))
	msgs, err := env.messages.List(t.Context(), sessionID)
	require.NoError(t, err)
	var out []message.Message
	for _, msg := range msgs {
		if msg.Role == message.Assistant {
			out = append(out, msg)
		}
	}
	return out
}

// TestRunPersistsStepUsageOnAssistantMessage is the write-side guard for the
// per-model cost breakdown. It runs a real turn through sessionAgent.Run and
// asserts the PERSISTED assistant message carries the step's tokens and cost.
// Deleting the AddFinishWithUsage call in OnStepFinish must fail this test;
// without it the breakdown reads zeros while session.Cost keeps the money.
func TestRunPersistsStepUsageOnAssistantMessage(t *testing.T) {
	t.Parallel()
	usage := fantasy.Usage{
		InputTokens:         1_000_000,
		OutputTokens:        200_000,
		CacheReadTokens:     500_000,
		CacheCreationTokens: 100_000,
	}
	sa, env := newPricedAgent(t, &usageStreamModel{usage: usage})

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)
	seedUserPrompt(t, env, sess.ID)

	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "go"})
	require.NoError(t, err)

	// Per 1M tokens: 3 in, 15 out, 0.3 cached-read, 3.75 cached-write.
	const wantCost = 3.0 + 3.0 + 0.15 + 0.375

	msgs := assistantMessages(t, env, sess.ID)
	require.Len(t, msgs, 1)
	fin := msgs[0].FinishPart()
	require.NotNil(t, fin, "the persisted assistant message must carry a finish part")
	require.Equal(t, int64(1_600_000), fin.PromptTokens,
		"prompt tokens (input + cache creation + cache read) must be recorded on the message")
	require.Equal(t, int64(200_000), fin.CompletionTokens)
	require.InDelta(t, wantCost, fin.Cost, 1e-9, "the step cost must be recorded on the message")

	// And it must reconcile with what landed on the session row.
	updated, err := env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.InDelta(t, wantCost, updated.Cost, 1e-9)
	require.InDelta(t, updated.Cost, fin.Cost, 1e-9,
		"session.Cost must equal the cost recorded on the assistant messages")
	require.Equal(t, int64(1_600_000), updated.PromptTokens,
		"the session context counter must agree with the recorded step prompt tokens")
}

// cancelAfterStepUsageSessions cancels the run from inside the session save
// that OnStepFinish performs right after recording a step's tokens and cost.
// When that step ended on a tool call, fantasy loops for another step and
// observes the canceled context BEFORE PrepareStep creates the next assistant
// message, so Run's terminal AddFinish lands on the SAME message that already
// holds the step usage. That is the production window the carry-forward guards.
type cancelAfterStepUsageSessions struct {
	session.Service
	fired atomic.Bool
	fire  func()
}

func (c *cancelAfterStepUsageSessions) Save(ctx context.Context, sess session.Session) (session.Session, error) {
	out, err := c.Service.Save(ctx, sess)
	if sess.Cost > 0 && c.fired.CompareAndSwap(false, true) {
		c.fire()
	}
	return out, err
}

func noopTool() fantasy.AgentTool {
	return fantasy.NewAgentTool("noop", "does nothing",
		func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse("ok"), nil
		})
}

// TestTerminalFinishKeepsPersistedStepUsage covers the accounting hole the
// cancel and error paths used to open: the step usage is recorded and added to
// session.Cost, then AddFinish replaces the finish part on that same message.
// Before the fix the money stayed on the session row while the tokens and cost
// vanished from the message, so no breakdown could reconcile.
func TestTerminalFinishKeepsPersistedStepUsage(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	priced := pricedModel(&usageStreamModel{
		usage:    fantasy.Usage{InputTokens: 1_000_000, OutputTokens: 200_000},
		toolName: "noop",
	})

	var sa *sessionAgent
	var sessID string
	sessions := &cancelAfterStepUsageSessions{Service: env.sessions}
	sessions.fire = func() { sa.Cancel(sessID) }

	sa = NewSessionAgent(SessionAgentOptions{
		LargeModel:           priced,
		SmallModel:           priced,
		SystemPrompt:         "system",
		IsYolo:               true,
		Sessions:             sessions,
		Messages:             env.messages,
		Tools:                []fantasy.AgentTool{noopTool()},
		DisableAutoSummarize: true,
	}).(*sessionAgent)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)
	sessID = sess.ID
	seedUserPrompt(t, env, sess.ID)

	_, runErr := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "go"})
	require.Error(t, runErr, "the cancellation must abort the turn")

	updated, err := env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Greater(t, updated.Cost, 0.0, "the aborted step still cost money")

	msgs := assistantMessages(t, env, sess.ID)
	require.NotEmpty(t, msgs)

	var recorded float64
	var sawTerminal bool
	for _, msg := range msgs {
		fin := msg.FinishPart()
		if fin == nil {
			continue
		}
		recorded += fin.Cost
		if fin.Reason == message.FinishReasonCanceled || fin.Reason == message.FinishReasonError {
			sawTerminal = true
			require.NotZero(t, fin.PromptTokens,
				"a terminal finish must not erase the prompt tokens already recorded for the step")
			require.NotZero(t, fin.Cost,
				"a terminal finish must not erase the cost already added to session.Cost")
		}
	}
	require.True(t, sawTerminal, "the turn must end on a canceled or error finish part")
	require.InDelta(t, updated.Cost, recorded, 1e-9,
		"cost recorded on the messages must still reconcile with session.Cost after an aborted turn")
}

// TestSummarizationUsageIsRecorded covers the second accounting hole: the
// summarization request's cost was added to session.Cost while its finish part
// carried no usage at all, so an auto-summarized session could never reconcile.
func TestSummarizationUsageIsRecorded(t *testing.T) {
	t.Parallel()
	// Usage well past the 200k context window forces the auto-summarize
	// StopWhen condition.
	usage := fantasy.Usage{InputTokens: 1_000_000, OutputTokens: 200_000}
	sa, env := newPricedAgentOpts(t, &usageStreamModel{usage: usage}, false)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)
	seedUserPrompt(t, env, sess.ID)

	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "go"})
	require.NoError(t, err)

	msgs := assistantMessages(t, env, sess.ID)
	require.Len(t, msgs, 2, "one step message plus the summary message")

	summary := msgs[1]
	require.True(t, summary.IsSummaryMessage)
	fin := summary.FinishPart()
	require.NotNil(t, fin)
	require.NotZero(t, fin.PromptTokens, "the summarization request's tokens must be recorded")
	require.NotZero(t, fin.Cost, "the summarization request's cost must be recorded")

	var recorded float64
	for _, msg := range msgs {
		if f := msg.FinishPart(); f != nil {
			recorded += f.Cost
		}
	}
	updated, err := env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.InDelta(t, updated.Cost, recorded, 1e-9,
		"summarization cost must be attributable, not just added to session.Cost")
}

// TestGenerateTitleUsageIsAttributable covers the third accounting hole: title
// generation runs on the first prompt of every session and adds its cost to the
// session row through UpdateTitleAndUsage, but wrote no message anywhere, so
// every session carried unattributable spend. The usage now lands in the
// session's title sub-session, where the by-model walk finds it.
func TestGenerateTitleUsageIsAttributable(t *testing.T) {
	t.Parallel()
	usage := fantasy.Usage{InputTokens: 1_000, OutputTokens: 200}
	sa, env := newPricedAgent(t, &usageStreamModel{usage: usage})

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	// No seeded user prompt, so title generation runs.
	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "go"})
	require.NoError(t, err)

	// Title generation runs on the agent's own goroutine: the sub-session,
	// its priced message, and the parent's cost row each commit
	// asynchronously, after Run returns. Poll for every stage instead of
	// racing the writes.
	var childID string
	require.Eventually(t, func() bool {
		children, err := env.sessions.ListChildren(t.Context(), sess.ID)
		if err != nil || len(children) != 1 {
			return false
		}
		childID = children[0].ID
		return children[0].Cost > 0
	}, 5*time.Second, 20*time.Millisecond,
		"title generation must record its usage in a sub-session")

	var titleFin *message.Finish
	require.Eventually(t, func() bool {
		titleMsgs := assistantMessages(t, env, childID)
		if len(titleMsgs) != 1 {
			return false
		}
		titleFin = titleMsgs[0].FinishPart()
		return titleFin != nil && titleFin.PromptTokens > 0
	}, 5*time.Second, 20*time.Millisecond,
		"title generation must write a priced message")
	// Re-read the child so the cost comparison uses a fresh row, not a stale
	// capture from inside the polling closure.
	children, err := env.sessions.ListChildren(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Equal(t, childID, children[0].ID)
	require.InDelta(t, children[0].Cost, titleFin.Cost, 1e-9)

	var recorded float64
	for _, msg := range assistantMessages(t, env, sess.ID) {
		if f := msg.FinishPart(); f != nil {
			recorded += f.Cost
		}
	}
	recorded += titleFin.Cost

	require.Eventually(t, func() bool {
		updated, err := env.sessions.Get(t.Context(), sess.ID)
		if err != nil {
			return false
		}
		return math.Abs(updated.Cost-recorded) <= 1e-9
	}, 5*time.Second, 20*time.Millisecond,
		"title generation cost must be attributable to the model that incurred it")
}
