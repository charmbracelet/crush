package cmd

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/client"
	"github.com/stretchr/testify/require"
)

// fakeRunCanceller ghi lại các lời huỷ; unsupported giả lập server cũ.
type fakeRunCanceller struct {
	unsupported bool
	runCancels  []string
	sessCancels []string
}

func (f *fakeRunCanceller) CancelAgentRun(_ context.Context, _, sessionID, runID string) error {
	if f.unsupported {
		return client.ErrCancelRunUnsupported
	}
	f.runCancels = append(f.runCancels, sessionID+"/"+runID)
	return nil
}

func (f *fakeRunCanceller) CancelAgentSession(_ context.Context, _, sessionID string) error {
	f.sessCancels = append(f.sessCancels, sessionID)
	return nil
}

// Ctrl+C trên run huỷ đúng lượt của nó theo RunID, kể cả trên session có sẵn
// (-s/-C): lượt khác và hàng đợi của session không bị đụng.
func TestCancelOwnRun_CancelsByRunID(t *testing.T) {
	t.Parallel()
	for _, own := range []bool{true, false} {
		f := &fakeRunCanceller{}
		require.NoError(t, cancelOwnRun(t.Context(), f, "ws", "S", "run-1", own))
		require.Equal(t, []string{"S/run-1"}, f.runCancels)
		require.Empty(t, f.sessCancels, "never cancel the whole session when the server can cancel one run")
	}
}

// Server cũ không huỷ theo RunID: session do run tạo thì huỷ cả session
// (chỉ có lượt của run trên đó); session có sẵn thì không đụng, báo run vẫn chạy.
func TestCancelOwnRun_OldServerFallback(t *testing.T) {
	t.Parallel()
	f := &fakeRunCanceller{unsupported: true}
	require.NoError(t, cancelOwnRun(t.Context(), f, "ws", "S", "run-1", true))
	require.Equal(t, []string{"S"}, f.sessCancels)

	f = &fakeRunCanceller{unsupported: true}
	err := cancelOwnRun(t.Context(), f, "ws", "S", "run-1", false)
	require.ErrorContains(t, err, "the run keeps going on session S")
	require.Empty(t, f.sessCancels, "another client's turn on an existing session must not be cancelled")
}
