package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"charm.land/fantasy"
	"github.com/google/uuid"
)

// toolCallDetectingModel wraps a fantasy.LanguageModel to detect tool call
// JSON emitted as plain text content. Some local model servers (e.g. Ollama)
// return tool call JSON in the text content field instead of the structured
// tool_calls field. This wrapper detects that pattern and converts the text
// into proper tool call stream events so the agent loop can execute them.
type toolCallDetectingModel struct {
	inner fantasy.LanguageModel
}

func newToolCallDetectingModel(inner fantasy.LanguageModel) fantasy.LanguageModel {
	return &toolCallDetectingModel{inner: inner}
}

func (m *toolCallDetectingModel) Provider() string { return m.inner.Provider() }
func (m *toolCallDetectingModel) Model() string    { return m.inner.Model() }

func (m *toolCallDetectingModel) GenerateObject(ctx context.Context, call fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return m.inner.GenerateObject(ctx, call)
}

func (m *toolCallDetectingModel) StreamObject(ctx context.Context, call fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return m.inner.StreamObject(ctx, call)
}

// Generate wraps the non-streaming path. If the response is text-only and
// parses as tool call JSON with known tool names, replace the text content
// with proper ToolCallContent entries.
func (m *toolCallDetectingModel) Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
	resp, err := m.inner.Generate(ctx, call)
	if err != nil {
		return resp, err
	}

	resolver := newToolNameResolver(call)
	if resolver == nil {
		return resp, nil
	}

	// Only attempt conversion when the response is pure text (no tool calls).
	if len(resp.Content.ToolCalls()) > 0 {
		return resp, nil
	}

	text := resp.Content.Text()
	parsed := parseToolCallsFromText(text, resolver)
	if len(parsed) == 0 {
		return resp, nil
	}

	slog.Debug("Detected tool call JSON in text content (non-streaming)", "count", len(parsed))

	content := make([]fantasy.Content, 0, len(parsed))
	for _, tc := range parsed {
		content = append(content, fantasy.ToolCallContent{
			ToolCallID: tc.id,
			ToolName:   tc.name,
			Input:      tc.args,
		})
	}
	resp.Content = content
	resp.FinishReason = fantasy.FinishReasonToolCalls
	return resp, nil
}

// Stream wraps the streaming path. It buffers text events and, once text
// ends, checks whether the accumulated text is tool call JSON. If so it
// emits tool call events instead of the buffered text events.
func (m *toolCallDetectingModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	inner, err := m.inner.Stream(ctx, call)
	if err != nil {
		return nil, err
	}

	resolver := newToolNameResolver(call)
	if resolver == nil {
		return inner, nil
	}

	return func(yield func(fantasy.StreamPart) bool) {
		var textBuf strings.Builder
		var buffered []fantasy.StreamPart
		buffering := true
		detectedToolCalls := false

		for part := range inner {
			if !buffering {
				if detectedToolCalls && part.Type == fantasy.StreamPartTypeFinish {
					part.FinishReason = fantasy.FinishReasonToolCalls
				}
				if !yield(part) {
					return
				}
				continue
			}

			switch part.Type {
			case fantasy.StreamPartTypeTextStart, fantasy.StreamPartTypeTextDelta:
				if part.Type == fantasy.StreamPartTypeTextDelta {
					textBuf.WriteString(part.Delta)
				}
				buffered = append(buffered, part)

			case fantasy.StreamPartTypeTextEnd:
				parsed := parseToolCallsFromText(textBuf.String(), resolver)
				if len(parsed) > 0 {
					slog.Debug("Detected tool call JSON in text content (streaming)", "count", len(parsed))
					detectedToolCalls = true
					for _, tc := range parsed {
						if !yield(fantasy.StreamPart{
							Type:         fantasy.StreamPartTypeToolInputStart,
							ID:           tc.id,
							ToolCallName: tc.name,
						}) {
							return
						}
						if !yield(fantasy.StreamPart{
							Type:  fantasy.StreamPartTypeToolInputDelta,
							ID:    tc.id,
							Delta: tc.args,
						}) {
							return
						}
						if !yield(fantasy.StreamPart{
							Type: fantasy.StreamPartTypeToolInputEnd,
							ID:   tc.id,
						}) {
							return
						}
						if !yield(fantasy.StreamPart{
							Type:          fantasy.StreamPartTypeToolCall,
							ID:            tc.id,
							ToolCallName:  tc.name,
							ToolCallInput: tc.args,
						}) {
							return
						}
					}
				} else {
					// Not tool calls — flush buffered text events.
					for _, p := range buffered {
						if !yield(p) {
							return
						}
					}
					if !yield(part) {
						return
					}
				}
				buffered = nil
				buffering = false

			default:
				// Non-text events (reasoning, native tool calls, errors,
				// finish, etc.) pass through immediately and stop buffering.
				for _, p := range buffered {
					if !yield(p) {
						return
					}
				}
				buffered = nil
				buffering = false
				if !yield(part) {
					return
				}
			}
		}

		// If the stream ended while still buffering (no TextEnd received),
		// flush whatever we have.
		for _, p := range buffered {
			if !yield(p) {
				return
			}
		}
	}, nil
}

// parsedToolCall holds a single parsed tool call extracted from text.
type parsedToolCall struct {
	id   string
	name string
	args string
}

