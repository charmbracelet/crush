package agent

import (
	"context"

	"charm.land/fantasy"
)

// escalationNoter consumes the one-shot note the permission service records
// when the user approves a request that auto mode escalated.
// permission.Service satisfies it.
type escalationNoter interface {
	EscalationNote(toolCallID string) string
}

// notedTool wraps a fantasy.AgentTool so the model is told when auto mode
// escalated a call and the user approved it. The permission service records
// a one-shot note at approval time; consuming it here — at the tool-response
// level — means the note reaches both the model's in-flight conversation
// (which fantasy builds from ToolResponse values, not from persistence
// callbacks) and the persisted tool result.
type notedTool struct {
	inner fantasy.AgentTool
	noter escalationNoter
}

// wrapToolsWithEscalationNotes returns tools whose responses are prefixed
// with the escalation note recorded for their tool call, when one exists.
// Applied unconditionally: escalation notes are independent of hook
// configuration and apply to sub-agents as well.
func wrapToolsWithEscalationNotes(tools []fantasy.AgentTool, noter escalationNoter) []fantasy.AgentTool {
	if noter == nil {
		return tools
	}
	out := make([]fantasy.AgentTool, len(tools))
	for i, tool := range tools {
		out[i] = &notedTool{inner: tool, noter: noter}
	}
	return out
}

func (n *notedTool) Info() fantasy.ToolInfo {
	return n.inner.Info()
}

func (n *notedTool) ProviderOptions() fantasy.ProviderOptions {
	return n.inner.ProviderOptions()
}

func (n *notedTool) SetProviderOptions(opts fantasy.ProviderOptions) {
	n.inner.SetProviderOptions(opts)
}

func (n *notedTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	resp, err := n.inner.Run(ctx, call)
	if err != nil {
		return resp, err
	}
	// Consume the note even for error responses: the bash tool marks any
	// non-zero exit as an error, and an escalated command frequently
	// fails for unrelated reasons — the model still needs to know the
	// human approved it before it ran.
	note := n.noter.EscalationNote(call.ID)
	if note == "" {
		return resp, nil
	}
	resp.Content = "[auto-mode] " + note + "\n\n" + resp.Content
	return resp, nil
}
