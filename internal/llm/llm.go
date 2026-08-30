// Package llm defines provider-neutral request and streaming contracts.
package llm

import (
	"context"
	"encoding/json"
	"sync"
)

// EventType identifies one provider-neutral stream event.
type EventType string

const (
	EventTextDelta      EventType = "text_delta"
	EventReasoningDelta EventType = "reasoning_delta"
	EventToolStart      EventType = "tool_start"
	EventToolDelta      EventType = "tool_delta"
	EventToolEnd        EventType = "tool_end"
	EventFinish         EventType = "finish"
)

// Message is the minimal provider-neutral conversation representation.
type Message struct {
	Role    string
	Content string
}

// Tool describes a callable tool without binding the domain to an SDK type.
type Tool struct {
	Name        string
	Description string
	Schema      json.RawMessage
}

// Request contains provider-independent generation inputs.
type Request struct {
	Model       string
	Messages    []Message
	Tools       []Tool
	MaxTokens   int64
	Temperature *float64
	Metadata    ProviderMetadata
}

// Event is emitted incrementally by a Client.
type Event struct {
	Type       EventType
	Text       string
	ToolCallID string
	ToolName   string
	ToolDelta  string
	Finish     string
	Metadata   ProviderMetadata
}

// Usage reports normalized token and cost information.
type Usage struct {
	InputTokens  int64
	OutputTokens int64
	Cost         float64
}

// ProviderMetadata preserves opaque replay metadata at adapter boundaries.
type ProviderMetadata map[string]json.RawMessage

// ProviderError is a normalized provider failure.
type ProviderError struct {
	StatusCode int
	Code       string
	Message    string
	Retryable  bool
}

func (e *ProviderError) Error() string { return e.Message }

// Client streams provider-neutral events to emit.
type Client interface {
	Stream(ctx context.Context, request Request, emit func(Event) error) (Usage, error)
}

// Factory creates a configured provider client.
type Factory func(config json.RawMessage) (Client, error)

var registry = struct {
	sync.RWMutex
	factories map[string]Factory
}{factories: make(map[string]Factory)}

// Register installs a provider factory for compiled-in editions.
func Register(name string, factory Factory) {
	registry.Lock()
	defer registry.Unlock()
	if name == "" || factory == nil {
		panic("llm: invalid provider registration")
	}
	if _, exists := registry.factories[name]; exists {
		panic("llm: duplicate provider registration: " + name)
	}
	registry.factories[name] = factory
}

// FactoryFor returns a compiled-in provider factory.
func FactoryFor(name string) (Factory, bool) {
	registry.RLock()
	defer registry.RUnlock()
	factory, ok := registry.factories[name]
	return factory, ok
}
