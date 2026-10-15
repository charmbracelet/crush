package agent

import (
	"context"
	"sync"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

// E: two clients (or two reloads) read the same dead session at once. Settle-on-read has
// no per-session exclusion, so each reader sees the same orphan and each writes a result.
func TestConcurrentSettleWritesOneResult(t *testing.T) {
	env := testEnv(t)
	ctx := t.Context()
	sess, err := env.sessions.Create(ctx, "s")
	require.NoError(t, err)
	var calls []message.ContentPart
	for _, id := range []string{"c1", "c2", "c3"} {
		calls = append(calls, message.ToolCall{ID: id, Name: "bash", Input: "{}", Finished: true})
	}
	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{Role: message.Assistant, Parts: calls})
	require.NoError(t, err)

	const readers = 8
	var wg sync.WaitGroup
	start := make(chan struct{})
	for range readers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, _ = ReadSettledMessages(ctx, env.messages, nil, sess.ID)
		}()
	}
	close(start)
	wg.Wait()

	msgs, err := env.messages.List(ctx, sess.ID)
	require.NoError(t, err)
	per := map[string]int{}
	for _, m := range msgs {
		for _, tr := range m.ToolResults() {
			per[tr.ToolCallID]++
		}
	}
	t.Logf("tool results per call after %d concurrent readers: %v", readers, per)
	for id, n := range per {
		require.Equalf(t, 1, n, "call %s answered %d times", id, n)
	}
}

// hookService runs a callback the first time List is called, to model a run that starts
// between the busy check and the read.
type hookService struct {
	message.Service
	once sync.Once
	hook func()
}

func (h *hookService) List(ctx context.Context, sessionID string) ([]message.Message, error) {
	h.once.Do(h.hook)
	return h.Service.List(ctx, sessionID)
}

// G: TOCTOU. busy() says idle; a run begins its first tool call before the read; the settle
// writes "Interrupted" over a live call and the real result lands later => two results.

// E2: what the model is sent after the duplicate settle.
func TestModelSeesOneResultAfterConcurrentSettle(t *testing.T) {
	env := testEnv(t)
	ctx := t.Context()
	sess, err := env.sessions.Create(ctx, "s")
	require.NoError(t, err)
	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "go"}}})
	require.NoError(t, err)
	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{Role: message.Assistant,
		Parts: []message.ContentPart{message.ToolCall{ID: "c1", Name: "bash", Input: "{}", Finished: true}}})
	require.NoError(t, err)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for range 4 {
		wg.Add(1)
		go func() { defer wg.Done(); <-start; _, _ = ReadSettledMessages(ctx, env.messages, nil, sess.ID) }()
	}
	close(start)
	wg.Wait()
	msgs, err := env.messages.List(ctx, sess.ID)
	require.NoError(t, err)
	sa := &sessionAgent{}
	prompt, _ := sa.preparePrompt(msgs, false)
	for i, m := range prompt {
		n := 0
		for _, p := range m.Content {
			if tr, ok := p.(fantasy.ToolResultPart); ok && tr.ToolCallID == "c1" {
				n++
			}
		}
		t.Logf("prompt[%d] role=%s parts=%d tool_results_for_c1=%d", i, m.Role, len(m.Content), n)
	}
}
