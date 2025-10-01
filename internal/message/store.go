package message

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Store is a simple in-memory message store for headless mode
// No persistence, no pubsub - just basic CRUD operations
type Store struct {
	mu       sync.RWMutex
	messages map[string][]Message // sessionID -> messages
}

// NewStore creates a new in-memory message store
func NewStore() *Store {
	return &Store{
		messages: make(map[string][]Message),
	}
}

// Create creates a new message in the store
func (s *Store) Create(ctx context.Context, sessionID string, params CreateMessageParams) (Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()
	msg := Message{
		ID:        uuid.New().String(),
		Role:      params.Role,
		SessionID: sessionID,
		Parts:     params.Parts,
		Model:     params.Model,
		Provider:  params.Provider,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if s.messages[sessionID] == nil {
		s.messages[sessionID] = []Message{}
	}
	s.messages[sessionID] = append(s.messages[sessionID], msg)

	return msg, nil
}

// Update updates an existing message in the store
func (s *Store) Update(ctx context.Context, msg Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	msgs, ok := s.messages[msg.SessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", msg.SessionID)
	}

	for i, m := range msgs {
		if m.ID == msg.ID {
			msg.UpdatedAt = time.Now().Unix()
			s.messages[msg.SessionID][i] = msg
			return nil
		}
	}

	return fmt.Errorf("message not found: %s", msg.ID)
}

// Get retrieves a message by ID
func (s *Store) Get(ctx context.Context, sessionID, messageID string) (Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	msgs, ok := s.messages[sessionID]
	if !ok {
		return Message{}, fmt.Errorf("session not found: %s", sessionID)
	}

	for _, m := range msgs {
		if m.ID == messageID {
			return m, nil
		}
	}

	return Message{}, fmt.Errorf("message not found: %s", messageID)
}

// List retrieves all messages for a session
func (s *Store) List(ctx context.Context, sessionID string) ([]Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	msgs, ok := s.messages[sessionID]
	if !ok {
		// Return empty list instead of error for non-existent sessions
		return []Message{}, nil
	}

	// Return a copy to avoid race conditions
	result := make([]Message, len(msgs))
	copy(result, msgs)
	return result, nil
}

// Delete removes a message from the store
func (s *Store) Delete(ctx context.Context, sessionID, messageID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	msgs, ok := s.messages[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	for i, m := range msgs {
		if m.ID == messageID {
			s.messages[sessionID] = append(msgs[:i], msgs[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("message not found: %s", messageID)
}

// DeleteSession removes all messages for a session
func (s *Store) DeleteSession(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.messages, sessionID)
}

// Clear removes all messages from the store
func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = make(map[string][]Message)
}
