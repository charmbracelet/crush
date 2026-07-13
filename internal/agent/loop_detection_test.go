package agent

import (
	"fmt"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

// makeStep creates a StepResult with the given tool calls and results in its Content.
func makeStep(calls []fantasy.ToolCallContent, results []fantasy.ToolResultContent) fantasy.StepResult {
	var content fantasy.ResponseContent
	for _, c := range calls {
		content = append(content, c)
	}
	for _, r := range results {
		content = append(content, r)
	}
	return fantasy.StepResult{
		Response: fantasy.Response{
			Content: content,
		},
	}
}

// makeToolStep creates a step with a single tool call and matching text result.
func makeToolStep(name, input, output string) fantasy.StepResult {
	callID := fmt.Sprintf("call_%s_%s", name, input)
	return makeStep(
		[]fantasy.ToolCallContent{
			{ToolCallID: callID, ToolName: name, Input: input},
		},
		[]fantasy.ToolResultContent{
			{ToolCallID: callID, ToolName: name, Result: fantasy.ToolResultOutputContentText{Text: output}},
		},
	)
}

// makeEmptyStep creates a step with no tool calls (e.g. a text-only response).
func makeEmptyStep() fantasy.StepResult {
	return fantasy.StepResult{
		Response: fantasy.Response{
			Content: fantasy.ResponseContent{
				fantasy.TextContent{Text: "thinking..."},
			},
		},
	}
}

func TestHasRepeatedToolCalls(t *testing.T) {
	t.Run("no steps", func(t *testing.T) {
		result := hasRepeatedToolCalls(nil, 10, 5)
		if result {
			t.Error("expected false for empty steps")
		}
	})

	t.Run("fewer steps than window below threshold", func(t *testing.T) {
		steps := make([]fantasy.StepResult, 4)
		for i := range steps {
			steps[i] = makeToolStep("read", `{"file":"a.go"}`, "content")
		}
		result := hasRepeatedToolCalls(steps, 10, 5)
		if result {
			t.Error("expected false when repeats remain below threshold")
		}
	})

	t.Run("loop detected before window fills", func(t *testing.T) {
		steps := make([]fantasy.StepResult, 3)
		for i := range steps {
			steps[i] = makeToolStep("read", `{"file":"a.go"}`, "content")
		}
		if !hasRepeatedToolCalls(steps, 10, 3) {
			t.Error("expected early detection once repeats reach threshold")
		}
	})

	t.Run("all different signatures", func(t *testing.T) {
		steps := make([]fantasy.StepResult, 10)
		for i := range steps {
			steps[i] = makeToolStep("tool", fmt.Sprintf(`{"i":%d}`, i), fmt.Sprintf("result-%d", i))
		}
		result := hasRepeatedToolCalls(steps, 10, 5)
		if result {
			t.Error("expected false when all signatures are different")
		}
	})

	t.Run("exact repeat at threshold detected", func(t *testing.T) {
		steps := make([]fantasy.StepResult, 10)
		for i := range 5 {
			steps[i] = makeToolStep("read", `{"file":"a.go"}`, "content")
		}
		for i := 5; i < 10; i++ {
			steps[i] = makeToolStep("tool", fmt.Sprintf(`{"i":%d}`, i), fmt.Sprintf("result-%d", i))
		}
		result := hasRepeatedToolCalls(steps, 10, 5)
		if !result {
			t.Error("expected true when count equals maxRepeats")
		}
	})

	t.Run("loop detected", func(t *testing.T) {
		// 6 identical steps in a window of 10 with maxRepeats=5 are detected.
		steps := make([]fantasy.StepResult, 10)
		for i := range 6 {
			steps[i] = makeToolStep("read", `{"file":"a.go"}`, "content")
		}
		for i := 6; i < 10; i++ {
			steps[i] = makeToolStep("tool", fmt.Sprintf(`{"i":%d}`, i), fmt.Sprintf("result-%d", i))
		}
		result := hasRepeatedToolCalls(steps, 10, 5)
		if !result {
			t.Error("expected true when same signature reaches maxRepeats")
		}
	})

	t.Run("steps without tool calls are skipped", func(t *testing.T) {
		// Mix of tool steps and empty steps — empty ones should not affect counts
		steps := make([]fantasy.StepResult, 10)
		for i := range 4 {
			steps[i] = makeToolStep("read", `{"file":"a.go"}`, "content")
		}
		for i := 4; i < 8; i++ {
			steps[i] = makeEmptyStep()
		}
		for i := 8; i < 10; i++ {
			steps[i] = makeToolStep("write", `{"file":"b.go"}`, "ok")
		}
		result := hasRepeatedToolCalls(steps, 10, 5)
		if result {
			t.Error("expected false: only 4 repeated tool calls, empty steps should be skipped")
		}
	})

	t.Run("multiple different patterns alternating", func(t *testing.T) {
		// Two patterns alternating: each appears 5 times and reaches the threshold.
		steps := make([]fantasy.StepResult, 10)
		for i := range steps {
			if i%2 == 0 {
				steps[i] = makeToolStep("read", `{"file":"a.go"}`, "content-a")
			} else {
				steps[i] = makeToolStep("write", `{"file":"b.go"}`, "content-b")
			}
		}
		result := hasRepeatedToolCalls(steps, 10, 5)
		if !result {
			t.Error("expected true: both patterns reach the repeat threshold")
		}
	})
}

func TestHasRepeatedFailureClass(t *testing.T) {
	t.Parallel()

	steps := []fantasy.StepResult{
		makeToolStep("bash", `{"command":"npm view package-a"}`, "npm error code E404\nnpm error 404 Not Found"),
		makeToolStep("bash", `{"command":"npm view package-b"}`, "npm error code E404\nnpm error 404 Not Found"),
		makeToolStep("bash", `{"command":"npm view package-c"}`, "npm error code E404\nnpm error 404 Not Found"),
	}
	if !hasRepeatedFailureClass(steps, 10, 3) {
		t.Fatal("expected repeated package-not-found failures to stop the loop")
	}
}

func TestHasRepeatedFailureClassCountsParallelStepOnce(t *testing.T) {
	t.Parallel()

	step := makeStep(
		[]fantasy.ToolCallContent{
			{ToolCallID: "a", ToolName: "bash", Input: `{"command":"a"}`},
			{ToolCallID: "b", ToolName: "bash", Input: `{"command":"b"}`},
			{ToolCallID: "c", ToolName: "bash", Input: `{"command":"c"}`},
		},
		[]fantasy.ToolResultContent{
			{ToolCallID: "a", ToolName: "bash", Result: fantasy.ToolResultOutputContentText{Text: "npm error code E404"}},
			{ToolCallID: "b", ToolName: "bash", Result: fantasy.ToolResultOutputContentText{Text: "npm error code E404"}},
			{ToolCallID: "c", ToolName: "bash", Result: fantasy.ToolResultOutputContentText{Text: "npm error code E404"}},
		},
	)
	if hasRepeatedFailureClass([]fantasy.StepResult{step}, 10, 3) {
		t.Fatal("parallel failures in one model step must count as one attempt")
	}
}

func TestRepeatedExternalFailureClassRequiresResearchAfterTwoSteps(t *testing.T) {
	t.Parallel()

	steps := []fantasy.StepResult{
		makeToolStep("bash", `{"command":"npm view guessed-a"}`, "npm error code E404"),
		makeToolStep("bash", `{"command":"npm view guessed-b"}`, "npm error code E404"),
	}
	if !hasRepeatedExternalFailureClass(steps, 10, 2) {
		t.Fatal("expected two external lookup failures to require research")
	}
	if hasRepeatedExternalFailureClass(steps[:1], 10, 2) {
		t.Fatal("one external lookup failure must not trigger forced research")
	}
}

func TestGeneric404IsMissingResourceNotPackage(t *testing.T) {
	t.Parallel()

	require.Equal(t, "package-not-found", failureClass("npm error code E404"))
	require.Equal(t, "resource-not-found", failureClass("GET https://api.example.test/repos/guessed: 404 Not Found"))
}

func TestRepeatedResourceNotFoundFinalizesAfterTwoSteps(t *testing.T) {
	t.Parallel()

	steps := []fantasy.StepResult{
		makeToolStep("remote_read", `{"resource":"guessed-a"}`, "GET https://api.example.test/guessed-a: 404 Not Found"),
		makeToolStep("remote_read", `{"resource":"guessed-b"}`, "GET https://api.example.test/guessed-b: status code: 404"),
	}
	require.True(t, hasRepeatedResourceNotFound(steps, 10, 2))
	require.False(t, hasRepeatedResourceNotFound(steps[:1], 10, 2))
	require.False(t, hasRepeatedExternalFailureClass(steps, 10, 2), "generic missing resources must not force package research")
}

func TestRepeatedExternalSchemaFailureRequiresResearch(t *testing.T) {
	t.Parallel()

	steps := []fantasy.StepResult{
		makeToolStep("mcp_refresh", `{"name":"filesystem"}`, "filesystem: error: unsupported mcp type"),
		makeToolStep("mcp_refresh", `{"name":"memory"}`, "memory: error: unsupported mcp type"),
	}
	if !hasRepeatedExternalFailureClass(steps, 10, 2) {
		t.Fatal("expected repeated MCP schema failures to require research")
	}
}

func TestMCPAddValidationFailureStopsWithoutModelReview(t *testing.T) {
	t.Parallel()

	steps := []fantasy.StepResult{
		makeToolStep("mcp_add", `{"name":"browser","stdio":{"command":"npx","url":"package"}}`, `browser: invalid_config; configuration_changed=false; unknown field "url"`),
	}
	require.True(t, hasMCPAddValidationFailure(steps))
	require.False(t, hasMCPAddValidationFailure([]fantasy.StepResult{
		makeToolStep("mcp_refresh", `{"name":"browser"}`, "browser: connected"),
	}))
}

func TestMCPAddBlockingFailureStopsWithoutSummaryOrContinuation(t *testing.T) {
	t.Parallel()

	steps := []fantasy.StepResult{
		makeToolStep("mcp_add", `{"name":"browser"}`, `browser: start_failed; configuration_changed=false; rollback=restored; error=dependency missing`),
	}
	require.True(t, hasMCPAddBlockingFailure(steps))
	require.False(t, hasMCPAddBlockingFailure([]fantasy.StepResult{
		makeToolStep("mcp_add", `{"name":"browser"}`, `browser: invalid_config; configuration_changed=false`),
	}))
	require.False(t, hasMCPAddBlockingFailure([]fantasy.StepResult{
		makeToolStep("mcp_refresh", `{"name":"browser"}`, "browser: start_failed"),
	}))
}

func TestGetToolInteractionSignature(t *testing.T) {
	t.Run("empty content returns empty string", func(t *testing.T) {
		sig := getToolInteractionSignature(fantasy.ResponseContent{})
		if sig != "" {
			t.Errorf("expected empty string, got %q", sig)
		}
	})

	t.Run("text only content returns empty string", func(t *testing.T) {
		content := fantasy.ResponseContent{
			fantasy.TextContent{Text: "hello"},
		}
		sig := getToolInteractionSignature(content)
		if sig != "" {
			t.Errorf("expected empty string, got %q", sig)
		}
	})

	t.Run("tool call with result produces signature", func(t *testing.T) {
		content := fantasy.ResponseContent{
			fantasy.ToolCallContent{ToolCallID: "1", ToolName: "read", Input: `{"file":"a.go"}`},
			fantasy.ToolResultContent{ToolCallID: "1", ToolName: "read", Result: fantasy.ToolResultOutputContentText{Text: "content"}},
		}
		sig := getToolInteractionSignature(content)
		if sig == "" {
			t.Error("expected non-empty signature")
		}
	})

	t.Run("same interactions produce same signature", func(t *testing.T) {
		content1 := fantasy.ResponseContent{
			fantasy.ToolCallContent{ToolCallID: "1", ToolName: "read", Input: `{"file":"a.go"}`},
			fantasy.ToolResultContent{ToolCallID: "1", ToolName: "read", Result: fantasy.ToolResultOutputContentText{Text: "content"}},
		}
		content2 := fantasy.ResponseContent{
			fantasy.ToolCallContent{ToolCallID: "2", ToolName: "read", Input: `{"file":"a.go"}`},
			fantasy.ToolResultContent{ToolCallID: "2", ToolName: "read", Result: fantasy.ToolResultOutputContentText{Text: "content"}},
		}
		sig1 := getToolInteractionSignature(content1)
		sig2 := getToolInteractionSignature(content2)
		if sig1 != sig2 {
			t.Errorf("expected same signature for same interactions, got %q and %q", sig1, sig2)
		}
	})

	t.Run("different inputs produce different signatures", func(t *testing.T) {
		content1 := fantasy.ResponseContent{
			fantasy.ToolCallContent{ToolCallID: "1", ToolName: "read", Input: `{"file":"a.go"}`},
			fantasy.ToolResultContent{ToolCallID: "1", ToolName: "read", Result: fantasy.ToolResultOutputContentText{Text: "content"}},
		}
		content2 := fantasy.ResponseContent{
			fantasy.ToolCallContent{ToolCallID: "1", ToolName: "read", Input: `{"file":"b.go"}`},
			fantasy.ToolResultContent{ToolCallID: "1", ToolName: "read", Result: fantasy.ToolResultOutputContentText{Text: "content"}},
		}
		sig1 := getToolInteractionSignature(content1)
		sig2 := getToolInteractionSignature(content2)
		if sig1 == sig2 {
			t.Error("expected different signatures for different inputs")
		}
	})
}
