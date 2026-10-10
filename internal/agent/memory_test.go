package agent

import (
	"context"
	"strings"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

// stubMemory is a Memory whose returns the test controls.
type stubMemory struct {
	snapshot string
	recall   string

	userTurns  []string
	agentTurns []string
	toolTurns  []string
}

func (m *stubMemory) Snapshot(context.Context) string       { return m.snapshot }
func (m *stubMemory) Recall(context.Context, string) string { return m.recall }
func (m *stubMemory) RecordUser(t string)                   { m.userTurns = append(m.userTurns, t) }
func (m *stubMemory) RecordAssistant(t string)              { m.agentTurns = append(m.agentTurns, t) }
func (m *stubMemory) RecordTool(t string)                   { m.toolTurns = append(m.toolTurns, t) }
func (m *stubMemory) SummarizeTool(tool, _ string) string   { return "used " + tool }

// assistantMessage builds an assistant turn. Fantasy has no
// constructor for one, since production code builds them from
// streamed parts rather than a literal.
func assistantMessage(text string) fantasy.Message {
	return fantasy.Message{
		Role:    fantasy.MessageRoleAssistant,
		Content: []fantasy.MessagePart{fantasy.TextPart{Text: text}},
	}
}

// messageText concatenates the text parts of a message.
func messageText(m fantasy.Message) string {
	var b strings.Builder
	for _, part := range m.Content {
		if tp, ok := part.(fantasy.TextPart); ok {
			b.WriteString(tp.Text)
		}
	}
	return b.String()
}

// markedIndexes returns the positions carrying cache control.
func markedIndexes(msgs []fantasy.Message) []int {
	var out []int
	for i, m := range msgs {
		if m.ProviderOptions != nil {
			out = append(out, i)
		}
	}
	return out
}

// conversation builds a system message followed by alternating user
// and assistant turns.
func conversation(turns int) []fantasy.Message {
	msgs := []fantasy.Message{fantasy.NewSystemMessage("system")}
	for i := range turns {
		msgs = append(msgs, fantasy.NewUserMessage("user"))
		if i < turns-1 {
			msgs = append(msgs, assistantMessage("assistant"))
		}
	}
	return msgs
}

func TestMarkCacheBreakpointsWithoutMemory(t *testing.T) {
	t.Parallel()

	msgs := conversation(3)
	opts := fantasy.ProviderOptions{"test": nil}
	markCacheBreakpoints(msgs, opts, false)

	// The system message plus the last two messages, and nothing
	// more: providers cap a request at four breakpoints and the tool
	// list already claims one.
	require.Equal(t, []int{0, len(msgs) - 2, len(msgs) - 1}, markedIndexes(msgs))
}

// TestMarkCacheBreakpointsSkipsVolatileTail is the regression guard
// for the whole memory design.
//
// A per-turn memory block changes before it can ever be read back, so
// marking it would spend one of four scarce breakpoints on bytes with
// no chance of a hit, and would displace the marker from a durable
// message the next request actually reuses.
func TestMarkCacheBreakpointsSkipsVolatileTail(t *testing.T) {
	t.Parallel()

	msgs := append(conversation(3), fantasy.NewUserMessage("<system_memory>volatile</system_memory>"))
	opts := fantasy.ProviderOptions{"test": nil}
	markCacheBreakpoints(msgs, opts, true)

	tail := len(msgs) - 1
	marked := markedIndexes(msgs)

	require.NotContains(t, marked, tail, "the volatile block must never be marked")
	// The markers fall on the two most recent durable messages, whose
	// prefix the next turn reuses.
	require.Equal(t, []int{0, tail - 2, tail - 1}, marked)
}

// TestMarkCacheBreakpointsRollingHit checks the property that makes
// tail injection free: the marker set from one turn must still cover
// a prefix of the next turn's request.
func TestMarkCacheBreakpointsRollingHit(t *testing.T) {
	t.Parallel()

	opts := fantasy.ProviderOptions{"test": nil}

	// Turn N: history plus a volatile block.
	turnN := append(conversation(3), fantasy.NewUserMessage("volatile N"))
	markCacheBreakpoints(turnN, opts, true)
	markedN := markedIndexes(turnN)
	deepest := markedN[len(markedN)-1]

	// Turn N+1 keeps the durable history, drops the old volatile
	// block, and appends a fresh one.
	turnNext := conversation(3)
	turnNext = append(turnNext,
		assistantMessage("assistant"),
		fantasy.NewUserMessage("user"),
		fantasy.NewUserMessage("volatile N+1"),
	)

	// Everything up to turn N's deepest marker must be unchanged, so
	// the cached prefix still matches.
	require.Less(t, deepest, len(turnNext))
	for i := range deepest + 1 {
		require.Equal(t, turnN[i].Role, turnNext[i].Role,
			"message %d diverged, so turn N's cache entry cannot be reused", i)
	}
}

func TestMarkCacheBreakpointsEdgeCases(t *testing.T) {
	t.Parallel()

	opts := fantasy.ProviderOptions{"test": nil}

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		require.NotPanics(t, func() { markCacheBreakpoints(nil, opts, false) })
		require.NotPanics(t, func() { markCacheBreakpoints(nil, opts, true) })
	})

	t.Run("only a volatile block", func(t *testing.T) {
		t.Parallel()
		msgs := []fantasy.Message{fantasy.NewUserMessage("volatile")}
		require.NotPanics(t, func() { markCacheBreakpoints(msgs, opts, true) })
		require.Empty(t, markedIndexes(msgs), "nothing durable to mark")
	})

	t.Run("system message only", func(t *testing.T) {
		t.Parallel()
		msgs := []fantasy.Message{fantasy.NewSystemMessage("system")}
		markCacheBreakpoints(msgs, opts, false)
		require.Equal(t, []int{0}, markedIndexes(msgs))
	})
}

