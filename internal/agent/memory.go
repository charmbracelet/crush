package agent

import (
	"context"
	"sync"

	"charm.land/fantasy"
)

// Memory supplies cross-session recall. It is satisfied by
// *honcho.Service, but the agent package depends on the behaviour
// rather than the implementation so the loop stays testable without a
// memory backend and so a future backend needs no changes here.
//
// Every method must tolerate being called on a nil implementation:
// "no memory configured" is the common case, not an error case.
type Memory interface {
	// Snapshot returns the session-stable memory block. It must
	// return the same text for the whole session so the value can be
	// sealed into the system prompt without invalidating the cached
	// prefix on every turn.
	Snapshot(ctx context.Context) string

	// Recall returns a volatile block relevant to the current prompt,
	// or the empty string when there is nothing new worth injecting.
	Recall(ctx context.Context, prompt string) string

	// RecordUser and RecordAssistant persist a turn. Both must return
	// immediately; a memory backend must never sit on the agent loop.
	RecordUser(text string)
	RecordAssistant(text string)

	// RecordTool persists a one-line summary of a tool call, so
	// memory reflects what was done and not only what was said.
	RecordTool(summary string)

	// SummarizeTool renders a tool call as that one-line summary, or
	// returns the empty string when the call is not worth recording.
	// It lives on the interface because deciding what is memorable,
	// and redacting credentials while doing so, is the memory
	// backend's judgement rather than the agent loop's.
	SummarizeTool(tool string, input string) string
}

// MemoryProvider yields the memory backend in force right now, or nil
// when none is configured.
//
// Memory is reached through a function rather than held as a value
// because the user can connect a backend mid-session from the command
// palette. A value captured at construction can never stop being nil,
// which would make "connect" mean "connect after you restart". A
// provider is re-asked on every rebuild, so the tool-refresh path a
// model change already takes is enough to pick one up.
type MemoryProvider func() Memory

// StaticMemory adapts a fixed value to a provider, for callers that
// wire memory once and never change it.
func StaticMemory(m Memory) MemoryProvider {
	return func() Memory { return m }
}

// get resolves the provider. It tolerates both a nil provider and a
// provider that yields nil, so a caller needs one check rather than
// two.
func (p MemoryProvider) get() Memory {
	if p == nil {
		return nil
	}
	return p()
}

// memoryHolder guards an agent's memory backend, which is swapped
// when the user connects or disconnects one mid-session.
//
// This is a plain mutex rather than a csync.Value because a Memory is
// an interface holding a pointer, and csync.Value panics on pointer
// types. A nil interface has no pointer to object to, so only an
// agent with memory actually configured ever hit that guard.
type memoryHolder struct {
	mu sync.RWMutex
	m  Memory
}

func newMemoryHolder(m Memory) *memoryHolder {
	return &memoryHolder{m: m}
}

func (h *memoryHolder) get() Memory {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.m
}

func (h *memoryHolder) set(m Memory) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.m = m
}

// memoryRecallTag wraps an injected recall block. It mirrors the
// existing <system_reminder> convention so the model reads it as
// harness-supplied context rather than as something the user typed.
const memoryRecallTag = "system_memory"

// memoryFeatureName is the feature a skill names in its `requires:`
// field to be shown only when memory is configured.
const memoryFeatureName = "honcho"

// memoryFeatures reports the optional features available to skill
// discovery, so a skill documenting memory tools stays out of the
// system prompt when there is no memory backend to use.
func memoryFeatures(mem Memory) []string {
	if mem == nil {
		return nil
	}
	return []string{memoryFeatureName}
}

// liveMemoryFeatures reports the features available right now, for the
// prompt builder to consult each time it renders.
//
// It asks the provider on every call rather than closing over a value,
// so a backend connected mid-session shows up on the next build. Note
// this answers "is there a working backend", which is not the same as
// "did the user enable one": an enabled backend that fails to start
// leaves the config saying yes and this saying no, and this is the one
// the prompt must believe.
func (c *coordinator) liveMemoryFeatures() []string {
	return memoryFeatures(c.memory.get())
}

// memoryForAgent returns the memory a given agent should use.
//
// Sub-agents read memory but never write to it. A sub-agent's prompt
// is scaffolding the coordinator wrote, not something the user said,
// and recording it would teach Honcho that the user speaks in task
// briefs.
func memoryForAgent(mem Memory, isSubAgent bool) Memory {
	if mem == nil {
		return nil
	}
	if !isSubAgent {
		return mem
	}
	return readOnlyMemory{mem}
}

