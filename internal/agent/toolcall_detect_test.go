package agent

import (
	"context"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

func testResolver() *toolNameResolver {
	return newToolNameResolver(fantasy.Call{
		Tools: []fantasy.Tool{
			fantasy.FunctionTool{Name: "view", Description: "Reads and displays file contents"},
			fantasy.FunctionTool{Name: "edit", Description: "Edits files by replacing text"},
			fantasy.FunctionTool{Name: "grep", Description: "Fast content search tool"},
			fantasy.FunctionTool{Name: "bash", Description: "Executes bash commands"},
		},
	})
}

func TestParseToolCallsFromText(t *testing.T) {
	t.Parallel()
	resolver := testResolver()

	tests := []struct {
		name      string
		text      string
		wantCount int
		wantNames []string
	}{
		{
			name:      "single tool call",
			text:      `{"name":"view","arguments":{"file_path":"./main.go"}}`,
			wantCount: 1,
			wantNames: []string{"view"},
		},
		{
			name:      "array of tool calls",
			text:      `[{"name":"view","arguments":{"file_path":"a.go"}},{"name":"grep","arguments":{"pattern":"foo"}}]`,
			wantCount: 2,
			wantNames: []string{"view", "grep"},
		},
		{
			name:      "with whitespace",
			text:      `  {"name":"edit","arguments":{"file_path":"x.go","content":"hello"}}  `,
			wantCount: 1,
			wantNames: []string{"edit"},
		},
		{
			name:      "stringified arguments",
			text:      `{"name":"bash","arguments":"{\"command\":\"ls -la\"}"}`,
			wantCount: 1,
			wantNames: []string{"bash"},
		},
		{
			name:      "empty arguments",
			text:      `{"name":"view","arguments":{}}`,
			wantCount: 1,
			wantNames: []string{"view"},
		},
		{
			name:      "normal text",
			text:      "Here is the file content you asked for.",
			wantCount: 0,
		},
		{
			name:      "empty string",
			text:      "",
			wantCount: 0,
		},
		{
			name:      "whitespace only",
			text:      "   ",
			wantCount: 0,
		},
		{
			name:      "completely unknown tool name",
			text:      `{"name":"unknown_tool","arguments":{}}`,
			wantCount: 0,
		},
		{
			name:      "array with one unknown rejects all",
			text:      `[{"name":"view","arguments":{}},{"name":"unknown","arguments":{}}]`,
			wantCount: 0,
		},
		{
			name:      "invalid JSON starting with brace",
			text:      `{"name": this is not json`,
			wantCount: 0,
		},
		{
			name:      "valid JSON but missing name field",
			text:      `{"tool":"view","arguments":{}}`,
			wantCount: 0,
		},
		{
			name:      "valid JSON but empty name",
			text:      `{"name":"","arguments":{}}`,
			wantCount: 0,
		},
		{
			name:      "JSON object not a tool call",
			text:      `{"key":"value","other":123}`,
			wantCount: 0,
		},
		{
			name:      "empty array",
			text:      `[]`,
			wantCount: 0,
		},
		{
			name:      "null arguments",
			text:      `{"name":"view","arguments":null}`,
			wantCount: 1,
			wantNames: []string{"view"},
		},
		{
			name:      "json code fence",
			text:      "```json\n{\"name\":\"view\",\"arguments\":{\"file_path\":\"./go.mod\"}}\n```",
			wantCount: 1,
			wantNames: []string{"view"},
		},
		{
			name:      "plain code fence",
			text:      "```\n{\"name\":\"bash\",\"arguments\":{\"command\":\"ls\"}}\n```",
			wantCount: 1,
			wantNames: []string{"bash"},
		},
		{
			name:      "code fence with extra whitespace",
			text:      "  ```json\n  {\"name\":\"edit\",\"arguments\":{\"file_path\":\"x.go\"}}  \n```  ",
			wantCount: 1,
			wantNames: []string{"edit"},
		},
		{
			name:      "code fence with unknown tool",
			text:      "```json\n{\"name\":\"unknown\",\"arguments\":{}}\n```",
			wantCount: 0,
		},
		{
			name:      "code fence with array",
			text:      "```json\n[{\"name\":\"view\",\"arguments\":{}},{\"name\":\"grep\",\"arguments\":{}}]\n```",
			wantCount: 2,
			wantNames: []string{"view", "grep"},
		},
		{
			name:      "capitalized tool name",
			text:      `{"name":"View","arguments":{"file_path":"main.go"}}`,
			wantCount: 1,
			wantNames: []string{"view"},
		},
		{
			name:      "uppercase tool name",
			text:      `{"name":"BASH","arguments":{"command":"ls"}}`,
			wantCount: 1,
			wantNames: []string{"bash"},
		},
		{
			name:      "code fence with capitalized name",
			text:      "```json\n{\"name\":\"View\",\"arguments\":{\"file_path\":\"go.mod\"}}\n```",
			wantCount: 1,
			wantNames: []string{"view"},
		},
		{
			name:      "description alias Read -> view",
			text:      `{"name":"Read","arguments":{"file_path":"./go.mod"}}`,
			wantCount: 1,
			wantNames: []string{"view"},
		},
		{
			name:      "description alias Reads -> view",
			text:      `{"name":"Reads","arguments":{"file_path":"./go.mod"}}`,
			wantCount: 1,
			wantNames: []string{"view"},
		},
		{
			name:      "description alias Execute -> bash",
			text:      `{"name":"Execute","arguments":{"command":"ls"}}`,
			wantCount: 1,
			wantNames: []string{"bash"},
		},
		{
			name:      "description alias in code fence",
			text:      "```json\n{\"name\":\"Read\",\"arguments\":{\"file_path\":\"main.go\"}}\n```",
			wantCount: 1,
			wantNames: []string{"view"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := parseToolCallsFromText(tt.text, resolver)
			require.Len(t, result, tt.wantCount)
			for i, name := range tt.wantNames {
				require.Equal(t, name, result[i].name)
				require.NotEmpty(t, result[i].id)
			}
		})
	}
}

func TestStripCodeFence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "json fence",
			text: "```json\n{\"key\":\"value\"}\n```",
			want: `{"key":"value"}`,
		},
		{
			name: "plain fence",
			text: "```\n{\"key\":\"value\"}\n```",
			want: `{"key":"value"}`,
		},
		{
			name: "JSON uppercase fence",
			text: "```JSON\n{\"key\":\"value\"}\n```",
			want: `{"key":"value"}`,
		},
		{
			name: "no fence",
			text: `{"key":"value"}`,
			want: `{"key":"value"}`,
		},
		{
			name: "fence with whitespace",
			text: "  ```json\n  {\"key\":\"value\"}  \n```  ",
			want: `{"key":"value"}`,
		},
		{
			name: "not a fence",
			text: "Hello world",
			want: "Hello world",
		},
		{
			name: "partial fence no closing",
			text: "```json\n{\"key\":\"value\"}",
			want: "```json\n{\"key\":\"value\"}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := stripCodeFence(tt.text)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestToolNameResolver(t *testing.T) {
	t.Parallel()
	resolver := testResolver()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "exact match", input: "view", want: "view"},
		{name: "capitalized", input: "View", want: "view"},
		{name: "uppercase", input: "BASH", want: "bash"},
		{name: "description alias Read", input: "Read", want: "view"},
		{name: "description alias Reads", input: "Reads", want: "view"},
		{name: "description alias Execute", input: "Execute", want: "bash"},
		{name: "description alias Executes", input: "Executes", want: "bash"},
		{name: "description alias Edit", input: "Edit", want: "edit"},
		{name: "unknown", input: "Unknown", want: ""},
		{name: "empty", input: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := resolver.resolve(tt.input)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestToolNameResolver_DescriptionCollision(t *testing.T) {
	t.Parallel()

	resolver := newToolNameResolver(fantasy.Call{
		Tools: []fantasy.Tool{
			fantasy.FunctionTool{Name: "grep", Description: "Fast content search"},
			fantasy.FunctionTool{Name: "glob", Description: "Fast file matching"},
		},
	})

	require.Equal(t, "", resolver.resolve("Fast"))
	require.Equal(t, "grep", resolver.resolve("grep"))
	require.Equal(t, "glob", resolver.resolve("glob"))
}

func TestNormalizeArguments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "object arguments",
			raw:  `{"file_path":"main.go"}`,
			want: `{"file_path":"main.go"}`,
		},
		{
			name: "stringified JSON",
			raw:  `"{\"file_path\":\"main.go\"}"`,
			want: `{"file_path":"main.go"}`,
		},
		{
			name: "empty",
			raw:  ``,
			want: `{}`,
		},
		{
			name: "null",
			raw:  `null`,
			want: `null`,
		},
		{
			name: "string that is not JSON",
			raw:  `"not json"`,
			want: `"not json"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := normalizeArguments([]byte(tt.raw))
			require.Equal(t, tt.want, got)
		})
	}
}

func TestNormalizeVerb(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"Reads", "read"},
		{"Read", "read"},
		{"read", "read"},
		{"Executes", "execute"},
		{"Execute", "execute"},
		{"Creates", "create"},
		{"BASH", "bash"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := normalizeVerb(tt.input)
			require.Equal(t, tt.want, got)
		})
	}
}

type fakeLanguageModel struct {
	generateResp *fantasy.Response
	streamParts  []fantasy.StreamPart
}

func (f *fakeLanguageModel) Provider() string { return "openai-compat" }
func (f *fakeLanguageModel) Model() string    { return "test-model" }

func (f *fakeLanguageModel) Generate(_ context.Context, _ fantasy.Call) (*fantasy.Response, error) {
	return f.generateResp, nil
}

func (f *fakeLanguageModel) Stream(_ context.Context, _ fantasy.Call) (fantasy.StreamResponse, error) {
	parts := f.streamParts
	return func(yield func(fantasy.StreamPart) bool) {
		for _, p := range parts {
			if !yield(p) {
				return
			}
		}
	}, nil
}

func (f *fakeLanguageModel) GenerateObject(_ context.Context, _ fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, nil
}

func (f *fakeLanguageModel) StreamObject(_ context.Context, _ fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, nil
}

func TestToolCallDetectingModel_Generate(t *testing.T) {
	t.Parallel()

	call := fantasy.Call{
		Tools: []fantasy.Tool{
			fantasy.FunctionTool{Name: "view", Description: "Reads and displays file contents"},
		},
	}

	t.Run("converts text tool call", func(t *testing.T) {
		t.Parallel()
		inner := &fakeLanguageModel{
			generateResp: &fantasy.Response{
				Content:      fantasy.ResponseContent{fantasy.TextContent{Text: `{"name":"view","arguments":{"file_path":"main.go"}}`}},
				FinishReason: fantasy.FinishReasonStop,
			},
		}
		model := newToolCallDetectingModel(inner)
		resp, err := model.Generate(context.Background(), call)
		require.NoError(t, err)
		require.Equal(t, fantasy.FinishReasonToolCalls, resp.FinishReason)
		require.Len(t, resp.Content.ToolCalls(), 1)
		require.Equal(t, "view", resp.Content.ToolCalls()[0].ToolName)
		require.Empty(t, resp.Content.Text())
	})

	t.Run("converts code-fenced tool call", func(t *testing.T) {
		t.Parallel()
		inner := &fakeLanguageModel{
			generateResp: &fantasy.Response{
				Content:      fantasy.ResponseContent{fantasy.TextContent{Text: "```json\n{\"name\":\"view\",\"arguments\":{\"file_path\":\"main.go\"}}\n```"}},
				FinishReason: fantasy.FinishReasonStop,
			},
		}
		model := newToolCallDetectingModel(inner)
		resp, err := model.Generate(context.Background(), call)
		require.NoError(t, err)
		require.Equal(t, fantasy.FinishReasonToolCalls, resp.FinishReason)
		require.Len(t, resp.Content.ToolCalls(), 1)
		require.Equal(t, "view", resp.Content.ToolCalls()[0].ToolName)
	})

	t.Run("converts description-alias tool name", func(t *testing.T) {
		t.Parallel()
		inner := &fakeLanguageModel{
			generateResp: &fantasy.Response{
				Content:      fantasy.ResponseContent{fantasy.TextContent{Text: "```json\n{\"name\":\"Read\",\"arguments\":{\"file_path\":\"go.mod\"}}\n```"}},
				FinishReason: fantasy.FinishReasonStop,
			},
		}
		model := newToolCallDetectingModel(inner)
		resp, err := model.Generate(context.Background(), call)
		require.NoError(t, err)
		require.Equal(t, fantasy.FinishReasonToolCalls, resp.FinishReason)
		require.Len(t, resp.Content.ToolCalls(), 1)
		require.Equal(t, "view", resp.Content.ToolCalls()[0].ToolName)
		require.Contains(t, resp.Content.ToolCalls()[0].Input, "go.mod")
	})

	t.Run("passes through normal text", func(t *testing.T) {
		t.Parallel()
		inner := &fakeLanguageModel{
			generateResp: &fantasy.Response{
				Content:      fantasy.ResponseContent{fantasy.TextContent{Text: "Hello, world!"}},
				FinishReason: fantasy.FinishReasonStop,
			},
		}
		model := newToolCallDetectingModel(inner)
		resp, err := model.Generate(context.Background(), call)
		require.NoError(t, err)
		require.Equal(t, fantasy.FinishReasonStop, resp.FinishReason)
		require.Equal(t, "Hello, world!", resp.Content.Text())
		require.Empty(t, resp.Content.ToolCalls())
	})

	t.Run("passes through existing tool calls", func(t *testing.T) {
		t.Parallel()
		inner := &fakeLanguageModel{
			generateResp: &fantasy.Response{
				Content: fantasy.ResponseContent{
					fantasy.ToolCallContent{ToolCallID: "1", ToolName: "view", Input: "{}"},
				},
				FinishReason: fantasy.FinishReasonToolCalls,
			},
		}
		model := newToolCallDetectingModel(inner)
		resp, err := model.Generate(context.Background(), call)
		require.NoError(t, err)
		require.Equal(t, fantasy.FinishReasonToolCalls, resp.FinishReason)
		require.Len(t, resp.Content.ToolCalls(), 1)
	})
}

