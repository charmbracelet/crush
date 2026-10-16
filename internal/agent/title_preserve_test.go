package agent

import (
	"context"
	"errors"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

// titleStreamModel trả về một tiêu đề LLM cố định; beforeStream chạy TRƯỚC khi
// stream trả về, mô phỏng người dùng đổi tên trong lúc tiêu đề LLM đang chạy.
type titleStreamModel struct {
	fastModel
	title        string
	beforeStream func()
}

func (m titleStreamModel) Stream(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
	if m.beforeStream != nil {
		m.beforeStream()
	}
	return func(yield func(fantasy.StreamPart) bool) {
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "1"}) ||
			!yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "1", Delta: m.title}) ||
			!yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "1"}) {
			return
		}
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop,
			Usage: fantasy.Usage{InputTokens: 20, OutputTokens: 5, TotalTokens: 25}})
	}, nil
}

func (titleStreamModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

// Tên người dùng tự đổi (F2/Ctrl+R) trong lúc tiêu đề LLM đang chạy không bị tiêu
// đề LLM ghi đè; usage vẫn được ghi.
func TestGenerateTitleKeepsUserRename(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	sess, err := env.sessions.Create(t.Context(), DefaultSessionName)
	require.NoError(t, err)

	model := titleStreamModel{title: "LLM Title", beforeStream: func() {
		require.NoError(t, env.sessions.Rename(t.Context(), sess.ID, "My Own Name"))
	}}
	sa := testSessionAgent(env, model, model, "system")
	sa.GenerateTitle(t.Context(), sess.ID, "do the thing")

	saved, err := env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Equal(t, "My Own Name", saved.Title, "a manual rename must survive the LLM title")
	require.Positive(t, saved.CompletionTokens, "usage must still be saved")
}

// Khi người dùng KHÔNG đổi tên, tiêu đề LLM vẫn ghi đè tên máy đặt như cũ.
func TestGenerateTitleStillWritesWhenMachineNamed(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	sess, err := env.sessions.Create(t.Context(), DefaultSessionName)
	require.NoError(t, err)

	model := titleStreamModel{title: "LLM Title"}
	sa := testSessionAgent(env, model, model, "system")
	sa.GenerateTitle(t.Context(), sess.ID, "do the thing")

	saved, err := env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Equal(t, "LLM Title", saved.Title)
}
