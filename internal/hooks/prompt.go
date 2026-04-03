package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

type promptHandler struct {
	config  HookConfig
	handler PromptHandler
}

func newPromptHandler(cfg HookConfig, ph PromptHandler) (*promptHandler, error) {
	if cfg.Prompt == nil {
		return nil, fmt.Errorf("prompt config is required for prompt handler type")
	}
	if ph == nil {
		return nil, fmt.Errorf("no PromptHandler registered — prompt hooks require an LLM integration")
	}
	return &promptHandler{config: cfg, handler: ph}, nil
}

func (h *promptHandler) Execute(ctx context.Context, input HookInput) (*HookOutput, error) {
	inputJSON, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling hook input: %w", err)
	}

	fullPrompt := fmt.Sprintf("You are a safety verification hook for a coding assistant.\n\n## Hook Context\n%s\n\n## Verification Task\n%s\n\n## Response Format\nRespond with a JSON object:\n- \"decision\": \"allow\" (safe to proceed), \"deny\" (block with reason), or \"modify\" (adjust input)\n- \"reason\": brief explanation of your decision\n- \"modified_input\": (only if decision is \"modify\") the adjusted tool input\n\nRespond ONLY with the JSON object, no other text.", string(inputJSON), h.config.Prompt.Prompt)

	output, err := h.handler.RunPrompt(ctx, fullPrompt, input)
	if err != nil {
		slog.Warn("Prompt hook LLM call failed, allowing (fail-open)",
			"hook", h.config.Name, "error", err)
		return &HookOutput{Decision: DecisionAllow}, nil
	}

	if output != nil {
		output.Decision = Decision(strings.ToLower(string(output.Decision)))
		switch output.Decision {
		case DecisionAllow, DecisionDeny, DecisionModify:
		default:
			slog.Warn("Prompt hook returned unknown decision, allowing",
				"hook", h.config.Name, "decision", output.Decision)
			return &HookOutput{Decision: DecisionAllow}, nil
		}
	}

	return output, nil
}
