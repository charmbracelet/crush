package agent

import (
	"context"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/stretchr/testify/require"
)

// toolPaletteModel records the tool names the turn offered the model,
// then finishes cleanly.
type toolPaletteModel struct {
	finishStreamModel
	names chan []string
}

func (m *toolPaletteModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	names := make([]string, 0, len(call.Tools))
	for _, tool := range call.Tools {
		names = append(names, tool.GetName())
	}
	select {
	case m.names <- names:
	default:
	}
	return m.finishStreamModel.Stream(ctx, call)
}

// TestRun_NonInteractiveTurnWithholdsTheQuestionTool pins the per-run
// half of the interactivity split. One coordinator now serves an
// attached TUI and headless `crush run` prompts at the same time, so the
// question tool cannot be decided when the coordinator is built.
//
// A run with nobody to answer must not be offered the tool: it would
// block until the run's context died, and `crush run` would look hung.
// An ordinary run must still get it.
func TestRun_NonInteractiveTurnWithholdsTheQuestionTool(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name           string
		nonInteractive bool
		wantQuestion   bool
	}{
		{name: "interactive", wantQuestion: true},
		{name: "non interactive", nonInteractive: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			env := testEnv(t)

			sess, err := env.sessions.Create(t.Context(), "session")
			require.NoError(t, err)

			model := &toolPaletteModel{names: make(chan []string, 1)}
			sa := NewSessionAgent(SessionAgentOptions{
				LargeModel:  Model{Model: model, CatwalkCfg: catwalk.Model{ContextWindow: 200000, DefaultMaxTokens: 10000}},
				SmallModel:  Model{Model: &finishStreamModel{text: "title"}, CatwalkCfg: catwalk.Model{ContextWindow: 200000, DefaultMaxTokens: 10000}},
				Sessions:    env.sessions,
				Messages:    env.messages,
				Tools:       []fantasy.AgentTool{tools.NewQuestionTool(question.NewService())},
				Permissions: env.permissions,
			}).(*sessionAgent)

			_, err = sa.Run(t.Context(), SessionAgentCall{
				SessionID:      sess.ID,
				Prompt:         "hi",
				NonInteractive: tc.nonInteractive,
			})
			require.NoError(t, err)

			names := <-model.names
			if tc.wantQuestion {
				require.Contains(t, names, tools.QuestionToolName,
					"an interactive run must still be able to ask the user")
				return
			}
			require.NotContains(t, names, tools.QuestionToolName,
				"a run with nobody to answer must not be offered the question tool")
		})
	}
}