// readOnlyMemory forwards reads and discards writes.
type readOnlyMemory struct{ Memory }

func (readOnlyMemory) RecordUser(string)      {}
func (readOnlyMemory) RecordAssistant(string) {}
func (readOnlyMemory) RecordTool(string)      {}

// memoryTool records a summary of each tool call after it runs.
//
// This is a separate decorator from hookedTool rather than another
// branch inside it: hooks may deny, rewrite, or halt a call, whereas
// memory only ever observes one. Keeping them apart means a memory
// failure can never change what a tool does.
type memoryTool struct {
	fantasy.AgentTool
	memory Memory
}

// wrapToolsWithMemory decorates tools so their calls are remembered.
// Sub-agent tools are left alone: their work is already summarized by
// the result they hand back to the parent.
func wrapToolsWithMemory(agentTools []fantasy.AgentTool, mem Memory, isSubAgent bool) []fantasy.AgentTool {
	if mem == nil || isSubAgent {
		return agentTools
	}
	out := make([]fantasy.AgentTool, len(agentTools))
	for i, t := range agentTools {
		out[i] = &memoryTool{AgentTool: t, memory: mem}
	}
	return out
}

func (m *memoryTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	resp, err := m.AgentTool.Run(ctx, call)
	// A call that failed says nothing durable about the work, and a
	// failed edit did not happen. Record only what took effect.
	if err == nil && !resp.IsError {
		if summary := m.memory.SummarizeTool(call.Name, call.Input); summary != "" {
			m.memory.RecordTool(summary)
		}
	}
	return resp, err
}

// withMemorySnapshot appends the session-stable memory block to a
// system prompt.
//
// This is safe to cache: the snapshot is computed once per session and
// frozen, so the system prefix stays byte-identical across turns and
// keeps hitting its breakpoint. Volatile recall must never come
// through here; it belongs at the tail via appendRecall.
func withMemorySnapshot(ctx context.Context, mem Memory, prompt string) string {
	if mem == nil {
		return prompt
	}
	block := mem.Snapshot(ctx)
	if block == "" {
		return prompt
	}
	return prompt + "\n\n" + block
}

// appendRecall adds a volatile memory message to the tail of the
// conversation and reports whether it did.
//
// Position matters. Anthropic-style prompt caching is strictly
// prefix-based, so text that changes every turn must sit after
// everything worth caching. The tail is the only such place: putting
// this in the system prompt would invalidate every breakpoint in the
// request, re-billing the whole conversation each turn.
//
// The caller must exclude the appended message from cache breakpoints
// — see markCacheBreakpoints.
func appendRecall(ctx context.Context, mem Memory, msgs []fantasy.Message, prompt string) ([]fantasy.Message, bool) {
	if mem == nil {
		return msgs, false
	}
	block := mem.Recall(ctx, prompt)
	if block == "" {
		return msgs, false
	}
	wrapped := "<" + memoryRecallTag + ">\n" + block + "\n</" + memoryRecallTag + ">"
	return append(msgs, fantasy.NewUserMessage(wrapped)), true
}

// markCacheBreakpoints attaches provider cache control to the system
// message and to the last two cacheable messages.
//
// Providers allow only four breakpoints per request and Crush already
// spends one on the tool list, so the remaining three must land on
// bytes that will actually recur. When a volatile memory block has
// been appended, skipping it is what preserves the rolling hit: the
// markers fall on the two most recent durable messages, whose prefix
// the next turn reuses. Marking the memory block instead would spend a
// breakpoint on text that changes before it can ever be read back.
func markCacheBreakpoints(msgs []fantasy.Message, opts fantasy.ProviderOptions, skipTail bool) {
	if len(msgs) == 0 {
		return
	}

	// Cacheable messages are everything except the volatile tail.
	cacheable := len(msgs)
	if skipTail {
		cacheable--
	}

	lastSystemIdx := 0
	systemMarked := false
	for i, msg := range msgs[:max(cacheable, 0)] {
		// Mark the final system message, which closes the static
		// prefix of tools plus instructions.
		if msg.Role == fantasy.MessageRoleSystem {
			lastSystemIdx = i
		} else if !systemMarked {
			msgs[lastSystemIdx].ProviderOptions = opts
			systemMarked = true
		}
		// Mark the last two cacheable messages. The pair rolls
		// forward each turn, so the older of the two is the prefix
		// the next request hits.
		if i > cacheable-3 {
			msgs[i].ProviderOptions = opts
		}
	}
}