func TestToolCallDetectingModel_Stream(t *testing.T) {
	t.Parallel()

	call := fantasy.Call{
		Tools: []fantasy.Tool{
			fantasy.FunctionTool{Name: "view", Description: "Reads and displays file contents"},
			fantasy.FunctionTool{Name: "grep", Description: "Fast content search"},
		},
	}

	t.Run("converts streamed text tool call", func(t *testing.T) {
		t.Parallel()
		inner := &fakeLanguageModel{
			streamParts: []fantasy.StreamPart{
				{Type: fantasy.StreamPartTypeTextStart, ID: "0"},
				{Type: fantasy.StreamPartTypeTextDelta, ID: "0", Delta: `{"name":"view",`},
				{Type: fantasy.StreamPartTypeTextDelta, ID: "0", Delta: `"arguments":{"file_path":"main.go"}}`},
				{Type: fantasy.StreamPartTypeTextEnd, ID: "0"},
				{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop},
			},
		}
		model := newToolCallDetectingModel(inner)
		stream, err := model.Stream(context.Background(), call)
		require.NoError(t, err)

		var parts []fantasy.StreamPart
		for part := range stream {
			parts = append(parts, part)
		}

		require.Len(t, parts, 5)
		require.Equal(t, fantasy.StreamPartTypeToolInputStart, parts[0].Type)
		require.Equal(t, "view", parts[0].ToolCallName)
		require.Equal(t, fantasy.StreamPartTypeToolInputDelta, parts[1].Type)
		require.Equal(t, fantasy.StreamPartTypeToolInputEnd, parts[2].Type)
		require.Equal(t, fantasy.StreamPartTypeToolCall, parts[3].Type)
		require.Equal(t, "view", parts[3].ToolCallName)
		require.Contains(t, parts[3].ToolCallInput, "main.go")
		require.Equal(t, fantasy.StreamPartTypeFinish, parts[4].Type)
		require.Equal(t, fantasy.FinishReasonToolCalls, parts[4].FinishReason)
	})

	t.Run("converts code-fenced streamed tool call", func(t *testing.T) {
		t.Parallel()
		inner := &fakeLanguageModel{
			streamParts: []fantasy.StreamPart{
				{Type: fantasy.StreamPartTypeTextStart, ID: "0"},
				{Type: fantasy.StreamPartTypeTextDelta, ID: "0", Delta: "```json\n"},
				{Type: fantasy.StreamPartTypeTextDelta, ID: "0", Delta: `{"name":"view",`},
				{Type: fantasy.StreamPartTypeTextDelta, ID: "0", Delta: `"arguments":{"file_path":"main.go"}}`},
				{Type: fantasy.StreamPartTypeTextDelta, ID: "0", Delta: "\n```"},
				{Type: fantasy.StreamPartTypeTextEnd, ID: "0"},
				{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop},
			},
		}
		model := newToolCallDetectingModel(inner)
		stream, err := model.Stream(context.Background(), call)
		require.NoError(t, err)

		var parts []fantasy.StreamPart
		for part := range stream {
			parts = append(parts, part)
		}

		require.Len(t, parts, 5)
		require.Equal(t, fantasy.StreamPartTypeToolInputStart, parts[0].Type)
		require.Equal(t, "view", parts[0].ToolCallName)
		require.Equal(t, fantasy.StreamPartTypeToolCall, parts[3].Type)
		require.Contains(t, parts[3].ToolCallInput, "main.go")
		require.Equal(t, fantasy.FinishReasonToolCalls, parts[4].FinishReason)
	})

	t.Run("converts description-alias in stream", func(t *testing.T) {
		t.Parallel()
		inner := &fakeLanguageModel{
			streamParts: []fantasy.StreamPart{
				{Type: fantasy.StreamPartTypeTextStart, ID: "0"},
				{Type: fantasy.StreamPartTypeTextDelta, ID: "0", Delta: "```json\n{\"name\":\"Read\","},
				{Type: fantasy.StreamPartTypeTextDelta, ID: "0", Delta: "\"arguments\":{\"file_path\":\"go.mod\"}}"},
				{Type: fantasy.StreamPartTypeTextDelta, ID: "0", Delta: "\n```"},
				{Type: fantasy.StreamPartTypeTextEnd, ID: "0"},
				{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop},
			},
		}
		model := newToolCallDetectingModel(inner)
		stream, err := model.Stream(context.Background(), call)
		require.NoError(t, err)

		var parts []fantasy.StreamPart
		for part := range stream {
			parts = append(parts, part)
		}

		require.Len(t, parts, 5)
		require.Equal(t, fantasy.StreamPartTypeToolInputStart, parts[0].Type)
		require.Equal(t, "view", parts[0].ToolCallName)
		require.Equal(t, fantasy.StreamPartTypeToolCall, parts[3].Type)
		require.Equal(t, "view", parts[3].ToolCallName)
		require.Equal(t, fantasy.FinishReasonToolCalls, parts[4].FinishReason)
	})

	t.Run("passes through normal streamed text", func(t *testing.T) {
		t.Parallel()
		inner := &fakeLanguageModel{
			streamParts: []fantasy.StreamPart{
				{Type: fantasy.StreamPartTypeTextStart, ID: "0"},
				{Type: fantasy.StreamPartTypeTextDelta, ID: "0", Delta: "Hello, "},
				{Type: fantasy.StreamPartTypeTextDelta, ID: "0", Delta: "world!"},
				{Type: fantasy.StreamPartTypeTextEnd, ID: "0"},
				{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop},
			},
		}
		model := newToolCallDetectingModel(inner)
		stream, err := model.Stream(context.Background(), call)
		require.NoError(t, err)

		var parts []fantasy.StreamPart
		for part := range stream {
			parts = append(parts, part)
		}

		require.Len(t, parts, 5)
		require.Equal(t, fantasy.StreamPartTypeTextStart, parts[0].Type)
		require.Equal(t, fantasy.StreamPartTypeTextDelta, parts[1].Type)
		require.Equal(t, "Hello, ", parts[1].Delta)
		require.Equal(t, fantasy.StreamPartTypeTextDelta, parts[2].Type)
		require.Equal(t, "world!", parts[2].Delta)
		require.Equal(t, fantasy.StreamPartTypeTextEnd, parts[3].Type)
		require.Equal(t, fantasy.FinishReasonStop, parts[4].FinishReason)
	})

	t.Run("passes through native tool calls", func(t *testing.T) {
		t.Parallel()
		inner := &fakeLanguageModel{
			streamParts: []fantasy.StreamPart{
				{Type: fantasy.StreamPartTypeToolInputStart, ID: "call_1", ToolCallName: "view"},
				{Type: fantasy.StreamPartTypeToolInputDelta, ID: "call_1", Delta: `{"file_path":"x"}`},
				{Type: fantasy.StreamPartTypeToolInputEnd, ID: "call_1"},
				{Type: fantasy.StreamPartTypeToolCall, ID: "call_1", ToolCallName: "view", ToolCallInput: `{"file_path":"x"}`},
				{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls},
			},
		}
		model := newToolCallDetectingModel(inner)
		stream, err := model.Stream(context.Background(), call)
		require.NoError(t, err)

		var parts []fantasy.StreamPart
		for part := range stream {
			parts = append(parts, part)
		}

		require.Len(t, parts, 5)
		require.Equal(t, fantasy.StreamPartTypeToolInputStart, parts[0].Type)
		require.Equal(t, fantasy.StreamPartTypeFinish, parts[4].Type)
		require.Equal(t, fantasy.FinishReasonToolCalls, parts[4].FinishReason)
	})

	t.Run("no tools in call skips wrapping", func(t *testing.T) {
		t.Parallel()
		inner := &fakeLanguageModel{
			streamParts: []fantasy.StreamPart{
				{Type: fantasy.StreamPartTypeTextStart, ID: "0"},
				{Type: fantasy.StreamPartTypeTextDelta, ID: "0", Delta: `{"name":"view"}`},
				{Type: fantasy.StreamPartTypeTextEnd, ID: "0"},
				{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop},
			},
		}
		model := newToolCallDetectingModel(inner)
		stream, err := model.Stream(context.Background(), fantasy.Call{})
		require.NoError(t, err)

		var parts []fantasy.StreamPart
		for part := range stream {
			parts = append(parts, part)
		}

		require.Len(t, parts, 4)
		require.Equal(t, fantasy.StreamPartTypeTextStart, parts[0].Type)
	})
}
