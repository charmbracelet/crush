package agent

import (
	"sync/atomic"
	"testing"
	"time"

	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

// TestRun_ModelSwitchMidRunAppliesToNextPrompt switches the model while a
// turn runs with a prompt queued behind it. The running turn and the prompt
// queued before the switch keep the model their options were built for; a
// prompt sent after the switch runs on the new model.
func TestRun_ModelSwitchMidRunAppliesToNextPrompt(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	broker := pubsub.NewBroker[notify.RunComplete]()
	t.Cleanup(broker.Shutdown)

	oldLM := &gatedStreamModel{text: "old", gate: make(chan struct{}), entered: make(chan struct{})}
	newLM := &gatedStreamModel{text: "new", gate: make(chan struct{}), entered: make(chan struct{})}
	close(newLM.gate)
	catwalkCfg := catwalk.Model{ContextWindow: 200000, DefaultMaxTokens: 10000}
	oldModel := Model{Model: oldLM, CatwalkCfg: catwalkCfg, ModelCfg: config.SelectedModel{Provider: "fake", Model: "old"}}
	newModel := Model{Model: newLM, CatwalkCfg: catwalkCfg, ModelCfg: config.SelectedModel{Provider: "fake", Model: "new"}}
	small := Model{Model: &finishStreamModel{text: "title"}, CatwalkCfg: catwalkCfg}

	var current atomic.Pointer[ModelSet]
	current.Store(&ModelSet{Large: oldModel, Small: small})

	sa := NewSessionAgent(SessionAgentOptions{
		Models:      func() ModelSet { return *current.Load() },
		Sessions:    env.sessions,
		Messages:    env.messages,
		RunComplete: broker,
	}).(*sessionAgent)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	mainDone := make(chan error, 1)
	go func() {
		_, runErr := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, RunID: "main", Prompt: "main", Model: oldModel})
		mainDone <- runErr
	}()
	select {
	case <-oldLM.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("main run never entered Stream")
	}

	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, RunID: "queued", Prompt: "queued", Model: oldModel})
	require.NoError(t, err)
	require.Equal(t, 1, sa.QueuedPrompts(sess.ID))

	current.Store(&ModelSet{Large: newModel, Small: small})
	close(oldLM.gate)
	require.NoError(t, <-mainDone)

	require.Equal(t, int64(2), oldLM.calls.Load(), "the running turn and the prompt queued before the switch stay on the old model")
	require.Zero(t, newLM.calls.Load())

	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "next"})
	require.NoError(t, err)
	require.Equal(t, int64(1), newLM.calls.Load(), "the next prompt runs on the new model")
	require.Equal(t, int64(2), oldLM.calls.Load())
}

// TestPersistCanceledTurn_RecordsCallModel cancels a turn after the model
// was switched; the record names the model the turn ran on.
func TestPersistCanceledTurn_RecordsCallModel(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	catwalkCfg := catwalk.Model{ContextWindow: 200000, DefaultMaxTokens: 10000}
	oldModel := Model{Model: &finishStreamModel{text: "old"}, CatwalkCfg: catwalkCfg, ModelCfg: config.SelectedModel{Provider: "fake", Model: "old"}}
	newModel := Model{Model: &finishStreamModel{text: "new"}, CatwalkCfg: catwalkCfg, ModelCfg: config.SelectedModel{Provider: "fake", Model: "new"}}

	sa := NewSessionAgent(SessionAgentOptions{
		Models:   func() ModelSet { return ModelSet{Large: newModel, Small: newModel} },
		Sessions: env.sessions,
		Messages: env.messages,
	}).(*sessionAgent)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	call := SessionAgentCall{SessionID: sess.ID, Prompt: "hi", Model: oldModel}
	require.NoError(t, sa.persistCanceledTurn(t.Context(), call, false))

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Len(t, msgs, 2)
	require.Equal(t, "old", msgs[1].Model)
}

// TestSubAgentCall_PinsModel checks a subagent call carries the model its
// options were built for, so a switch while it is queued cannot split them.
func TestSubAgentCall_PinsModel(t *testing.T) {
	t.Parallel()

	c, _ := newOverrideTestCoordinator(t)
	model := c.mainModels().Large
	call, err := c.subAgentCall(&mockSessionAgent{model: model}, "sess", "task")
	require.NoError(t, err)
	require.Equal(t, model.ModelCfg, call.Model.ModelCfg)
}