func TestWithMemorySnapshot(t *testing.T) {
	t.Parallel()

	t.Run("nil memory leaves the prompt alone", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, "base", withMemorySnapshot(t.Context(), nil, "base"))
	})

	t.Run("empty snapshot leaves the prompt alone", func(t *testing.T) {
		t.Parallel()
		got := withMemorySnapshot(t.Context(), &stubMemory{}, "base")
		require.Equal(t, "base", got)
	})

	t.Run("snapshot is appended after the base prompt", func(t *testing.T) {
		t.Parallel()
		got := withMemorySnapshot(t.Context(), &stubMemory{snapshot: "<memory>x</memory>"}, "base")
		require.True(t, strings.HasPrefix(got, "base"),
			"the stable prefix must stay byte-identical so it keeps hitting cache")
		require.Contains(t, got, "<memory>x</memory>")
	})
}

func TestAppendRecall(t *testing.T) {
	t.Parallel()

	base := conversation(2)

	t.Run("nil memory appends nothing", func(t *testing.T) {
		t.Parallel()
		got, added := appendRecall(t.Context(), nil, base, "prompt")
		require.False(t, added)
		require.Len(t, got, len(base))
	})

	t.Run("empty recall appends nothing", func(t *testing.T) {
		t.Parallel()
		got, added := appendRecall(t.Context(), &stubMemory{}, base, "prompt")
		require.False(t, added)
		require.Len(t, got, len(base))
	})

	t.Run("recall lands at the tail", func(t *testing.T) {
		t.Parallel()
		got, added := appendRecall(t.Context(), &stubMemory{recall: "remembered"}, base, "prompt")
		require.True(t, added)
		require.Len(t, got, len(base)+1)

		last := got[len(got)-1]
		require.Equal(t, fantasy.MessageRoleUser, last.Role)
		require.Contains(t, messageText(last), "remembered")
		require.Contains(t, messageText(last), memoryRecallTag)
	})
}

func TestMemoryForAgent(t *testing.T) {
	t.Parallel()

	t.Run("nil stays nil", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, memoryForAgent(nil, false))
		require.Nil(t, memoryForAgent(nil, true))
	})

	t.Run("main agent reads and writes", func(t *testing.T) {
		t.Parallel()
		stub := &stubMemory{snapshot: "snap"}
		mem := memoryForAgent(stub, false)
		mem.RecordUser("hello")
		require.Equal(t, []string{"hello"}, stub.userTurns)
	})

	t.Run("sub-agent reads but does not write", func(t *testing.T) {
		t.Parallel()
		stub := &stubMemory{snapshot: "snap", recall: "rec"}
		mem := memoryForAgent(stub, true)

		mem.RecordUser("scaffolding")
		mem.RecordAssistant("scaffolding")
		mem.RecordTool("scaffolding")
		require.Empty(t, stub.userTurns, "a sub-agent prompt is not something the user said")
		require.Empty(t, stub.agentTurns)
		require.Empty(t, stub.toolTurns)

		// Reads still pass through.
		require.Equal(t, "snap", mem.Snapshot(t.Context()))
		require.Equal(t, "rec", mem.Recall(t.Context(), "q"))
	})
}

func TestMemoryProvider(t *testing.T) {
	t.Parallel()

	t.Run("nil provider means no memory", func(t *testing.T) {
		t.Parallel()
		var p MemoryProvider
		require.Nil(t, p.get(), "an unset provider must behave as no memory")
	})

	t.Run("a provider yielding nil means no memory", func(t *testing.T) {
		t.Parallel()
		p := MemoryProvider(func() Memory { return nil })
		require.Nil(t, p.get())
	})

	t.Run("static memory always yields the same value", func(t *testing.T) {
		t.Parallel()
		stub := &stubMemory{snapshot: "snap"}
		p := StaticMemory(stub)
		require.Same(t, stub, p.get())
		require.Same(t, stub, p.get())
	})

	t.Run("a provider is re-asked, so connecting mid-session works", func(t *testing.T) {
		t.Parallel()
		// This is the whole point of the indirection: a value
		// captured once could never stop being nil, which would make
		// "connect memory" mean "connect memory, then restart".
		var current Memory
		p := MemoryProvider(func() Memory { return current })
		require.Nil(t, p.get())

		stub := &stubMemory{snapshot: "snap"}
		current = stub
		require.Same(t, stub, p.get(), "a backend connected later must be picked up")

		current = nil
		require.Nil(t, p.get(), "a disconnected backend must stop being used")
	})
}

func TestNewSessionAgentAcceptsAConfiguredMemory(t *testing.T) {
	t.Parallel()

	// A Memory is an interface holding a pointer, and the agent's
	// swappable slot has to survive that. Constructing with memory
	// nil exercises none of it, which is how a panic on every
	// memory-enabled launch once passed a green test suite.
	stub := &stubMemory{snapshot: "snap", recall: "rec"}

	require.NotPanics(t, func() {
		agent := NewSessionAgent(SessionAgentOptions{Memory: stub})
		require.Same(t, stub, agent.(*sessionAgent).memory.get())

		// And the slot must still swap once it holds a real value.
		agent.SetMemory(nil)
		require.Nil(t, agent.(*sessionAgent).memory.get())

		agent.SetMemory(stub)
		require.Same(t, stub, agent.(*sessionAgent).memory.get())
	})
}

func TestNewSessionAgentWithoutMemory(t *testing.T) {
	t.Parallel()

	require.NotPanics(t, func() {
		agent := NewSessionAgent(SessionAgentOptions{})
		require.Nil(t, agent.(*sessionAgent).memory.get())
	})
}
