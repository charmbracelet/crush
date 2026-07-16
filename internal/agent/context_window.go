package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"charm.land/fantasy"
)

type contextWindowModel struct {
	fantasy.LanguageModel
	contextWindow int64
}

// contextWindowModel enforces Crush's provider-independent admission policy at
// the final LanguageModel boundary. The count is deliberately conservative,
// not a claim of provider-exact tokenization; the output reserve covers normal
// tokenizer and serialization differences while the wrapper prevents known
// oversized calls from reaching any provider implementation.

type contextWindowExceededError struct {
	inputTokens   int64
	outputReserve int64
	contextWindow int64
}

func (e *contextWindowExceededError) Error() string {
	return fmt.Sprintf(
		"model request exceeds context window: estimated %d input tokens plus %d reserved output tokens exceeds the %d token limit",
		e.inputTokens,
		e.outputReserve,
		e.contextWindow,
	)
}

func withContextWindowLimit(model fantasy.LanguageModel, contextWindow int64) fantasy.LanguageModel {
	if model == nil || contextWindow <= 0 {
		return model
	}
	return &contextWindowModel{
		LanguageModel: model,
		contextWindow: contextWindow,
	}
}

func (m *contextWindowModel) Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
	if err := validateContextWindow(call, m.contextWindow); err != nil {
		return nil, err
	}
	return m.LanguageModel.Generate(ctx, call)
}

func (m *contextWindowModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	if err := validateContextWindow(call, m.contextWindow); err != nil {
		return nil, err
	}
	return m.LanguageModel.Stream(ctx, call)
}

func validateContextWindow(call fantasy.Call, contextWindow int64) error {
	inputTokens, err := estimateCallInputTokens(call)
	if err != nil {
		return fmt.Errorf("estimate model request size: %w", err)
	}

	reserve := contextWindowOutputReserve(contextWindow, call.MaxOutputTokens)
	if inputTokens+reserve <= contextWindow {
		return nil
	}

	return &contextWindowExceededError{
		inputTokens:   inputTokens,
		outputReserve: reserve,
		contextWindow: contextWindow,
	}
}

func contextWindowOutputReserve(contextWindow int64, maxOutputTokens *int64) int64 {
	reserve := contextWindowReserve(contextWindow)
	if maxOutputTokens != nil {
		reserve = max(reserve, *maxOutputTokens)
	}
	return reserve
}

// resolveMaxOutputTokens applies one cross-provider policy: a call-specific
// limit wins, then the model default, then the context reserve. The final
// fallback turns every known-context request into a bounded generation so
// admission and provider execution share the same maximum total size. An
// unknown-context model with no configured limit preserves provider behavior.
func resolveMaxOutputTokens(contextWindow, defaultMaxTokens, requested int64) *int64 {
	value := requested
	if value <= 0 {
		value = defaultMaxTokens
	}
	if value <= 0 && contextWindow > 0 {
		value = contextWindowReserve(contextWindow)
	}
	if value <= 0 {
		return nil
	}
	return &value
}

func estimateNextStepInputTokens(currentMessages []fantasy.Message, step fantasy.StepResult, tools []fantasy.AgentTool) int64 {
	tokens := estimateMessageTokens(currentMessages) + estimateMessageTokens(step.Messages)
	toolsJSON, err := json.Marshal(agentToolInfo(tools))
	if err == nil {
		tokens += approxTokenCount(string(toolsJSON))
	}
	return tokens
}

func estimateInitialCallInputTokens(
	systemPrompt string,
	systemPromptPrefix string,
	history []fantasy.Message,
	prompt string,
	files []fantasy.FilePart,
	tools []fantasy.AgentTool,
) int64 {
	messages := make([]fantasy.Message, 0, len(history)+3)
	if systemPromptPrefix != "" {
		messages = append(messages, fantasy.NewSystemMessage(systemPromptPrefix))
	}
	if systemPrompt != "" {
		messages = append(messages, fantasy.NewSystemMessage(systemPrompt))
	}
	messages = append(messages, history...)
	messages = append(messages, fantasy.NewUserMessage(prompt, files...))
	return estimateNextStepInputTokens(messages, fantasy.StepResult{}, tools)
}

func agentToolInfo(tools []fantasy.AgentTool) []fantasy.ToolInfo {
	info := make([]fantasy.ToolInfo, len(tools))
	for i, tool := range tools {
		info[i] = tool.Info()
	}
	return info
}

func estimateCallInputTokens(call fantasy.Call) (int64, error) {
	tokens := estimateMessageTokens(call.Prompt)
	if len(call.Tools) == 0 {
		return tokens, nil
	}

	toolsJSON, err := json.Marshal(call.Tools)
	if err != nil {
		return 0, err
	}
	return tokens + approxTokenCount(string(toolsJSON)), nil
}

func contextWindowReserve(contextWindow int64) int64 {
	if contextWindow > largeContextWindowThreshold {
		return largeContextWindowBuffer
	}
	return int64(float64(contextWindow) * smallContextWindowRatio)
}
