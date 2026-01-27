package toolchain

import (
	"context"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
)

// Event types for chain detection.
const (
	// ChainStartedEvent is emitted when a new tool chain begins.
	ChainStartedEvent pubsub.EventType = "chain_started"
	// ChainUpdatedEvent is emitted when a tool call is added to a chain.
	ChainUpdatedEvent pubsub.EventType = "chain_updated"
	// ChainCompletedEvent is emitted when a tool chain is complete.
	ChainCompletedEvent pubsub.EventType = "chain_completed"
)

// ChainEvent contains the chain and associated metadata for events.
type ChainEvent struct {
	// Chain is the tool chain associated with this event.
	Chain *Chain
	// LatestCall is the most recently added tool call (for update events).
	LatestCall *ToolCall
}

// Detector observes message events and detects tool chains.
// It tracks tool calls within assistant messages and emits events
// when chains start, update, and complete.
type Detector struct {
	*pubsub.Broker[ChainEvent]

	mu     sync.RWMutex
	chains map[string]*Chain // messageID -> chain

	config Config
}

// NewDetector creates a new chain detector with the given configuration.
func NewDetector(cfg Config) *Detector {
	return &Detector{
		Broker: pubsub.NewBroker[ChainEvent](),
		chains: make(map[string]*Chain),
		config: cfg,
	}
}

// NewDetectorWithDefaults creates a new chain detector with default configuration.
func NewDetectorWithDefaults() *Detector {
	return NewDetector(DefaultConfig())
}

// ObserveMessages subscribes to message events and processes them
// to detect and track tool chains. This method blocks until the context
// is cancelled.
func (d *Detector) ObserveMessages(ctx context.Context, messages message.Service) {
	sub := messages.Subscribe(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-sub:
			if !ok {
				return
			}
			d.processMessageEvent(event)
		}
	}
}

// processMessageEvent handles a single message event.
func (d *Detector) processMessageEvent(event pubsub.Event[message.Message]) {
	msg := event.Payload

	switch event.Type {
	case pubsub.CreatedEvent:
		d.handleMessageCreated(msg)
	case pubsub.UpdatedEvent:
		d.handleMessageUpdated(msg)
	}
}

// handleMessageCreated processes a newly created message.
func (d *Detector) handleMessageCreated(msg message.Message) {
	// We only care about assistant messages with tool calls
	if msg.Role != message.Assistant {
		return
	}

	toolCalls := msg.ToolCalls()
	if len(toolCalls) == 0 {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// Create a new chain for this message
	chain := &Chain{
		SessionID: msg.SessionID,
		MessageID: msg.ID,
	}

	// Add any tool calls that are already present
	for _, tc := range toolCalls {
		call := d.convertToolCall(tc, nil)
		chain.Add(call)
	}

	d.chains[msg.ID] = chain

	// Emit chain started event
	d.Publish(ChainStartedEvent, ChainEvent{
		Chain:      chain,
		LatestCall: chain.Last(),
	})
}

// handleMessageUpdated processes an updated message.
func (d *Detector) handleMessageUpdated(msg message.Message) {
	// Handle tool result messages
	if msg.Role == message.Tool {
		d.handleToolResults(msg)
		return
	}

	// Handle assistant message updates
	if msg.Role != message.Assistant {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	chain, exists := d.chains[msg.ID]
	if !exists {
		// No existing chain, check if we should create one
		toolCalls := msg.ToolCalls()
		if len(toolCalls) == 0 {
			return
		}

		// Create new chain
		chain = &Chain{
			SessionID: msg.SessionID,
			MessageID: msg.ID,
		}
		d.chains[msg.ID] = chain

		// Add tool calls
		for _, tc := range toolCalls {
			call := d.convertToolCall(tc, nil)
			chain.Add(call)
		}

		d.Publish(ChainStartedEvent, ChainEvent{
			Chain:      chain,
			LatestCall: chain.Last(),
		})
		return
	}

	// Check for new tool calls
	currentCalls := msg.ToolCalls()
	existingIDs := make(map[string]struct{})
	for _, c := range chain.Calls {
		existingIDs[c.ID] = struct{}{}
	}

	// Add any new tool calls
	for _, tc := range currentCalls {
		if _, exists := existingIDs[tc.ID]; !exists {
			call := d.convertToolCall(tc, nil)
			chain.Add(call)
			d.Publish(ChainUpdatedEvent, ChainEvent{
				Chain:      chain,
				LatestCall: &call,
			})
		}
	}

	// Check if the message is finished
	if msg.IsFinished() {
		// Chain is complete
		chain.FinishedAt = time.Now()
		d.Publish(ChainCompletedEvent, ChainEvent{
			Chain: chain,
		})
	}
}

// handleToolResults processes tool result messages and updates the corresponding chain.
func (d *Detector) handleToolResults(msg message.Message) {
	results := msg.ToolResults()
	if len(results) == 0 {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// Find the chain that contains these tool calls
	for _, chain := range d.chains {
		if chain.SessionID != msg.SessionID {
			continue
		}

		for _, result := range results {
			// Update the corresponding tool call with its result
			for i := range chain.Calls {
				if chain.Calls[i].ID == result.ToolCallID {
					chain.Calls[i].Output = result.Content
					chain.Calls[i].IsError = result.IsError
					chain.Calls[i].FinishedAt = time.Now()
					chain.FinishedAt = time.Now()

					d.Publish(ChainUpdatedEvent, ChainEvent{
						Chain:      chain,
						LatestCall: &chain.Calls[i],
					})
					break
				}
			}
		}
	}
}

// convertToolCall converts a message.ToolCall to a toolchain.ToolCall.
func (d *Detector) convertToolCall(tc message.ToolCall, result *message.ToolResult) ToolCall {
	call := ToolCall{
		ID:        tc.ID,
		Name:      tc.Name,
		Input:     tc.Input,
		StartedAt: time.Now(),
	}

	if result != nil {
		call.Output = result.Content
		call.IsError = result.IsError
		call.FinishedAt = time.Now()
	}

	return call
}

// GetChain returns the chain for the given message ID, or nil if not found.
func (d *Detector) GetChain(messageID string) *Chain {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.chains[messageID]
}

// GetChainForSession returns the active chain for the given session, or nil if none.
func (d *Detector) GetChainForSession(sessionID string) *Chain {
	d.mu.RLock()
	defer d.mu.RUnlock()

	for _, chain := range d.chains {
		if chain.SessionID == sessionID {
			return chain
		}
	}
	return nil
}

// GetAllChains returns all tracked chains.
func (d *Detector) GetAllChains() []*Chain {
	d.mu.RLock()
	defer d.mu.RUnlock()

	chains := make([]*Chain, 0, len(d.chains))
	for _, chain := range d.chains {
		chains = append(chains, chain)
	}
	return chains
}

// ClearChain removes a chain from tracking.
func (d *Detector) ClearChain(messageID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.chains, messageID)
}

// ClearSessionChains removes all chains for a given session.
func (d *Detector) ClearSessionChains(sessionID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for id, chain := range d.chains {
		if chain.SessionID == sessionID {
			delete(d.chains, id)
		}
	}
}

// Config returns the detector's configuration.
func (d *Detector) Config() Config {
	return d.config
}

// SetConfig updates the detector's configuration.
func (d *Detector) SetConfig(cfg Config) {
	d.config = cfg
}

// ActiveChainCount returns the number of chains currently being tracked.
func (d *Detector) ActiveChainCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.chains)
}
