package message

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStreamAccumulatorCheckpoint(t *testing.T) {
	t.Parallel()

	msg := Message{}
	acc := NewStreamAccumulator(&msg)
	acc.AppendReasoning("thinking")
	acc.AppendText("hello")

	require.True(t, acc.Checkpoint(time.Now()))
	require.Equal(t, "hello", msg.Content().Text)
	require.Equal(t, "thinking", msg.ReasoningContent().Thinking)
	require.True(t, acc.Empty())
}

func TestStreamAccumulatorDefersSmallDelta(t *testing.T) {
	t.Parallel()

	msg := Message{}
	acc := NewStreamAccumulator(&msg)
	acc.AppendText("hello")
	require.False(t, acc.CheckpointIfDue(acc.lastCheckpoint))
	require.Empty(t, msg.Parts)
	require.True(t, acc.CheckpointIfDue(acc.lastCheckpoint.Add(acc.interval)))
}

func TestStreamAccumulatorReset(t *testing.T) {
	t.Parallel()

	msg := Message{}
	acc := NewStreamAccumulator(&msg)
	acc.AppendText("partial")
	acc.AppendToolArguments("tool", "{\"x\":")
	acc.Reset()
	require.False(t, acc.Checkpoint(time.Now()))
	require.Empty(t, acc.ToolArguments("tool"))
}

func BenchmarkStreamTextNaive_10000Deltas(b *testing.B) {
	for b.Loop() {
		msg := Message{}
		for range 10_000 {
			msg.AppendContent("token ")
		}
	}
}

func BenchmarkStreamText_10000Deltas(b *testing.B) {
	for b.Loop() {
		msg := Message{}
		acc := NewStreamAccumulator(&msg)
		for range 10_000 {
			acc.AppendText("token ")
		}
		acc.Checkpoint(time.Now())
	}
}

func BenchmarkStreamReasoning_10000Deltas(b *testing.B) {
	for b.Loop() {
		msg := Message{}
		acc := NewStreamAccumulator(&msg)
		for range 10_000 {
			acc.AppendReasoning("thought ")
		}
		acc.Checkpoint(time.Now())
	}
}

func BenchmarkToolArguments_100000Fragments(b *testing.B) {
	for b.Loop() {
		msg := Message{}
		acc := NewStreamAccumulator(&msg)
		for range 100_000 {
			acc.AppendToolArguments("call", "x")
		}
		if got := acc.ToolArguments("call"); len(got) != 100_000 || !strings.HasPrefix(got, "x") {
			b.Fatal("unexpected buffered tool arguments")
		}
	}
}
