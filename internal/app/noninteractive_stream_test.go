package app

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

// TestNonInteractiveStream_HandleMessage_WritesAssistantText verifies the
// basic streaming path: an assistant message for the correct session is
// written to output.
func TestNonInteractiveStream_HandleMessage_WritesAssistantText(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	s := &nonInteractiveStream{
		sessionID: "S",
		out:       buf,
		read:      map[string]int{},
	}

	n, err := s.handleMessage(message.Message{
		ID:        "m1",
		SessionID: "S",
		Role:      message.Assistant,
		Parts:     []message.ContentPart{message.TextContent{Text: "hello world"}},
	})
	require.NoError(t, err)
	require.Equal(t, len("hello world"), n)
	require.Equal(t, "hello world", buf.String())
	require.True(t, s.printed)
	require.Equal(t, len("hello world"), s.read["m1"])
}

// TestNonInteractiveStream_HandleMessage_IgnoresOtherSessions verifies that
// messages for a different session are silently skipped.
func TestNonInteractiveStream_HandleMessage_IgnoresOtherSessions(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	s := &nonInteractiveStream{
		sessionID: "S",
		out:       buf,
		read:      map[string]int{},
	}

	n, err := s.handleMessage(message.Message{
		ID:        "m1",
		SessionID: "OTHER",
		Role:      message.Assistant,
		Parts:     []message.ContentPart{message.TextContent{Text: "hello"}},
	})
	require.NoError(t, err)
	require.Equal(t, 0, n)
	require.Empty(t, buf.String())
}

// TestNonInteractiveStream_HandleMessage_IgnoresUserMessages verifies that
// user-role messages are silently skipped.
func TestNonInteractiveStream_HandleMessage_IgnoresUserMessages(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	s := &nonInteractiveStream{
		sessionID: "S",
		out:       buf,
		read:      map[string]int{},
	}

	n, err := s.handleMessage(message.Message{
		ID:        "m1",
		SessionID: "S",
		Role:      message.User,
		Parts:     []message.ContentPart{message.TextContent{Text: "hello"}},
	})
	require.NoError(t, err)
	require.Equal(t, 0, n)
	require.Empty(t, buf.String())
}

// TestNonInteractiveStream_HandleMessage_IncrementalStreaming verifies that
// multiple updates to the same message only append the delta (the new
// bytes since the last read).
func TestNonInteractiveStream_HandleMessage_IncrementalStreaming(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	s := &nonInteractiveStream{
		sessionID: "S",
		out:       buf,
		read:      map[string]int{},
	}

	// First chunk: "hello"
	_, err := s.handleMessage(message.Message{
		ID:        "m1",
		SessionID: "S",
		Role:      message.Assistant,
		Parts:     []message.ContentPart{message.TextContent{Text: "hello"}},
	})
	require.NoError(t, err)
	require.Equal(t, "hello", buf.String())

	// Second chunk: full text "hello world" — only " world" should be appended
	_, err = s.handleMessage(message.Message{
		ID:        "m1",
		SessionID: "S",
		Role:      message.Assistant,
		Parts:     []message.ContentPart{message.TextContent{Text: "hello world"}},
	})
	require.NoError(t, err)
	require.Equal(t, "hello world", buf.String())
}

// TestNonInteractiveStream_HandleMessage_LeadingWhitespaceTrimmedOnce
// verifies that leading whitespace is trimmed only on the very first chunk
// (readBytes == 0), not on subsequent deltas.
func TestNonInteractiveStream_HandleMessage_LeadingWhitespaceTrimmedOnce(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	s := &nonInteractiveStream{
		sessionID: "S",
		out:       buf,
		read:      map[string]int{},
	}

	_, err := s.handleMessage(message.Message{
		ID:        "m1",
		SessionID: "S",
		Role:      message.Assistant,
		Parts:     []message.ContentPart{message.TextContent{Text: "  \tactual output"}},
	})
	require.NoError(t, err)
	require.Equal(t, "actual output", buf.String())
}

// TestNonInteractiveStream_HandleMessage_SkipsInitialWhitespaceOnly
// verifies that a whitespace-only message is not printed unless we have
// already printed real content.
func TestNonInteractiveStream_HandleMessage_SkipsInitialWhitespaceOnly(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	s := &nonInteractiveStream{
		sessionID: "S",
		out:       buf,
		read:      map[string]int{},
	}

	_, err := s.handleMessage(message.Message{
		ID:        "m1",
		SessionID: "S",
		Role:      message.Assistant,
		Parts:     []message.ContentPart{message.TextContent{Text: "   "}},
	})
	require.NoError(t, err)
	require.Empty(t, buf.String(), "whitespace-only initial message should not be printed")
	require.False(t, s.printed)
}

// TestNonInteractiveStream_HandleMessage_ContentShrunkError verifies that
// if the message content is shorter than previously read bytes, an error
// is returned.
func TestNonInteractiveStream_HandleMessage_ContentShrunkError(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	s := &nonInteractiveStream{
		sessionID: "S",
		out:       buf,
		read:      map[string]int{"m1": 100},
	}

	_, err := s.handleMessage(message.Message{
		ID:        "m1",
		SessionID: "S",
		Role:      message.Assistant,
		Parts:     []message.ContentPart{message.TextContent{Text: "short"}},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "message content is shorter than read bytes")
}

// ---------------------------------------------------------------------------
// drainMessages regression tests
// ---------------------------------------------------------------------------

// TestNonInteractiveStream_DrainMessages_EmptyChannel verifies that
// drainMessages returns immediately when the channel is empty.
func TestNonInteractiveStream_DrainMessages_EmptyChannel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	broker := pubsub.NewBroker[message.Message]()
	defer broker.Shutdown()

	ch := broker.Subscribe(ctx)

	buf := &bytes.Buffer{}
	s := &nonInteractiveStream{
		sessionID: "S",
		out:       buf,
		read:      map[string]int{},
	}

	// Channel is empty — drain should return nil immediately.
	err := s.drainMessages(ch)
	require.NoError(t, err)
	require.Empty(t, buf.String())
}

