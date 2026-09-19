package agent

import (
	"testing"

	"github.com/charmbracelet/crush/internal/message"
)

func TestRootUserPromptPicksEarliestUserMessage(t *testing.T) {
	// ListUserMessages returns newest first, so the earliest user prompt — the
	// task the session was started with — sits at the end of the slice. An
	// auto-summarize continuation must restate that earliest prompt, not the
	// prompt of the interrupted run (#3867).
	userMessages := []message.Message{
		{Parts: []message.ContentPart{message.TextContent{Text: "Your test stuck forever. Add timeouts."}}},
		{Parts: []message.ContentPart{message.TextContent{Text: "continue"}}},
		{Parts: []message.ContentPart{message.TextContent{Text: "Debug the race condition in the worker pool"}}},
	}

	got := rootUserPrompt(userMessages, "fallback")
	want := "Debug the race condition in the worker pool"
	if got != want {
		t.Fatalf("rootUserPrompt = %q, want %q", got, want)
	}
}

func TestRootUserPromptSkipsBlankMessages(t *testing.T) {
	userMessages := []message.Message{
		{Parts: []message.ContentPart{message.TextContent{Text: "latest"}}},
		{Parts: []message.ContentPart{message.TextContent{Text: "   "}}},
		{Parts: []message.ContentPart{message.TextContent{Text: "original task"}}},
	}

	got := rootUserPrompt(userMessages, "fallback")
	if got != "original task" {
		t.Fatalf("rootUserPrompt = %q, want %q", got, "original task")
	}
}

func TestRootUserPromptFallsBackWhenHistoryIsEmpty(t *testing.T) {
	if got := rootUserPrompt(nil, "fallback"); got != "fallback" {
		t.Fatalf("rootUserPrompt(nil) = %q, want fallback %q", got, "fallback")
	}
	empty := []message.Message{{}}
	if got := rootUserPrompt(empty, "fallback"); got != "fallback" {
		t.Fatalf("rootUserPrompt(no text) = %q, want fallback %q", got, "fallback")
	}
}