// toolCallJSON is the JSON shape local models typically emit.
type toolCallJSON struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// codeBlockRe matches markdown fenced code blocks (```json ... ``` or ``` ... ```).
var codeBlockRe = regexp.MustCompile("(?s)^\\s*```(?:json|JSON)?\\s*\n?(.*?)\\s*```\\s*$")

// stripCodeFence removes a markdown fenced code block wrapper if present.
// Local models frequently wrap tool call JSON in ```json ... ``` blocks.
func stripCodeFence(text string) string {
	if m := codeBlockRe.FindStringSubmatch(text); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return text
}

// parseToolCallsFromText attempts to parse text as tool call JSON. It returns
// nil if the text is not valid tool call JSON or if any tool name is not in
// the known set. It handles markdown code fences and case-insensitive tool
// name matching.
func parseToolCallsFromText(text string, resolver *toolNameResolver) []parsedToolCall {
	trimmed := strings.TrimSpace(text)
	if len(trimmed) == 0 {
		return nil
	}

	// Strip markdown code fences if present.
	trimmed = stripCodeFence(trimmed)

	// Must start with { or [ to be candidate JSON.
	if len(trimmed) == 0 || (trimmed[0] != '{' && trimmed[0] != '[') {
		return nil
	}

	var candidates []toolCallJSON
	if trimmed[0] == '[' {
		if err := json.Unmarshal([]byte(trimmed), &candidates); err != nil {
			return nil
		}
	} else {
		var single toolCallJSON
		if err := json.Unmarshal([]byte(trimmed), &single); err != nil {
			return nil
		}
		candidates = []toolCallJSON{single}
	}

	if len(candidates) == 0 {
		return nil
	}

	result := make([]parsedToolCall, 0, len(candidates))
	for _, c := range candidates {
		resolvedName := resolver.resolve(c.Name)
		if resolvedName == "" {
			// If any candidate has an unknown name, reject the entire batch
			// to avoid false positives.
			return nil
		}
		result = append(result, parsedToolCall{
			id:   fmt.Sprintf("call_%s", uuid.NewString()),
			name: resolvedName,
			args: normalizeArguments(c.Arguments),
		})
	}
	return result
}

// toolNameResolver maps model-emitted tool names to canonical registered names.
// It handles exact matches, case-insensitive matches, and description-based
// aliases (e.g. a model emitting "Read" can be resolved to the "view" tool
// whose description starts with "Reads").
type toolNameResolver struct {
	// nameMap maps lowercased lookup keys to canonical tool names.
	// Keys include the tool name itself and aliases derived from the
	// first word of the tool description.
	nameMap map[string]string
}

// newToolNameResolver builds a resolver from the tools in a fantasy.Call.
func newToolNameResolver(call fantasy.Call) *toolNameResolver {
	if len(call.Tools) == 0 {
		return nil
	}

	nameMap := make(map[string]string, len(call.Tools)*2)
	for _, t := range call.Tools {
		name := t.GetName()
		if name == "" {
			continue
		}

		// Register the canonical name (lowercased).
		nameMap[strings.ToLower(name)] = name

		// Register an alias from the first word of the description.
		// Small models often emit the description verb as the tool name
		// (e.g. "Read" instead of "view" for a tool described as "Reads
		// and displays file contents...").
		ft, ok := t.(fantasy.FunctionTool)
		if !ok || ft.Description == "" {
			continue
		}
		fields := strings.Fields(ft.Description)
		if len(fields) == 0 {
			continue
		}
		alias := normalizeVerb(fields[0])
		if alias == "" || alias == strings.ToLower(name) {
			continue
		}
		// Only register if unambiguous (no collision with another tool).
		if _, exists := nameMap[alias]; !exists {
			nameMap[alias] = name
		} else if nameMap[alias] != name {
			// Collision — remove the alias to avoid wrong matches.
			delete(nameMap, alias)
		}
	}

	return &toolNameResolver{nameMap: nameMap}
}

// resolve maps a model-emitted name to the canonical tool name.
// Returns "" if no match is found.
func (r *toolNameResolver) resolve(name string) string {
	if name == "" {
		return ""
	}
	lower := normalizeVerb(name)
	if canonical, ok := r.nameMap[lower]; ok {
		return canonical
	}
	return ""
}

// normalizeVerb lowercases a word and strips common English verb suffixes
// so that "Reads", "Read", "reads" all normalize to "read".
func normalizeVerb(word string) string {
	lower := strings.ToLower(strings.TrimSpace(word))
	// Strip trailing "s" (Reads -> Read, Creates -> Create, Executes -> Execute).
	lower = strings.TrimSuffix(lower, "s")
	return lower
}

// normalizeArguments handles both object arguments and stringified JSON
// arguments. Some models emit {"arguments":"{\"key\":\"value\"}"} instead
// of {"arguments":{"key":"value"}}.
func normalizeArguments(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "{}"
	}

	// Try to unmarshal as a string (stringified JSON).
	var s string
	if json.Unmarshal(raw, &s) == nil {
		// Verify the string is valid JSON before returning it.
		if json.Valid([]byte(s)) {
			return s
		}
	}

	// Already an object/array — return as-is.
	return string(raw)
}
