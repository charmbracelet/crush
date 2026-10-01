package agent

import (
	"context"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeNoteTool struct {
	resp fantasy.ToolResponse
	err  error
	runs int
}

func (f *fakeNoteTool) Info() fantasy.ToolInfo {
	return fantasy.ToolInfo{Name: "fake", Description: "fake tool"}
}

func (f *fakeNoteTool) ProviderOptions() fantasy.ProviderOptions { return nil }

func (f *fakeNoteTool) SetProviderOptions(fantasy.ProviderOptions) {}

func (f *fakeNoteTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	f.runs++
	return f.resp, f.err
}

type fakeNoter struct {
	notes map[string]string
}

func (f *fakeNoter) EscalationNote(toolCallID string) string {
	note := f.notes[toolCallID]
	delete(f.notes, toolCallID)
	return note
}

func TestNotedTool_PrependsEscalationNote(t *testing.T) {
	noter := &fakeNoter{notes: map[string]string{
		"call-1": "this action was escalated and approved by the user; outcome: ESCALATED (not ALLOW)",
	}}
	inner := &fakeNoteTool{resp: fantasy.NewTextResponse("output")}
	wrapped := wrapToolsWithEscalationNotes([]fantasy.AgentTool{inner}, noter)

	resp, err := wrapped[0].Run(context.Background(), fantasy.ToolCall{ID: "call-1", Name: "fake"})
	require.NoError(t, err)
	assert.Equal(t,
		"[auto-mode] this action was escalated and approved by the user; outcome: ESCALATED (not ALLOW)\n\noutput",
		resp.Content)

	// The note is consumed once: a second call on the same ID sees none.
	resp, err = wrapped[0].Run(context.Background(), fantasy.ToolCall{ID: "call-1", Name: "fake"})
	require.NoError(t, err)
	assert.Equal(t, "output", resp.Content)
}

func TestNotedTool_NoNoteLeavesResponseUnchanged(t *testing.T) {
	noter := &fakeNoter{notes: map[string]string{}}
	inner := &fakeNoteTool{resp: fantasy.NewTextResponse("output")}
	wrapped := wrapToolsWithEscalationNotes([]fantasy.AgentTool{inner}, noter)

	resp, err := wrapped[0].Run(context.Background(), fantasy.ToolCall{ID: "call-2", Name: "fake"})
	require.NoError(t, err)
	assert.Equal(t, "output", resp.Content)
}

func TestNotedTool_ErrorResponsesStillCarryNote(t *testing.T) {
	noter := &fakeNoter{notes: map[string]string{
		"call-3": "this action was escalated and approved by the user",
	}}
	errResp := fantasy.NewTextErrorResponse("boom")
	inner := &fakeNoteTool{resp: errResp}
	wrapped := wrapToolsWithEscalationNotes([]fantasy.AgentTool{inner}, noter)

	resp, err := wrapped[0].Run(context.Background(), fantasy.ToolCall{ID: "call-3", Name: "fake"})
	require.NoError(t, err)
	assert.True(t, resp.IsError)
	assert.Equal(t,
		"[auto-mode] this action was escalated and approved by the user\n\nboom",
		resp.Content)
}

func TestNotedTool_RunErrorPassthrough(t *testing.T) {
	noter := &fakeNoter{notes: map[string]string{}}
	innerErr := assert.AnError
	inner := &fakeNoteTool{err: innerErr}
	wrapped := wrapToolsWithEscalationNotes([]fantasy.AgentTool{inner}, noter)

	_, err := wrapped[0].Run(context.Background(), fantasy.ToolCall{ID: "call-4", Name: "fake"})
	assert.ErrorIs(t, err, innerErr)
}

func TestWrapToolsWithEscalationNotes_NilNoter(t *testing.T) {
	inner := &fakeNoteTool{resp: fantasy.NewTextResponse("output")}
	tools := []fantasy.AgentTool{inner}
	assert.Same(t, tools[0], wrapToolsWithEscalationNotes(tools, nil)[0])
}
