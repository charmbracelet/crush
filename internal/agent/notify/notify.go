// Package notify defines domain notification types for agent events.
// These types are decoupled from UI concerns so the agent can publish
// events without importing UI packages.
package notify

// Type identifies the kind of agent notification.
type Type string

const (
	// TypeAgentFinished indicates the agent has completed its turn.
	TypeAgentFinished       Type = "agent_finished"
	TypeMemoryDreamStarted  Type = "memory_dream_started"
	TypeMemoryDreamFinished Type = "memory_dream_finished"
	TypeMemoryDreamFailed   Type = "memory_dream_failed"
)

// Notification represents a domain event published by the agent.
type Notification struct {
	SessionID    string
	SessionTitle string
	Type         Type
}
