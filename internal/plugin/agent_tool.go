package plugin

import (
	"context"
	"encoding/json"
	"fmt"

	"charm.land/fantasy"
)

// sessionIDContextKey is the context key for session ID.
// This mirrors the key in internal/agent/tools to avoid import cycles.
type sessionIDContextKey string

const sessionIDKey sessionIDContextKey = "session_id"

// getSessionID extracts the session ID from the context.
func getSessionID(ctx context.Context) string {
	if v := ctx.Value(sessionIDKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

type toolWrapper struct {
	inner fantasy.AgentTool
}

func WrapAgentTool(tool fantasy.AgentTool) fantasy.AgentTool {
	if tool == nil {
		return nil
	}
	if wrapped, ok := tool.(*toolWrapper); ok {
		return wrapped
	}
	return &toolWrapper{inner: tool}
}

func (w *toolWrapper) Info() fantasy.ToolInfo {
	return w.inner.Info()
}

func (w *toolWrapper) ProviderOptions() fantasy.ProviderOptions {
	return w.inner.ProviderOptions()
}

func (w *toolWrapper) SetProviderOptions(opts fantasy.ProviderOptions) {
	w.inner.SetProviderOptions(opts)
}

func (w *toolWrapper) Run(ctx context.Context, params fantasy.ToolCall) (fantasy.ToolResponse, error) {
	args := decodeToolArgs(params.Input)
	sessionID := getSessionID(ctx)
	beforeOut, err := TriggerToolBeforeExecute(ctx, ToolBeforeExecuteInput{
		Tool:      w.Info().Name,
		CallID:    params.ID,
		Args:      args,
		SessionID: sessionID,
	})
	if err != nil {
		return fantasy.ToolResponse{}, err
	}
	if beforeOut != nil {
		if beforeOut.Skip {
			return fantasy.NewTextResponse(beforeOut.PreResult), nil
		}
		if beforeOut.Args != nil {
			args = beforeOut.Args
			encoded, marshalErr := json.Marshal(beforeOut.Args)
			if marshalErr != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to encode tool args: %w", marshalErr)
			}
			params.Input = string(encoded)
		}
	}

	response, err := w.inner.Run(ctx, params)
	if err != nil {
		return response, err
	}

	metadata := decodeMetadata(response.Metadata)
	afterOut, err := TriggerToolAfterExecute(ctx, ToolAfterExecuteInput{
		Tool:      w.Info().Name,
		CallID:    params.ID,
		Args:      args,
		Result:    response.Content,
		Metadata:  metadata,
		SessionID: sessionID,
	})
	if err != nil {
		return fantasy.ToolResponse{}, err
	}
	if afterOut != nil {
		if afterOut.ResultChanged {
			response.Content = afterOut.Result
		}
		if afterOut.Metadata != nil {
			encodedMetadata, marshalErr := json.Marshal(afterOut.Metadata)
			if marshalErr != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to encode tool metadata: %w", marshalErr)
			}
			response.Metadata = string(encodedMetadata)
		}
	}

	return response, nil
}

func decodeToolArgs(input string) map[string]any {
	if input == "" {
		return map[string]any{}
	}
	parsed := make(map[string]any)
	if err := json.Unmarshal([]byte(input), &parsed); err != nil {
		return map[string]any{}
	}
	return parsed
}

func decodeMetadata(metadata string) map[string]any {
	if metadata == "" {
		return map[string]any{}
	}
	parsed := make(map[string]any)
	if err := json.Unmarshal([]byte(metadata), &parsed); err != nil {
		return map[string]any{}
	}
	return parsed
}

type customAgentTool struct {
	info            fantasy.ToolInfo
	def             ToolDefinition
	providerOptions fantasy.ProviderOptions
	defaultDir      string
}

func NewCustomToolAgentTool(def ToolDefinition, workingDir string) fantasy.AgentTool {
	return &customAgentTool{
		info: fantasy.ToolInfo{
			Name:        def.Name,
			Description: def.Description,
			Parameters:  normalizeToolParameters(def.Parameters),
		},
		def:        def,
		defaultDir: workingDir,
	}
}

func normalizeToolParameters(parameters any) map[string]any {
	if parameters == nil {
		return map[string]any{}
	}
	if parsed, ok := parameters.(map[string]any); ok {
		return parsed
	}
	encoded, err := json.Marshal(parameters)
	if err != nil {
		return map[string]any{}
	}
	decoded := make(map[string]any)
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return map[string]any{}
	}
	return decoded
}

func (t *customAgentTool) Info() fantasy.ToolInfo {
	return t.info
}

func (t *customAgentTool) ProviderOptions() fantasy.ProviderOptions {
	return t.providerOptions
}

func (t *customAgentTool) SetProviderOptions(opts fantasy.ProviderOptions) {
	t.providerOptions = opts
}

func (t *customAgentTool) Run(ctx context.Context, params fantasy.ToolCall) (fantasy.ToolResponse, error) {
	args := decodeToolArgs(params.Input)
	result, err := t.def.Execute(ctx, args, ToolContext{Directory: t.defaultDir})
	if err != nil {
		return fantasy.NewTextErrorResponse(err.Error()), nil
	}
	return fantasy.NewTextResponse(result), nil
}
