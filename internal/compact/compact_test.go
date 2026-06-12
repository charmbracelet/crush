package compact

import (
	"slices"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

func TestNormalizeUserMessage(t *testing.T) {
	t.Parallel()
	msgs := []message.Message{
		{
			Role: message.User,
			Parts: []message.ContentPart{
				message.TextContent{Text: "Fix the login bug"},
			},
		},
	}
	blocks := Normalize(msgs)
	require.Len(t, blocks, 1)
	require.Equal(t, BlockUser, blocks[0].Kind)
	require.Equal(t, "Fix the login bug", blocks[0].Text)
}

func TestNormalizeAssistantWithToolCall(t *testing.T) {
	t.Parallel()
	msgs := []message.Message{
		{
			Role: message.Assistant,
			Parts: []message.ContentPart{
				message.TextContent{Text: "Let me look at that file."},
				message.ToolCall{
					ID:       "tc1",
					Name:     "view",
					Input:    `{"file_path": "src/auth.go"}`,
					Finished: true,
				},
			},
		},
	}
	blocks := Normalize(msgs)
	require.Len(t, blocks, 2)
	require.Equal(t, BlockAssistant, blocks[0].Kind)
	require.Equal(t, "Let me look at that file.", blocks[0].Text)
	require.Equal(t, BlockToolCall, blocks[1].Kind)
	require.Equal(t, "view", blocks[1].Name)
	require.Equal(t, "src/auth.go", blocks[1].Args["file_path"])
}

func TestNormalizeToolResult(t *testing.T) {
	t.Parallel()
	msgs := []message.Message{
		{
			Role: message.Tool,
			Parts: []message.ContentPart{
				message.ToolResult{
					ToolCallID: "tc1",
					Name:       "view",
					Content:    "package main\n\nfunc Main() {}",
				},
			},
		},
	}
	blocks := Normalize(msgs)
	require.Len(t, blocks, 1)
	require.Equal(t, BlockToolResult, blocks[0].Kind)
	require.Equal(t, "view", blocks[0].Name)
	require.Contains(t, blocks[0].ResultText, "func Main()")
}

func TestFilterNoiseRemovesThinkingAndNoiseTools(t *testing.T) {
	t.Parallel()
	blocks := []NormalizedBlock{
		{Kind: BlockUser, Text: "Do something"},
		{Kind: BlockToolCall, Name: "todos"},
		{Kind: BlockToolResult, Name: "todos"},
		{Kind: BlockToolCall, Name: "edit", Args: map[string]string{"file_path": "x.go"}},
		{Kind: BlockUser, Text: "<system-reminder>context info</system-reminder>"},
	}
	filtered := FilterNoise(blocks)
	require.Len(t, filtered, 2)
	require.Equal(t, BlockUser, filtered[0].Kind)
	require.Equal(t, BlockToolCall, filtered[1].Kind)
	require.Equal(t, "edit", filtered[1].Name)
}

func TestExtractGoals(t *testing.T) {
	t.Parallel()
	blocks := []NormalizedBlock{
		{Kind: BlockUser, Text: "Fix the authentication bug in login flow"},
		{Kind: BlockAssistant, Text: "I'll look into it."},
		{Kind: BlockUser, Text: "Actually, also update the session token refresh logic"},
	}
	goals := ExtractGoals(blocks)
	require.NotEmpty(t, goals)
	require.Contains(t, goals[0], "Fix the authentication bug")
	// Should detect scope change.
	found := slices.Contains(goals, "[Scope change]")
	require.True(t, found, "expected [Scope change] marker")
}

func TestExtractGoalsRejectsNoise(t *testing.T) {
	t.Parallel()
	blocks := []NormalizedBlock{
		{Kind: BlockUser, Text: "ok"},
		{Kind: BlockUser, Text: "yes"},
		{Kind: BlockUser, Text: "thanks"},
	}
	goals := ExtractGoals(blocks)
	require.Empty(t, goals)
}

func TestExtractFileAndSymbols(t *testing.T) {
	t.Parallel()
	blocks := []NormalizedBlock{
		{
			Kind: BlockToolCall,
			Name: "edit",
			Args: map[string]string{
				"file_path":  "internal/auth/session.go",
				"new_string": "func RefreshToken(token string) (*Session, error) {\n\treturn nil, nil\n}",
			},
		},
		{
			Kind:       BlockToolResult,
			Name:       "edit",
			ResultText: "File edited successfully.",
		},
		{
			Kind: BlockToolCall,
			Name: "view",
			Args: map[string]string{"file_path": "internal/types.go"},
		},
		{
			Kind:       BlockToolResult,
			Name:       "view",
			ResultText: "package types\n\ntype User struct {\n\tID string\n}\n\nfunc NewUser(id string) *User {\n\treturn &User{ID: id}\n}",
		},
	}
	result := ExtractFileAndSymbols(blocks)
	require.Contains(t, result.FileActivity.Modified, "internal/auth/session.go")
	require.Contains(t, result.FileActivity.Read, "internal/types.go")
	// Should have extracted Go symbols.
	require.NotEmpty(t, result.SymbolChanges)
}

func TestExtractOutstandingContext(t *testing.T) {
	t.Parallel()
	blocks := []NormalizedBlock{
		{
			Kind:     BlockBash,
			Command:  "go test ./...",
			Output:   "FAIL\ngithub.com/foo/bar 0.5s",
			ExitCode: 1,
		},
		{
			Kind:       BlockToolResult,
			Name:       "grep",
			ResultText: "No matches found.",
		},
	}
	items := ExtractOutstandingContext(blocks)
	require.NotEmpty(t, items)
	// Should contain bash error and no-matches.
	hasError := false
	hasNoMatch := false
	for _, item := range items {
		if contains(item, "[bash:exit 1]") {
			hasError = true
		}
		if contains(item, "[no matches]") {
			hasNoMatch = true
		}
	}
	require.True(t, hasError, "expected bash error item")
	require.True(t, hasNoMatch, "expected no-matches item")
}

func TestExtractCommits(t *testing.T) {
	t.Parallel()
	blocks := []NormalizedBlock{
		{
			Kind:   BlockBash,
			Output: "abc1234: fix(auth): refresh token after password reset\ndef5678: chore: update deps",
		},
	}
	commits := ExtractCommits(blocks)
	require.Len(t, commits, 2)
	require.Contains(t, commits[0], "abc1234")
	require.Contains(t, commits[1], "def5678")
}

func TestBriefTranscript(t *testing.T) {
	t.Parallel()
	blocks := []NormalizedBlock{
		{Kind: BlockUser, Text: "Fix the login bug"},
		{Kind: BlockAssistant, Text: "Let me check the auth code."},
		{Kind: BlockToolCall, Name: "view", Args: map[string]string{"file_path": "src/auth.go"}},
		{Kind: BlockToolResult, Name: "view", ResultText: "package auth"},
		{Kind: BlockToolCall, Name: "edit", Args: map[string]string{"file_path": "src/auth.go"}},
	}
	sections := BuildBriefSections(blocks)
	require.NotEmpty(t, sections)
	text := StringifyBrief(sections)
	require.Contains(t, text, "[user]")
	require.Contains(t, text, "[assistant]")
	require.Contains(t, text, "* view")
}

func TestIdentifyTurns(t *testing.T) {
	t.Parallel()
	blocks := []NormalizedBlock{
		{Kind: BlockUser, Text: "Fix the login bug"},
		{Kind: BlockToolCall, Name: "edit", Args: map[string]string{"file_path": "src/auth.go"}},
		{Kind: BlockUser, Text: "Also add tests"},
		{Kind: BlockToolCall, Name: "write", Args: map[string]string{"file_path": "src/auth_test.go"}},
	}
	turns := IdentifyTurns(blocks)
	require.Len(t, turns, 2)
	require.Contains(t, turns[0].Summary, "Fix the login bug")
	require.Contains(t, turns[1].Summary, "Also add tests")
}

func TestFormatSummary(t *testing.T) {
	t.Parallel()
	data := SectionData{
		SessionGoal:     []string{"Fix the login bug"},
		FilesAndChanges: []string{"Modified: src/auth.go"},
		BriefTranscript: "[user]\nFix the login bug",
	}
	result := FormatSummary(data)
	require.Contains(t, result, "[Session Goal]")
	require.Contains(t, result, "[Files And Changes]")
	// Stable sections should come before volatile.
	goalIdx := indexOf(result, "[Session Goal]")
	filesIdx := indexOf(result, "[Files And Changes]")
	require.Less(t, goalIdx, filesIdx)
}

func TestMergePrevious(t *testing.T) {
	t.Parallel()
	prev := "[Session Goal]\n- Fix the login bug\n\n[Files And Changes]\n- Modified: src/auth.go"
	fresh := "[Session Goal]\n- Add tests for auth\n\n[Files And Changes]\n- Modified: src/auth_test.go"
	merged := MergePrevious(prev, fresh)
	// Both goals should be present (sticky).
	require.Contains(t, merged, "Fix the login bug")
	require.Contains(t, merged, "Add tests for auth")
	// Both files should be present (union).
	require.Contains(t, merged, "src/auth.go")
	require.Contains(t, merged, "src/auth_test.go")
}

func TestCompactEndToEnd(t *testing.T) {
	t.Parallel()
	msgs := []message.Message{
		{
			Role:  message.User,
			Parts: []message.ContentPart{message.TextContent{Text: "Fix the login bug in the auth module"}},
		},
		{
			Role: message.Assistant,
			Parts: []message.ContentPart{
				message.TextContent{Text: "Let me look at the auth code."},
				message.ToolCall{ID: "tc1", Name: "view", Input: `{"file_path": "src/auth.go"}`, Finished: true},
			},
		},
		{
			Role: message.Tool,
			Parts: []message.ContentPart{
				message.ToolResult{ToolCallID: "tc1", Name: "view", Content: "package auth\n\nfunc Login(user string) error {\n\treturn nil\n}"},
			},
		},
		{
			Role: message.Assistant,
			Parts: []message.ContentPart{
				message.ToolCall{ID: "tc2", Name: "edit", Input: `{"file_path": "src/auth.go", "new_string": "func Login(user string) (*Session, error) {\n\treturn nil, nil\n}"}`, Finished: true},
			},
		},
		{
			Role: message.Tool,
			Parts: []message.ContentPart{
				message.ToolResult{ToolCallID: "tc2", Name: "edit", Content: "File edited successfully."},
			},
		},
	}
	result := Compact(Input{Messages: msgs})
	require.NotEmpty(t, result)
	require.Contains(t, result, "This summary captures work done before")
	require.Contains(t, result, "[Session Goal]")
	require.Contains(t, result, "Fix the login bug")
}

func TestNormalizeBashCommandCorrelation(t *testing.T) {
	t.Parallel()
	msgs := []message.Message{
		{
			Role: message.Assistant,
			Parts: []message.ContentPart{
				message.ToolCall{
					ID:       "tc1",
					Name:     "bash",
					Input:    `{"command": "go test ./..."}`,
					Finished: true,
				},
			},
		},
		{
			Role: message.Tool,
			Parts: []message.ContentPart{
				message.ToolResult{
					ToolCallID: "tc1",
					Name:       "bash",
					Content:    "PASS\nok github.com/foo/bar 0.5s",
					Metadata:   `{"exit_code": 0}`,
				},
			},
		},
	}
	blocks := Normalize(msgs)
	// Should have ToolCall + Bash blocks.
	var bashBlock *NormalizedBlock
	for i := range blocks {
		if blocks[i].Kind == BlockBash {
			bashBlock = &blocks[i]
			break
		}
	}
	require.NotNil(t, bashBlock, "expected a BlockBash")
	require.Equal(t, "go test ./...", bashBlock.Command, "bash command should be correlated from preceding ToolCall")
	require.Contains(t, bashBlock.Output, "PASS")
}

func TestClip(t *testing.T) {
	t.Parallel()
	require.Equal(t, "hello", Clip("hello", 10))
	require.Equal(t, "hello", Clip("hello world foo bar", 8))
	require.Equal(t, "hello world foo", Clip("hello world foo bar baz qux", 16))
}

func TestFirstLine(t *testing.T) {
	t.Parallel()
	require.Equal(t, "first line", FirstLine("first line\nsecond line", 200))
	require.Equal(t, "first", FirstLine("first line\nsecond", 5))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
