package notebook

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

func TestVerificationState(t *testing.T) {
	t.Parallel()

	resultWith := func(meta string) *message.ToolResult {
		return &message.ToolResult{ToolCallID: "tc", Name: "edit", Content: "ok", Metadata: meta}
	}

	tests := []struct {
		name   string
		tool   string
		result *message.ToolResult
		want   string
	}{
		{name: "non-mutation tool carries no state", tool: "view", result: resultWith(""), want: ""},
		{name: "missing result carries no state", tool: "edit", result: nil, want: ""},
		{name: "mutation without metadata is unverified", tool: "edit", result: resultWith(""), want: "unverified"},
		{
			name: "all passed is verified", tool: "edit",
			result: resultWith(`{"verification":[{"check":"diagnostics","state":"passed"}]}`), want: "verified",
		},
		{
			name: "failed beats verified", tool: "write",
			result: resultWith(`{"verification":[{"check":"diagnostics","state":"passed"},{"check":"verify:x","state":"failed"}]}`),
			want:   "failed",
		},
		{
			name: "unverified beats verified", tool: "edit",
			result: resultWith(`{"verification":[{"check":"diagnostics","state":"passed"},{"check":"verify:x","state":"unverified"}]}`),
			want:   "unverified",
		},
		{
			name: "pending maps to unverified", tool: "edit",
			result: resultWith(`{"verification":[{"check":"diagnostics","state":"passed"},{"check":"verify:x","state":"pending"}]}`),
			want:   "unverified",
		},
		{
			name: "failed beats pending", tool: "multiedit",
			result: resultWith(`{"verification":[{"check":"verify:x","state":"pending"},{"check":"verify:y","state":"failed"}]}`),
			want:   "failed",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, verificationState(tc.tool, tc.result))
		})
	}
}

func TestGenerateEntries_PersistsVerifiedAndTag(t *testing.T) {
	t.Parallel()
	svc, _, sessionID := newTestService(t, nil)

	msgs := []message.Message{
		{
			Role: message.Assistant,
			Parts: []message.ContentPart{
				message.ToolCall{ID: "tc1", Name: "edit", Input: `{"file_path":"auth.go"}`, Finished: true},
			},
		},
		{
			Role: message.Tool,
			Parts: []message.ContentPart{
				message.ToolResult{
					ToolCallID: "tc1", Name: "edit", Content: "edited",
					Metadata: `{"verification":[{"check":"verify:build","state":"failed","detail":"exit code 1"}]}`,
				},
			},
		},
	}
	require.NoError(t, svc.GenerateEntries(context.Background(), sessionID, 1, msgs))

	entries, err := svc.GetEntries(context.Background(), sessionID)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "failed", entries[0].Verified)
	require.Contains(t, entries[0].Tags, "verification-failed")
}

func TestGenerateEntries_UnverifiedMutationTagged(t *testing.T) {
	t.Parallel()
	svc, _, sessionID := newTestService(t, nil)

	// A write result with no verification metadata records as
	// unverified and gets the structural tag.
	msgs := []message.Message{
		{
			Role:  message.Assistant,
			Parts: []message.ContentPart{message.ToolCall{ID: "tc1", Name: "write", Input: `{"file_path":"x.go"}`, Finished: true}},
		},
		{
			Role:  message.Tool,
			Parts: []message.ContentPart{message.ToolResult{ToolCallID: "tc1", Name: "write", Content: "wrote"}},
		},
	}
	require.NoError(t, svc.GenerateEntries(context.Background(), sessionID, 1, msgs))

	entries, err := svc.GetEntries(context.Background(), sessionID)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "unverified", entries[0].Verified)
	require.Contains(t, entries[0].Tags, "unverified")
}
