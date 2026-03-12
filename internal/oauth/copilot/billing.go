// Package copilot provides HTTP transport with GitHub Copilot billing support.
package copilot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// ContextKey for initiator type.
type ContextKey string

const (
	// InitiatorTypeKey is the context key for storing initiator type.
	InitiatorTypeKey ContextKey = "copilot_initiator_type"
	// InitiatorUser indicates a billable user-initiated request.
	InitiatorUser = "user"
	// InitiatorAgent indicates a free agent-initiated request.
	InitiatorAgent = "agent"
)

// NewBillingClient creates a new HTTP client that sets X-Initiator header
// based on request type for GitHub Copilot billing optimization.
//
// When copilotService is true:
// - Direct user prompts get X-Initiator: user (billable)
// - Tool calls, sub-agents, summaries, and continuations get X-Initiator: agent (free)
func NewBillingClient(copilotService, debug bool) *http.Client {
	if !copilotService {
		if debug {
			return &http.Client{Transport: &debugTransport{}}
		}
		return http.DefaultClient
	}

	return &http.Client{
		Transport: &billingTransport{
			debug:   debug,
			wrapped: getTransport(debug),
		},
	}
}

// NewClientWithInitiator creates a new HTTP client with explicit initiator type.
// This is the preferred method when you know the request type upfront.
func NewClientWithInitiator(initiatorType string, debug bool) *http.Client {
	return &http.Client{
		Transport: &billingTransport{
			initiatorType: initiatorType,
			debug:         debug,
			wrapped:       getTransport(debug),
			fallback:      false,
		},
	}
}

func getTransport(debug bool) http.RoundTripper {
	if debug {
		return &debugTransport{}
	}
	return http.DefaultTransport
}

// debugTransport logs all requests.
type debugTransport struct{}

func (t *debugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	slog.Debug("HTTP request", "method", req.Method, "url", req.URL.String())
	return http.DefaultTransport.RoundTrip(req)
}

type billingTransport struct {
	initiatorType string
	debug         bool
	wrapped       http.RoundTripper
	fallback      bool
}

func (t *billingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	const (
		xInitiatorHeader = "X-Initiator"
	)

	if req == nil {
		return nil, fmt.Errorf("HTTP request is nil")
	}

	// Get initiator type (may read request body).
	initiator, err := t.getInitiatorType(req)
	if err != nil {
		return nil, fmt.Errorf("failed to determine initiator type: %w", err)
	}
	req.Header.Set(xInitiatorHeader, initiator)

	if t.debug {
		slog.Debug("Setting X-Initiator header", "value", initiator, "url", req.URL.String())
	}

	return t.wrapped.RoundTrip(req)
}

func (t *billingTransport) getInitiatorType(req *http.Request) (string, error) {
	// Priority 1: Context value (highest priority, allows explicit control).
	if v := req.Context().Value(InitiatorTypeKey); v != nil {
		if s, ok := v.(string); ok && (s == InitiatorUser || s == InitiatorAgent) {
			return s, nil
		}
	}

	// Priority 2: Explicit initiator type (deprecated, kept for backward compatibility).
	if t.initiatorType != "" {
		return t.initiatorType, nil
	}

	// Priority 3: Inspect request body.
	if req.Body != nil && req.Body != http.NoBody {
		bodyBytes, err := readAndRestoreRequestBody(req)
		if err != nil {
			return "", fmt.Errorf("failed to read request body for initiator detection: %w", err)
		}
		if initiator := detectInitiatorFromBody(bodyBytes); initiator != "" {
			return initiator, nil
		}
	}

	// Default to user (safe fallback).
	return InitiatorUser, nil
}

func readAndRestoreRequestBody(req *http.Request) ([]byte, error) {
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	req.Body.Close()
	req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	return bodyBytes, nil
}

type requestItem struct {
	kind    string
	role    string
	content string
}

func detectInitiatorFromBody(bodyBytes []byte) string {
	items, ok := parseRequestItems(bodyBytes)
	if !ok {
		return ""
	}

	lastItem, ok := lastMeaningfulItem(items)
	if !ok {
		return ""
	}

	if lastItem.role == "user" {
		slog.Debug("Last meaningful item is a user prompt, marking as user")
		return InitiatorUser
	}

	slog.Debug("Last meaningful item is agent-generated, marking as agent", "kind", lastItem.kind, "role", lastItem.role)
	return InitiatorAgent
}

func parseRequestItems(bodyBytes []byte) ([]requestItem, bool) {
	var payload map[string]any
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		slog.Debug("Failed to parse request body as JSON", "error", err)
		return nil, false
	}

	if rawMessages, ok := payload["messages"].([]any); ok {
		return parseChatMessages(rawMessages)
	}

	if rawInput, ok := payload["input"]; ok {
		switch input := rawInput.(type) {
		case string:
			return []requestItem{{kind: "message", role: "user", content: input}}, true
		case []any:
			return parseResponsesInput(input)
		}
	}

	return nil, false
}

func parseChatMessages(rawMessages []any) ([]requestItem, bool) {
	if len(rawMessages) == 0 {
		return nil, false
	}

	items := make([]requestItem, 0, len(rawMessages))
	for _, rawMsg := range rawMessages {
		msg, ok := rawMsg.(map[string]any)
		if !ok {
			return nil, false
		}

		role, _ := msg["role"].(string)
		items = append(items, requestItem{
			kind:    "message",
			role:    role,
			content: extractTextContent(msg["content"]),
		})
	}

	return items, true
}

func parseResponsesInput(rawInput []any) ([]requestItem, bool) {
	if len(rawInput) == 0 {
		return nil, false
	}

	items := make([]requestItem, 0, len(rawInput))
	for _, rawItem := range rawInput {
		item, ok := rawItem.(map[string]any)
		if !ok {
			return nil, false
		}

		kind, _ := item["type"].(string)
		role, _ := item["role"].(string)
		if kind == "" && role != "" {
			kind = "message"
		}

		items = append(items, requestItem{
			kind:    kind,
			role:    role,
			content: extractTextContent(item["content"]),
		})
	}

	return items, true
}

func lastMeaningfulItem(items []requestItem) (requestItem, bool) {
	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]
		if item.kind == "message" {
			if item.role == "system" || item.role == "developer" || item.role == "" {
				continue
			}
		}

		if item.kind == "" && item.role == "" {
			continue
		}

		return item, true
	}

	return requestItem{}, false
}

func extractTextContent(content any) string {
	switch value := content.(type) {
	case string:
		return value
	case []any:
		parts := make([]string, 0, len(value))
		for _, part := range value {
			if text := extractTextContent(part); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	case map[string]any:
		for _, key := range []string{"text", "content", "input_text", "output_text", "refusal"} {
			if text, ok := value[key].(string); ok {
				return text
			}
		}
		if nested, ok := value["content"]; ok {
			return extractTextContent(nested)
		}
	}

	return ""
}

// ContextWithInitiatorType returns a context with the initiator type set.
// Use this to explicitly control billing behavior for a request.
func ContextWithInitiatorType(ctx context.Context, initiatorType string) context.Context {
	return context.WithValue(ctx, InitiatorTypeKey, initiatorType)
}

func contextInitiator(ctx any) (string, bool) {
	contextValueGetter, ok := ctx.(interface{ Value(any) any })
	if !ok {
		return "", false
	}

	v := contextValueGetter.Value(InitiatorTypeKey)
	s, ok := v.(string)
	if !ok || (s != InitiatorUser && s != InitiatorAgent) {
		return "", false
	}
	return s, true
}
