package flowrag

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompletionDetector_Match(t *testing.T) {
	detector := NewCompletionDetector()

	tests := []struct {
		input    string
		expected bool
	}{
		{"ok", true},
		{"OK", true},
		{"好的", true},
		{"没问题", true},
		{"就这样了", true},
		{"搞定", true},
		{"ok，就这样", true},
		{"好的，就这样吧", true},
		{"done", true},
		{"great", true},
		{"帮我写一个函数", false},
		{"这个代码有问题", false},
		{"hello world", false},
		{"", false},
		{"   ", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := detector.IsCompletionPhrase(tt.input)
			assert.Equal(t, tt.expected, result, "input: %q", tt.input)
		})
	}
}

func TestTaskCompleteMarker(t *testing.T) {
	detector := NewCompletionDetector()

	tests := []struct {
		input    string
		expected bool
	}{
		{"task complete", true},
		{"save this workflow for future", true},
		{"remember this workflow", true},
		{"store this flow in rag", true},
		{"save for later use", true},
		{"random text", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := detector.IsTaskCompleteMarker(tt.input)
			assert.Equal(t, tt.expected, result, "input: %q", tt.input)
		})
	}
}

func TestShouldTriggerFlowRAG(t *testing.T) {
	detector := NewCompletionDetector()
	assert.True(t, detector.ShouldTriggerFlowRAG("好的，OK"))
	assert.True(t, detector.ShouldTriggerFlowRAG("task complete"))
	assert.False(t, detector.ShouldTriggerFlowRAG("write a function"))
}

func TestSegmenter_SuccessfulFlow(t *testing.T) {
	messages := []message.Message{
		{
			Role: message.Assistant,
			Parts: []message.ContentPart{
				message.TextContent{Text: "Let me read the file first."},
			},
		},
		{
			Role: message.Assistant,
			Parts: []message.ContentPart{
				message.ToolCall{ID: "tc1", Name: "read", Input: `{"path":"main.go"}`, Finished: true},
			},
		},
		{
			Role: message.Tool,
			Parts: []message.ContentPart{
				message.ToolResult{ToolCallID: "tc1", Name: "read", Content: "package main\nfunc main() {}", IsError: false},
			},
		},
		{
			Role: message.Assistant,
			Parts: []message.ContentPart{
				message.TextContent{Text: "Now I'll write the fix."},
			},
		},
		{
			Role: message.Assistant,
			Parts: []message.ContentPart{
				message.ToolCall{ID: "tc2", Name: "write", Input: `{"path":"main.go","content":"fixed"}`, Finished: true},
			},
		},
		{
			Role: message.Tool,
			Parts: []message.ContentPart{
				message.ToolResult{ToolCallID: "tc2", Name: "write", Content: "File written successfully", IsError: false},
			},
		},
	}

	segmenter := NewSegmenter()
	workflow := segmenter.Segment("Fix the main function", messages)

	require.NotNil(t, workflow)
	assert.Equal(t, "Fix the main function", workflow.UserPrompt)
	assert.Greater(t, len(workflow.Steps), 0)

	hasWriteSteps := false
	hasReadSteps := false
	for _, step := range workflow.Steps {
		if step.Tool == "write" {
			hasWriteSteps = true
		}
		if step.Tool == "read" {
			hasReadSteps = true
		}
	}
	assert.True(t, hasReadSteps)
	assert.True(t, hasWriteSteps)
}

func TestSegmenter_ExcludeErrorSteps(t *testing.T) {
	messages := []message.Message{
		{
			Role: message.Assistant,
			Parts: []message.ContentPart{
				message.ToolCall{ID: "tc1", Name: "read", Input: `{"path":"nonexistent"}`, Finished: true},
			},
		},
		{
			Role: message.Tool,
			Parts: []message.ContentPart{
				message.ToolResult{ToolCallID: "tc1", Name: "read", Content: "file not found", IsError: true},
			},
		},
		{
			Role: message.Assistant,
			Parts: []message.ContentPart{
				message.ToolCall{ID: "tc2", Name: "read", Input: `{"path":"real.go"}`, Finished: true},
			},
		},
		{
			Role: message.Tool,
			Parts: []message.ContentPart{
				message.ToolResult{ToolCallID: "tc2", Name: "read", Content: "package main", IsError: false},
			},
		},
	}

	segmenter := NewSegmenter()
	workflow := segmenter.Segment("Read a file", messages)

	require.NotNil(t, workflow)
	for _, step := range workflow.Steps {
		if step.Role == "tool_call" && step.Tool == "read" && step.Input == `{"path":"nonexistent"}` {
			t.Error("error tool call should not be in successful workflow")
		}
	}

	hasRealRead := false
	for _, step := range workflow.Steps {
		if step.Role == "tool_result" && step.Tool == "read" && step.Output == "package main" {
			hasRealRead = true
		}
	}
	assert.True(t, hasRealRead, "successful read should be in workflow")
}

func TestWorkflow_ToText(t *testing.T) {
	wf := &Workflow{
		UserPrompt: "Fix the bug",
		Steps: []WorkflowStep{
			{Role: "tool_call", Tool: "read", Input: `{"path":"main.go"}`},
			{Role: "tool_result", Tool: "read", Output: "package main\nfunc main() {}"},
			{Role: "tool_call", Tool: "write", Input: `{"path":"main.go","content":"fixed"}`},
			{Role: "tool_result", Tool: "write", Output: "Written successfully"},
		},
	}
	text := wf.ToText()
	assert.Contains(t, text, "Fix the bug")
	assert.Contains(t, text, "read")
	assert.Contains(t, text, "write")
}

