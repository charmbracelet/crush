package agent

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/lock"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

type deniedWriterSessions struct {
	session.Service
	err error
}

func (s deniedWriterSessions) AcquireWriter(context.Context, string) error {
	return s.err
}

func TestWriterRejectionPublishesTerminalEvent(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		err      error
		canceled bool
	}{
		{"contended", lock.ErrContended, false},
		{"canceled", context.Canceled, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := &sessionAgent{sessions: deniedWriterSessions{err: tc.err}}
			var completions []notify.RunComplete
			_, err := a.Run(t.Context(), SessionAgentCall{
				SessionID: "session", RunID: "run", Prompt: "Continue",
				OnComplete: func(event notify.RunComplete) { completions = append(completions, event) },
			})
			require.ErrorIs(t, err, tc.err)
			require.Len(t, completions, 1)
			require.Equal(t, "run", completions[0].RunID)
			require.Equal(t, tc.canceled, completions[0].Cancelled)
			require.Equal(t, tc.err.Error(), completions[0].Error)
		})
	}
}
