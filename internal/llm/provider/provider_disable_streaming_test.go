package provider

import (
    "context"
    "errors"
    "testing"
    "time"

    "github.com/charmbracelet/catwalk/pkg/catwalk"
    "github.com/charmbracelet/crush/internal/llm/tools"
    "github.com/charmbracelet/crush/internal/message"
)

type fakeClient struct {
    resp         *ProviderResponse
    err          error
    streamEvents []ProviderEvent
    sendCalled   bool
    streamCalled bool
}

func (f *fakeClient) send(ctx context.Context, messages []message.Message, tools []tools.BaseTool) (*ProviderResponse, error) {
    f.sendCalled = true
    return f.resp, f.err
}

func (f *fakeClient) stream(ctx context.Context, messages []message.Message, tools []tools.BaseTool) <-chan ProviderEvent {
    f.streamCalled = true
    ch := make(chan ProviderEvent, len(f.streamEvents))
    for _, e := range f.streamEvents {
        ch <- e
    }
    close(ch)
    return ch
}

func (f *fakeClient) Model() catwalk.Model { return catwalk.Model{ID: "fake"} }

// Test that when disableStreaming is true, StreamResponse emits a single EventComplete using send.
func TestBaseProvider_StreamResponse_FallbackComplete(t *testing.T) {
    t.Parallel()

    fc := &fakeClient{resp: &ProviderResponse{Content: "hello"}}
    p := &baseProvider[*fakeClient]{
        options: providerClientOptions{disableStreaming: true},
        client:  fc,
    }

    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    t.Cleanup(cancel)

    msgs := []message.Message{{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "hi"}}}}
    ch := p.StreamResponse(ctx, msgs, nil)

    var events []ProviderEvent
    for ev := range ch {
        events = append(events, ev)
    }

    if len(events) != 1 {
        t.Fatalf("expected 1 event, got %d", len(events))
    }
    if events[0].Type != EventComplete {
        t.Fatalf("expected EventComplete, got %v", events[0].Type)
    }
    if events[0].Response == nil || events[0].Response.Content != "hello" {
        t.Fatalf("unexpected response: %+v", events[0].Response)
    }
    if !fc.sendCalled || fc.streamCalled {
        t.Fatalf("expected sendCalled=true and streamCalled=false, got send=%v stream=%v", fc.sendCalled, fc.streamCalled)
    }
}

// Test that errors are surfaced as a single EventError when disableStreaming is true.
func TestBaseProvider_StreamResponse_FallbackError(t *testing.T) {
    t.Parallel()

    fc := &fakeClient{err: errors.New("boom")}
    p := &baseProvider[*fakeClient]{
        options: providerClientOptions{disableStreaming: true},
        client:  fc,
    }

    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    t.Cleanup(cancel)

    msgs := []message.Message{{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "hi"}}}}
    ch := p.StreamResponse(ctx, msgs, nil)

    var events []ProviderEvent
    for ev := range ch {
        events = append(events, ev)
    }

    if len(events) != 1 {
        t.Fatalf("expected 1 event, got %d", len(events))
    }
    if events[0].Type != EventError {
        t.Fatalf("expected EventError, got %v", events[0].Type)
    }
    if events[0].Error == nil || events[0].Error.Error() != "boom" {
        t.Fatalf("unexpected error: %v", events[0].Error)
    }
    if !fc.sendCalled || fc.streamCalled {
        t.Fatalf("expected sendCalled=true and streamCalled=false, got send=%v stream=%v", fc.sendCalled, fc.streamCalled)
    }
}

// Test that when disableStreaming is false, we pass through to the client's stream.
func TestBaseProvider_StreamResponse_PassThrough(t *testing.T) {
    t.Parallel()

    fc := &fakeClient{streamEvents: []ProviderEvent{{Type: EventContentDelta, Content: "x"}}}
    p := &baseProvider[*fakeClient]{
        options: providerClientOptions{disableStreaming: false},
        client:  fc,
    }

    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    t.Cleanup(cancel)

    msgs := []message.Message{{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "hi"}}}}
    ch := p.StreamResponse(ctx, msgs, nil)

    var events []ProviderEvent
    for ev := range ch {
        events = append(events, ev)
    }

    if len(events) != 1 || events[0].Type != EventContentDelta || events[0].Content != "x" {
        t.Fatalf("unexpected streamed events: %+v", events)
    }
    if !fc.streamCalled || fc.sendCalled {
        t.Fatalf("expected streamCalled=true and sendCalled=false, got send=%v stream=%v", fc.sendCalled, fc.streamCalled)
    }
}