// TestNonInteractiveStream_DrainMessages_ConsumesBufferedEvents is the
// REGRESSION TEST for the race condition that caused truncated stdout
// output in `opencode run --format json`.
//
// Before the fix, the RunNonInteractive event loop's select statement
// could pick the done channel before the final messageEvents
// (containing the actual AI text content), causing the loop to exit
// with truncated or completely missing output.
//
// This test simulates the race scenario: message events are buffered
// in the channel BEFORE drainMessages is called (representing the
// state where done fired but messageEvents still has unconsumed
// content). drainMessages must consume all of them.
func TestNonInteractiveStream_DrainMessages_ConsumesBufferedEvents(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	broker := pubsub.NewBroker[message.Message]()
	defer broker.Shutdown()

	ch := broker.Subscribe(ctx)

	// Publish two message events before draining, simulating the race
	// condition where the agent run completes and done fires while
	// messageEvents still has buffered content.
	broker.Publish(pubsub.UpdatedEvent, message.Message{
		ID:        "m1",
		SessionID: "S",
		Role:      message.Assistant,
		Parts:     []message.ContentPart{message.TextContent{Text: "VERDICT: "}},
	})
	broker.Publish(pubsub.UpdatedEvent, message.Message{
		ID:        "m1",
		SessionID: "S",
		Role:      message.Assistant,
		Parts:     []message.ContentPart{message.TextContent{Text: "VERDICT: APPROVED"}},
	})

	// Give the broker time to deliver events to the subscriber channel.
	require.Eventually(t, func() bool {
		return broker.DropCount() == 0
	}, time.Second, 10*time.Millisecond)

	buf := &bytes.Buffer{}
	s := &nonInteractiveStream{
		sessionID: "S",
		out:       buf,
		read:      map[string]int{},
	}

	err := s.drainMessages(ch)
	require.NoError(t, err)
	require.Equal(t, "VERDICT: APPROVED", buf.String(),
		"drainMessages must consume all buffered message events, not just the first")
}

// TestNonInteractiveStream_DrainMessages_MixedEvents verifies that
// drainMessages only writes content from assistant messages for the
// correct session, ignoring messages from other sessions or user role.
func TestNonInteractiveStream_DrainMessages_MixedEvents(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	broker := pubsub.NewBroker[message.Message]()
	defer broker.Shutdown()

	ch := broker.Subscribe(ctx)

	// Publish events for different sessions and roles.
	broker.Publish(pubsub.UpdatedEvent, message.Message{
		ID:        "m1",
		SessionID: "OTHER",
		Role:      message.Assistant,
		Parts:     []message.ContentPart{message.TextContent{Text: "noise"}},
	})
	broker.Publish(pubsub.UpdatedEvent, message.Message{
		ID:        "m2",
		SessionID: "S",
		Role:      message.User,
		Parts:     []message.ContentPart{message.TextContent{Text: "user prompt"}},
	})
	broker.Publish(pubsub.UpdatedEvent, message.Message{
		ID:        "m3",
		SessionID: "S",
		Role:      message.Assistant,
		Parts:     []message.ContentPart{message.TextContent{Text: "actual response"}},
	})

	require.Eventually(t, func() bool {
		return broker.DropCount() == 0
	}, time.Second, 10*time.Millisecond)

	buf := &bytes.Buffer{}
	s := &nonInteractiveStream{
		sessionID: "S",
		out:       buf,
		read:      map[string]int{},
	}

	err := s.drainMessages(ch)
	require.NoError(t, err)
	require.Equal(t, "actual response", buf.String(),
		"drainMessages must only write assistant messages for the correct session")
}

// TestNonInteractiveStream_DrainMessages_WithPartialStream verifies that
// drainMessages correctly resumes from the last read position when some
// content was already streamed before draining begins.
func TestNonInteractiveStream_DrainMessages_WithPartialStream(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	broker := pubsub.NewBroker[message.Message]()
	defer broker.Shutdown()

	ch := broker.Subscribe(ctx)

	buf := &bytes.Buffer{}
	s := &nonInteractiveStream{
		sessionID: "S",
		out:       buf,
		read:      map[string]int{},
	}

	// Simulate: some streaming already happened via handleMessage.
	_, err := s.handleMessage(message.Message{
		ID:        "m1",
		SessionID: "S",
		Role:      message.Assistant,
		Parts:     []message.ContentPart{message.TextContent{Text: "VERDICT: "}},
	})
	require.NoError(t, err)
	require.Equal(t, "VERDICT: ", buf.String())

	// Now the agent finishes. A final update is buffered in the channel.
	broker.Publish(pubsub.UpdatedEvent, message.Message{
		ID:        "m1",
		SessionID: "S",
		Role:      message.Assistant,
		Parts:     []message.ContentPart{message.TextContent{Text: "VERDICT: APPROVED"}},
	})

	require.Eventually(t, func() bool {
		return broker.DropCount() == 0
	}, time.Second, 10*time.Millisecond)

	// drainMessages must only append the unread tail.
	err = s.drainMessages(ch)
	require.NoError(t, err)
	require.Equal(t, "VERDICT: APPROVED", buf.String(),
		"drainMessages must append only the unread tail, not duplicate the prefix")
}
