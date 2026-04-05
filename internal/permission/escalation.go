package permission

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// EscalationRequest represents a permission escalation request from a worker agent.
type EscalationRequest struct {
	RequestID       string                 `json:"request_id"`
	WorkerID        string                 `json:"worker_id"`
	WorkerName      string                 `json:"worker_name"`
	WorkerColor     string                 `json:"worker_color,omitempty"`
	ToolName        string                 `json:"tool_name"`
	ToolInput       map[string]interface{} `json:"tool_input"`
	Description     string                 `json:"description"`
	SessionID       string                 `json:"session_id"`
	ParentSessionID string                 `json:"parent_session_id,omitempty"`
	CreatedAt       int64                  `json:"created_at"`
}

// EscalationResponse is the leader's response to an escalation request.
type EscalationResponse struct {
	RequestID    string                 `json:"request_id"`
	Approved     bool                   `json:"approved"`
	Reason       string                 `json:"reason,omitempty"`
	UpdatedInput map[string]interface{} `json:"updated_input,omitempty"`
}

// EscalationBridge handles permission escalation from subagents to the leader agent.
type EscalationBridge struct {
	mu           sync.RWMutex
	requests     map[string]chan EscalationResponse
	pendingQueue []EscalationRequest
	requestIDGen int64
}

// NewEscalationBridge creates a new escalation bridge.
func NewEscalationBridge() *EscalationBridge {
	return &EscalationBridge{
		requests:     make(map[string]chan EscalationResponse),
		pendingQueue: make([]EscalationRequest, 0),
	}
}

// RequestEscalation submits a permission escalation request from a worker and waits for response.
func (eb *EscalationBridge) RequestEscalation(ctx context.Context, req EscalationRequest) (*EscalationResponse, error) {
	eb.mu.Lock()
	eb.requestIDGen++
	req.RequestID = fmt.Sprintf("escal-%d-%d", time.Now().UnixMilli(), eb.requestIDGen)
	req.CreatedAt = time.Now().UnixMilli()

	respChan := make(chan EscalationResponse, 1)
	eb.requests[req.RequestID] = respChan
	eb.pendingQueue = append(eb.pendingQueue, req)
	eb.mu.Unlock()

	select {
	case resp := <-respChan:
		return &resp, nil
	case <-ctx.Done():
		eb.mu.Lock()
		delete(eb.requests, req.RequestID)
		for i, pending := range eb.pendingQueue {
			if pending.RequestID == req.RequestID {
				eb.pendingQueue = append(eb.pendingQueue[:i], eb.pendingQueue[i+1:]...)
				break
			}
		}
		eb.mu.Unlock()
		return nil, fmt.Errorf("escalation request canceled: %w", ctx.Err())
	}
}

// RespondToEscalation sends a response to a pending escalation request.
func (eb *EscalationBridge) RespondToEscalation(resp EscalationResponse) error {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	respChan, ok := eb.requests[resp.RequestID]
	if !ok {
		return fmt.Errorf("escalation request %s not found", resp.RequestID)
	}

	respChan <- resp
	delete(eb.requests, resp.RequestID)

	for i, req := range eb.pendingQueue {
		if req.RequestID == resp.RequestID {
			eb.pendingQueue = append(eb.pendingQueue[:i], eb.pendingQueue[i+1:]...)
			break
		}
	}

	return nil
}

// GetPendingEscalations returns all pending escalation requests.
func (eb *EscalationBridge) GetPendingEscalations() []EscalationRequest {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	result := make([]EscalationRequest, 0, len(eb.pendingQueue))
	for _, pending := range eb.pendingQueue {
		cloned := pending
		if pending.ToolInput != nil {
			cloned.ToolInput = make(map[string]interface{}, len(pending.ToolInput))
			for key, value := range pending.ToolInput {
				cloned.ToolInput[key] = value
			}
		}
		result = append(result, cloned)
	}
	return result
}

// HasPendingEscalations checks if there are any pending requests.
func (eb *EscalationBridge) HasPendingEscalations() bool {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	return len(eb.pendingQueue) > 0
}

type escalationBridgeKey struct{}

// WithEscalationBridge adds an escalation bridge to the context.
func WithEscalationBridge(ctx context.Context, bridge *EscalationBridge) context.Context {
	return context.WithValue(ctx, escalationBridgeKey{}, bridge)
}

// EscalationBridgeFromContext retrieves the escalation bridge from context.
func EscalationBridgeFromContext(ctx context.Context) *EscalationBridge {
	bridge, _ := ctx.Value(escalationBridgeKey{}).(*EscalationBridge)
	return bridge
}

// WorkerIdentity identifies a worker agent for permission requests.
type WorkerIdentity struct {
	AgentID   string `json:"agent_id"`
	AgentName string `json:"agent_name"`
	AgentType string `json:"agent_type"`
	Color     string `json:"color,omitempty"`
}

type workerIdentityKey struct{}

// WithWorkerIdentity adds worker identity to the context.
func WithWorkerIdentity(ctx context.Context, identity WorkerIdentity) context.Context {
	return context.WithValue(ctx, workerIdentityKey{}, identity)
}

// WorkerIdentityFromContext retrieves worker identity from context.
func WorkerIdentityFromContext(ctx context.Context) WorkerIdentity {
	identity, _ := ctx.Value(workerIdentityKey{}).(WorkerIdentity)
	return identity
}
