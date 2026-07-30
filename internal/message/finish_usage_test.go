package message

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAddFinishWithUsageRecordsUsage(t *testing.T) {
	t.Parallel()
	var m Message
	m.AddFinishWithUsage(FinishReasonEndTurn, "", "", 146385, 772, 0.05)

	fin := m.FinishPart()
	require.NotNil(t, fin)
	require.Equal(t, int64(146385), fin.PromptTokens)
	require.Equal(t, int64(772), fin.CompletionTokens)
	require.InDelta(t, 0.05, fin.Cost, 1e-12)
}

// TestAddFinishPreservesRecordedUsage covers the accounting hole that a
// terminal AddFinish used to open. The agent records a step's tokens and cost
// on the assistant message and adds the same cost to session.Cost; every
// cancel and error path then calls AddFinish on that SAME message. Because
// AddFinish replaces the finish part, the usage was erased while the money
// stayed on the session row, so the per-model breakdown could not reconcile.
func TestAddFinishPreservesRecordedUsage(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		finish func(m *Message)
	}{
		{"canceled", func(m *Message) {
			m.AddFinish(FinishReasonCanceled, "User canceled request", "")
		}},
		{"error", func(m *Message) {
			m.AddFinish(FinishReasonError, "Provider Error", "boom")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var m Message
			m.AddFinishWithUsage(FinishReasonToolUse, "", "", 146385, 772, 0.05)
			tc.finish(&m)

			fin := m.FinishPart()
			require.NotNil(t, fin)
			require.Equal(t, int64(146385), fin.PromptTokens, "prompt tokens must survive a terminal finish")
			require.Equal(t, int64(772), fin.CompletionTokens, "completion tokens must survive a terminal finish")
			require.InDelta(t, 0.05, fin.Cost, 1e-12, "cost must survive a terminal finish")

			// The terminal reason and text must still win.
			require.NotEqual(t, FinishReasonToolUse, fin.Reason)
			require.NotEmpty(t, fin.Message)

			// Exactly one finish part.
			var count int
			for _, part := range m.Parts {
				if _, ok := part.(Finish); ok {
					count++
				}
			}
			require.Equal(t, 1, count)
		})
	}
}

// TestAddFinishWithUsageOverwritesWithFreshUsage guards the other direction:
// a call that DOES carry usage must replace, not merge.
func TestAddFinishWithUsageOverwritesWithFreshUsage(t *testing.T) {
	t.Parallel()
	var m Message
	m.AddFinishWithUsage(FinishReasonToolUse, "", "", 10, 20, 0.5)
	m.AddFinishWithUsage(FinishReasonEndTurn, "", "", 1, 2, 0.25)

	fin := m.FinishPart()
	require.NotNil(t, fin)
	require.Equal(t, int64(1), fin.PromptTokens)
	require.Equal(t, int64(2), fin.CompletionTokens)
	require.InDelta(t, 0.25, fin.Cost, 1e-12)
}

// TestResetStreamedContentDropsRecordedUsage documents the deliberate discard
// path: a provider retry throws the failed attempt's usage away, so the
// carry-forward above must not resurrect it.
func TestResetStreamedContentDropsRecordedUsage(t *testing.T) {
	t.Parallel()
	var m Message
	m.AddFinishWithUsage(FinishReasonEndTurn, "", "", 100, 50, 0.05)
	m.ResetStreamedContent()
	require.Nil(t, m.FinishPart())

	m.AddFinish(FinishReasonError, "Provider Error", "boom")
	fin := m.FinishPart()
	require.NotNil(t, fin)
	require.Zero(t, fin.PromptTokens)
	require.Zero(t, fin.CompletionTokens)
	require.Zero(t, fin.Cost)
}
