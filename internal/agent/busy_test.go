package agent

import (
	"context"
	"fmt"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/message"
)

func TestIsSessionBusyAcrossKeys(t *testing.T) {
	t.Parallel()

	a := &sessionAgent{
		activeRequests: csync.NewMap[string, context.CancelFunc](),
	}
	noop := context.CancelFunc(func() {})

	if a.IsSessionBusy("s1") {
		t.Fatalf("expected empty agent to report not busy")
	}

	a.activeRequests.Set("s1", noop)
	if !a.IsSessionBusy("s1") {
		t.Fatalf("expected busy when regular request key is set")
	}
	a.activeRequests.Del("s1")

	a.activeRequests.Set("s1-summarize", noop)
	if !a.IsSessionBusy("s1") {
		t.Fatalf("expected busy when summarize key is set — prevents concurrent generations during summary")
	}

	if a.IsSessionBusy("s2") {
		t.Fatalf("summarize key for s1 must not leak into s2 busy check")
	}
}

func TestSummarizeActiveRequestLifespan(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	sess, err := env.sessions.Create(t.Context(), "New Session")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	_, err = env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "summarize me"}},
	})
	if err != nil {
		t.Fatalf("create user message: %v", err)
	}

	streamStarted := make(chan struct{})
	allowStreamFinish := make(chan struct{})
	streamCalls := make(chan string, 2)
	model := &summaryLifecycleModel{
		streamFunc: func(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
			streamCalls <- call.Prompt[len(call.Prompt)-1].Content[0].(fantasy.TextPart).Text
			return func(yield func(fantasy.StreamPart) bool) {
				select {
				case streamStarted <- struct{}{}:
				default:
				}
				<-allowStreamFinish
				if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "text-1"}) {
					return
				}
				if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "text-1", Delta: "summary"}) {
					return
				}
				if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "text-1"}) {
					return
				}
				yield(fantasy.StreamPart{
					Type:         fantasy.StreamPartTypeFinish,
					Usage:        fantasy.Usage{InputTokens: 10, OutputTokens: 2, TotalTokens: 12},
					FinishReason: fantasy.FinishReasonStop,
				})
			}, nil
		},
	}
	a := NewSessionAgent(SessionAgentOptions{
		LargeModel: Model{
			Model:    model,
			ModelCfg: config.SelectedModel{Provider: "test-provider", Model: "test-model"},
		},
		SmallModel: Model{
			Model:    model,
			ModelCfg: config.SelectedModel{Provider: "test-provider", Model: "test-model"},
		},
		Sessions: env.sessions,
		Messages: env.messages,
	}).(*sessionAgent)

	summarizeDone := make(chan error, 1)
	go func() {
		summarizeDone <- a.Summarize(t.Context(), sess.ID, fantasy.ProviderOptions{})
	}()

	<-streamStarted
	if _, ok := a.activeRequests.Get(sess.ID); ok {
		t.Fatalf("summarize must not occupy the regular request key")
	}
	if _, ok := a.activeRequests.Get(summarizeSessionKey(sess.ID)); !ok {
		t.Fatalf("summarize must occupy the summary request key")
	}
	if !a.IsSessionBusy(sess.ID) {
		t.Fatalf("session must report busy while summarize is active")
	}

	_, err = a.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "queued while summarizing"})
	if err != nil {
		t.Fatalf("queue run while summarizing: %v", err)
	}
	if queued := a.QueuedPrompts(sess.ID); queued != 1 {
		t.Fatalf("expected one queued prompt while summarizing, got %d", queued)
	}

	close(allowStreamFinish)
	if err := <-summarizeDone; err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if _, ok := a.activeRequests.Get(summarizeSessionKey(sess.ID)); ok {
		t.Fatalf("summarize key must be released after summary completes")
	}
	if a.IsSessionBusy(sess.ID) {
		t.Fatalf("session must not stay busy after queued prompt is drained")
	}
	if queued := a.QueuedPrompts(sess.ID); queued != 0 {
		t.Fatalf("queued prompt must be drained after summarize completes, got %d", queued)
	}
	firstPrompt := <-streamCalls
	if firstPrompt != "Provide a detailed summary of our conversation above." {
		t.Fatalf("expected summary prompt first, got %q", firstPrompt)
	}
	if secondPrompt := <-streamCalls; secondPrompt != "queued while summarizing" {
		t.Fatalf("queued run did not start after summarize released the summary key, got %q", secondPrompt)
	}
}

type summaryLifecycleModel struct {
	streamFunc func(context.Context, fantasy.Call) (fantasy.StreamResponse, error)
}

func (m *summaryLifecycleModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return nil, fmt.Errorf("generate not implemented")
}

func (m *summaryLifecycleModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	return m.streamFunc(ctx, call)
}

func (m *summaryLifecycleModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, fmt.Errorf("generate object not implemented")
}

func (m *summaryLifecycleModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, fmt.Errorf("stream object not implemented")
}

func (m *summaryLifecycleModel) Provider() string {
	return "test-provider"
}

func (m *summaryLifecycleModel) Model() string {
	return "test-model"
}
