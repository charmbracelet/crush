package agent

import (
	"context"
	"errors"
	"sync"
	"testing"

	"charm.land/catwalk/v3/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

const (
	// A 1000 input and 2000 output token step at 10 and 20 per million
	// tokens costs 0.01 + 0.04.
	pricedStepCost = 0.05
	rolledUpCost   = 0.40
)

// pricedStreamModel streams one text part and finishes with reported usage.
type pricedStreamModel struct{}

func (pricedStreamModel) Provider() string { return "fake" }
func (pricedStreamModel) Model() string    { return "priced-model" }

func (pricedStreamModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return nil, errors.New("not implemented")
}

func (pricedStreamModel) Stream(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
	return func(yield func(fantasy.StreamPart) bool) {
		for _, part := range []fantasy.StreamPart{
			{Type: fantasy.StreamPartTypeTextStart, ID: "1"},
			{Type: fantasy.StreamPartTypeTextDelta, ID: "1", Delta: "done"},
			{Type: fantasy.StreamPartTypeTextEnd, ID: "1"},
			{
				Type:         fantasy.StreamPartTypeFinish,
				FinishReason: fantasy.FinishReasonStop,
				Usage:        fantasy.Usage{InputTokens: 1000, OutputTokens: 2000, TotalTokens: 3000},
			},
		} {
			if !yield(part) {
				return
			}
		}
	}, nil
}

func (pricedStreamModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (pricedStreamModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

// costRaceSessions lands a concurrent AddCost, like a background subagent's
// roll-up, immediately before the first Save, which is the point where a
// stale read-modify-write would overwrite it.
type costRaceSessions struct {
	session.Service
	once sync.Once
	err  error
}

func (s *costRaceSessions) Save(ctx context.Context, sess session.Session) (session.Session, error) {
	s.once.Do(func() { s.err = s.Service.AddCost(ctx, sess.ID, rolledUpCost) })
	return s.Service.Save(ctx, sess)
}

func newPricedAgent(env fakeEnv, sessions session.Service) *sessionAgent {
	priced := Model{
		Model: pricedStreamModel{},
		CatwalkCfg: catwalk.Model{
			ContextWindow:    200000,
			DefaultMaxTokens: 10000,
			Pricing:          catwalk.Pricing{Input: 10, Output: 20},
		},
	}
	free := Model{
		Model:      &finishStreamModel{text: "title"},
		CatwalkCfg: catwalk.Model{ContextWindow: 200000, DefaultMaxTokens: 10000},
	}
	return NewSessionAgent(SessionAgentOptions{
		Models:   fixedModels(priced, free),
		Sessions: sessions,
		Messages: env.messages,
	}).(*sessionAgent)
}

func TestRunBillsStepCostExactlyOnce(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	sa := newPricedAgent(env, env.sessions)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "hi"})
	require.NoError(t, err)

	updated, err := env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.InDelta(t, pricedStepCost, updated.Cost, 1e-9)
}

func TestRunKeepsCostRolledUpWhileStepFinishes(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	racing := &costRaceSessions{Service: env.sessions}
	sa := newPricedAgent(env, racing)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "hi"})
	require.NoError(t, err)
	require.NoError(t, racing.err)

	updated, err := env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.InDelta(t, pricedStepCost+rolledUpCost, updated.Cost, 1e-9,
		"the step's Save must neither drop the roll-up nor double count the step")
}

func TestSummarizeKeepsCostRolledUpDuringTheSummaryCall(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	racing := &costRaceSessions{Service: env.sessions}
	sa := newPricedAgent(env, racing)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)
	_, err = env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "hello"}},
	})
	require.NoError(t, err)

	require.NoError(t, sa.Summarize(t.Context(), sess.ID, fantasy.ProviderOptions{}, nil))
	require.NoError(t, racing.err)

	updated, err := env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.NotEmpty(t, updated.SummaryMessageID)
	require.InDelta(t, pricedStepCost+rolledUpCost, updated.Cost, 1e-9,
		"the summary's Save must neither drop the roll-up nor double count the summary")
}
