package agent

import (
	"context"
	"sync"

	"charm.land/fantasy"
)

// Tool call deduplication operates at three layers because providers
// sometimes emit duplicate tool call IDs within a single stream response,
// and each layer protects a different boundary:
//
//  1. Event gating (start/finish/result): prevents duplicate stream events
//     from producing duplicate DB writes or UI updates.
//  2. Execution coalescing (execute): prevents concurrent tool.Run calls
//     for the same ID from executing the tool twice. Late arrivals block
//     on the first caller's result via a shared done channel.
//  3. Message sanitization (deduplicateToolCallMessages): scrubs persisted
//     message history before sending to the provider, because previously
//     stored duplicates (from older sessions or edge cases) would cause
//     providers to reject the request.
//
// The deduplicator is scoped to a single agentic step (one LLM call + its
// tool executions). It is reset at the start of each PrepareStep so entries
// do not accumulate across the session lifetime.

// toolCallEntry unifies dedup state and execution coalescing for one ID.
type toolCallEntry struct {
	started  bool
	finished bool
	resulted bool
	done     chan struct{} // closed when execution completes; nil until execute claims ownership
	response fantasy.ToolResponse
	err      error
}

type toolCallDeduplicator struct {
	mu      sync.Mutex
	entries map[string]*toolCallEntry
}

func newToolCallDeduplicator() *toolCallDeduplicator {
	return &toolCallDeduplicator{
		entries: make(map[string]*toolCallEntry),
	}
}

// reset clears all entries. Call at the start of each agentic step so the
// deduplicator does not accumulate state across steps or leak memory.
func (d *toolCallDeduplicator) reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	clear(d.entries)
}

// start records that a tool call has begun streaming. Returns false if this
// ID was already seen.
func (d *toolCallDeduplicator) start(id string) bool {
	if id == "" {
		return true
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, exists := d.entries[id]; exists {
		return false
	}
	d.entries[id] = &toolCallEntry{started: true}
	return true
}

// finish records that a tool call's input is complete. Returns false if this
// ID was already finished.
func (d *toolCallDeduplicator) finish(id string) bool {
	if id == "" {
		return true
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	e, exists := d.entries[id]
	if !exists || e.finished {
		return false
	}
	e.finished = true
	return true
}

// result records that a tool result has been received. Returns false if this
// ID already had a result.
func (d *toolCallDeduplicator) result(id string) bool {
	if id == "" {
		return true
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	e, exists := d.entries[id]
	if !exists || e.resulted {
		return false
	}
	e.resulted = true
	return true
}

func (d *toolCallDeduplicator) wrapTools(tools []fantasy.AgentTool) []fantasy.AgentTool {
	wrapped := make([]fantasy.AgentTool, len(tools))
	for i, tool := range tools {
		wrapped[i] = &deduplicatingTool{
			inner:        tool,
			deduplicator: d,
		}
	}
	return wrapped
}

// execute runs the tool, coalescing concurrent calls with the same ID onto a
// single execution. Late arrivals block on the first caller's result.
func (d *toolCallDeduplicator) execute(
	ctx context.Context,
	tool fantasy.AgentTool,
	call fantasy.ToolCall,
) (fantasy.ToolResponse, error) {
	if call.ID == "" {
		return tool.Run(ctx, call)
	}

	d.mu.Lock()
	e, exists := d.entries[call.ID]
	if !exists {
		e = &toolCallEntry{done: make(chan struct{})}
		d.entries[call.ID] = e
	} else if e.done == nil {
		// Entry was created by start() but execute hasn't claimed it yet.
		e.done = make(chan struct{})
		exists = false // we are the first executor
	}
	d.mu.Unlock()

	if exists {
		select {
		case <-e.done:
			return e.response, e.err
		case <-ctx.Done():
			return fantasy.ToolResponse{}, ctx.Err()
		}
	}

	e.response, e.err = tool.Run(ctx, call)
	close(e.done)
	return e.response, e.err
}

type deduplicatingTool struct {
	inner        fantasy.AgentTool
	deduplicator *toolCallDeduplicator
}

func (t *deduplicatingTool) Info() fantasy.ToolInfo {
	return t.inner.Info()
}

func (t *deduplicatingTool) ProviderOptions() fantasy.ProviderOptions {
	return t.inner.ProviderOptions()
}

func (t *deduplicatingTool) SetProviderOptions(opts fantasy.ProviderOptions) {
	t.inner.SetProviderOptions(opts)
}

func (t *deduplicatingTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	return t.deduplicator.execute(ctx, t.inner, call)
}

// deduplicateToolCallMessages removes duplicate tool call and tool result parts
// from a message history, keeping only the first occurrence of each ID.
// Messages that become empty after filtering are dropped entirely, since
// providers reject empty-content assistant and tool messages.
//
// This sanitizes persisted history before sending to the provider. It is
// independent of the stream-level dedup (start/finish/result/execute) which
// prevents duplicates from being stored in the first place.
func deduplicateToolCallMessages(messages []fantasy.Message) []fantasy.Message {
	seenCalls := make(map[string]struct{})
	seenResults := make(map[string]struct{})
	deduplicated := make([]fantasy.Message, 0, len(messages))

	for _, msg := range messages {
		content := make([]fantasy.MessagePart, 0, len(msg.Content))
		for _, part := range msg.Content {
			if call, ok := fantasy.AsMessagePart[fantasy.ToolCallPart](part); ok && call.ToolCallID != "" {
				if _, exists := seenCalls[call.ToolCallID]; exists {
					continue
				}
				seenCalls[call.ToolCallID] = struct{}{}
			}
			if result, ok := fantasy.AsMessagePart[fantasy.ToolResultPart](part); ok && result.ToolCallID != "" {
				if _, exists := seenResults[result.ToolCallID]; exists {
					continue
				}
				seenResults[result.ToolCallID] = struct{}{}
			}
			content = append(content, part)
		}
		// Drop messages whose content was entirely duplicate tool parts.
		// Providers reject empty-content assistant and tool messages.
		if len(msg.Content) > 0 && len(content) == 0 {
			continue
		}
		msg.Content = content
		deduplicated = append(deduplicated, msg)
	}

	return deduplicated
}
