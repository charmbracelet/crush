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

// toolCallDetectingModel detects tool call JSON emitted as plain text by local model servers (e.g. Ollama) and converts it into proper tool call events.
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

// Generate converts text-only responses containing tool call JSON into proper ToolCallContent entries.
func (m *toolCallDetectingModel) Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
	resp, err := m.inner.Generate(ctx, call)
	if err != nil {
		return resp, err
	}

	resolver := newToolNameResolver(call)
	if resolver == nil || len(resp.Content.ToolCalls()) > 0 {
		return resp, nil
	}

	parsed := parseToolCallsFromText(resp.Content.Text(), resolver)
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

// Stream buffers text events and converts accumulated tool call JSON into tool call stream events on TextEnd.
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

		flush := func() bool {
			for _, p := range buffered {
				if !yield(p) {
					return false
				}
			}
			buffered = nil
			buffering = false
			return true
		}

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
					buffered = nil
					buffering = false
					for _, tc := range parsed {
						for _, p := range []fantasy.StreamPart{
							{Type: fantasy.StreamPartTypeToolInputStart, ID: tc.id, ToolCallName: tc.name},
							{Type: fantasy.StreamPartTypeToolInputDelta, ID: tc.id, Delta: tc.args},
							{Type: fantasy.StreamPartTypeToolInputEnd, ID: tc.id},
							{Type: fantasy.StreamPartTypeToolCall, ID: tc.id, ToolCallName: tc.name, ToolCallInput: tc.args},
						} {
							if !yield(p) {
								return
							}
						}
					}
				} else {
					if !flush() {
						return
					}
					if !yield(part) {
						return
					}
				}

			default:
				if !flush() {
					return
				}
				if !yield(part) {
					return
				}
			}
		}

		flush()
	}, nil
}

type parsedToolCall struct {
	id   string
	name string
	args string
}

type toolCallJSON struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

var codeBlockRe = regexp.MustCompile("(?s)^\\s*```(?:json|JSON)?\\s*\n?(.*?)\\s*```\\s*$")

func stripCodeFence(text string) string {
	if m := codeBlockRe.FindStringSubmatch(text); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return text
}

// parseToolCallsFromText parses text as tool call JSON, returning nil if invalid or any tool name is unknown.
func parseToolCallsFromText(text string, resolver *toolNameResolver) []parsedToolCall {
	trimmed := stripCodeFence(strings.TrimSpace(text))
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
		resolved := resolver.resolve(c.Name)
		if resolved == "" {
			return nil // unknown name — reject entire batch to avoid false positives
		}
		result = append(result, parsedToolCall{
			id:   fmt.Sprintf("call_%s", uuid.NewString()),
			name: resolved,
			args: normalizeArguments(c.Arguments),
		})
	}
	return result
}

// toolNameResolver maps model-emitted tool names to canonical names via exact match, case-insensitive match, or description-based alias (e.g. "Read" -> "view").
type toolNameResolver struct {
	nameMap map[string]string
}

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
		nameMap[strings.ToLower(name)] = name

		// Models often emit the description verb as the tool name (e.g. "Read" for "view").
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
		if _, exists := nameMap[alias]; !exists {
			nameMap[alias] = name
		} else if nameMap[alias] != name {
			delete(nameMap, alias) // collision — remove to avoid wrong matches
		}
	}

	return &toolNameResolver{nameMap: nameMap}
}

func (r *toolNameResolver) resolve(name string) string {
	if name == "" {
		return ""
	}
	if canonical, ok := r.nameMap[normalizeVerb(name)]; ok {
		return canonical
	}
	return ""
}

// normalizeVerb lowercases and strips trailing "s" so "Reads", "Read", "reads" all become "read".
func normalizeVerb(word string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(word)), "s")
}

// normalizeArguments unwraps stringified JSON arguments into proper JSON objects.
func normalizeArguments(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "{}"
	}
	var s string
	if json.Unmarshal(raw, &s) == nil && json.Valid([]byte(s)) {
		return s
	}
	return string(raw)
}