func TestJSONFileStore_InsertAndSearch(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "workflows.json")

	mockClient := &MockEmbeddingClient{Dim: 128}
	store, err := NewFileVectorStore(storePath, mockClient)
	require.NoError(t, err)
	assert.Equal(t, 0, store.Count())

	ctx := context.Background()

	record := WorkflowRecord{
		ID:         "test-1",
		UserPrompt: "Fix the authentication bug",
		StepsText:  "User: Fix the auth bug\nTool Call: read('auth.go')\nTool Result: read -> code here\nTool Call: write('auth.go')\nTool Result: write -> Done",
		Steps: []WorkflowStep{
			{Role: "tool_call", Tool: "read", Input: `{"path":"auth.go"}`},
			{Role: "tool_result", Tool: "read", Output: "code here"},
			{Role: "tool_call", Tool: "write", Input: `{"path":"auth.go"}`},
			{Role: "tool_result", Tool: "write", Output: "Done"},
		},
		SessionID: "session-1",
	}

	err = store.Insert(ctx, record)
	require.NoError(t, err)
	assert.Equal(t, 1, store.Count())

	results, err := store.Search(ctx, "Fix authentication", 5)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "test-1", results[0].ID)

	_ = os.Remove(storePath)
}

func TestJSONFileStore_SearchEmpty(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "workflows.json")

	mockClient := &MockEmbeddingClient{Dim: 128}
	store, err := NewFileVectorStore(storePath, mockClient)
	require.NoError(t, err)

	ctx := context.Background()
	results, err := store.Search(ctx, "some query", 5)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestCosineSimilarity(t *testing.T) {
	a := Vector{1, 0, 0}
	b := Vector{1, 0, 0}
	assert.InDelta(t, 1.0, cosineSimilarity(a, b), 0.001)

	c := Vector{0, 1, 0}
	assert.InDelta(t, 0.0, cosineSimilarity(a, c), 0.001)

	d := Vector{-1, 0, 0}
	assert.InDelta(t, -1.0, cosineSimilarity(a, d), 0.001)

	e := Vector{1, 0}
	f := Vector{1, 0, 0}
	assert.InDelta(t, 0.0, cosineSimilarity(e, f), 0.001)
}

func TestRetriever_BuildContextPrompt(t *testing.T) {
	retriever := &Retriever{}
	records := []WorkflowRecord{
		{
			UserPrompt: "Fix login bug",
			Steps: []WorkflowStep{
				{Role: "tool_call", Tool: "read", Input: `{"path":"login.go"}`},
				{Role: "tool_result", Tool: "read", Output: "code here"},
				{Role: "tool_call", Tool: "edit", Input: `{}`},
				{Role: "tool_result", Tool: "edit", Output: "Done"},
			},
		},
	}

	prompt := retriever.BuildContextPrompt(records)
	assert.Contains(t, prompt, "past_successful_workflows")
	assert.Contains(t, prompt, "Fix login bug")
	assert.Contains(t, prompt, "read")
	assert.Contains(t, prompt, "edit")
}

func TestRetriever_BuildContextPromptEmpty(t *testing.T) {
	retriever := &Retriever{}
	prompt := retriever.BuildContextPrompt(nil)
	assert.Empty(t, prompt)
	prompt = retriever.BuildContextPrompt([]WorkflowRecord{})
	assert.Empty(t, prompt)
}

func TestWorkflowManager_Integration(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "workflows.json")

	mgr, err := NewWorkflowManager(Config{
		StorePath: storePath,
	})
	require.NoError(t, err)

	assert.True(t, mgr.Detector().ShouldTriggerFlowRAG("ok"))
	assert.True(t, mgr.Detector().ShouldTriggerFlowRAG("task complete"))
	assert.True(t, mgr.Detector().ShouldTriggerFlowRAG("好的，就这样"))
	assert.False(t, mgr.Detector().ShouldTriggerFlowRAG("帮我改代码"))

	ctx := context.Background()
	err = mgr.SaveSuccessfulWorkflow(ctx, SaveWorkflowInput{
		UserPrompt: "Add unit tests for handler.go",
		Messages: []message.Message{
			{
				Role: message.Assistant,
				Parts: []message.ContentPart{
					message.ToolCall{ID: "tc1", Name: "read", Input: `{"path":"handler.go"}`, Finished: true},
				},
			},
			{
				Role: message.Tool,
				Parts: []message.ContentPart{
					message.ToolResult{ToolCallID: "tc1", Name: "read", Content: "package main\nfunc handler() {}", IsError: false},
				},
			},
			{
				Role: message.Assistant,
				Parts: []message.ContentPart{
					message.ToolCall{ID: "tc2", Name: "write", Input: `{"path":"handler_test.go","content":"tests"}`, Finished: true},
				},
			},
			{
				Role: message.Tool,
				Parts: []message.ContentPart{
					message.ToolResult{ToolCallID: "tc2", Name: "write", Content: "Written successfully", IsError: false},
				},
			},
		},
		SessionID: "session-1",
	})
	require.NoError(t, err)

	contextPrompt := mgr.SearchAndBuildContext(ctx, "Write tests for handler", 3)
	assert.Contains(t, contextPrompt, "past_successful_workflows")
	assert.Contains(t, contextPrompt, "Add unit tests")
}

func TestTruncate(t *testing.T) {
	assert.Equal(t, "hello", truncate("hello", 10))
	assert.Equal(t, "hello...", truncate("hello world", 5))
	assert.Equal(t, "", truncate("", 10))
}

func TestMustMarshalSteps(t *testing.T) {
	steps := []WorkflowStep{
		{Role: "tool_call", Tool: "read", Input: `{"path":"main.go"}`},
	}
	result := mustMarshalSteps(steps)
	assert.Contains(t, result, "tool_call")
	assert.Contains(t, result, "read")
}
