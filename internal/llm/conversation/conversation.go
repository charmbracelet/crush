package conversation

import (
	"errors"

	"github.com/charmbracelet/crush/internal/message"
)

type Conversation struct {
	turns []message.Message
}

var ErrEmptyMessage = errors.New("message empty")

// New creates a new Conversation containing the provided messages.
// The input slice is copied to avoid sharing underlying arrays.
func New(turns ...message.Message) Conversation {
	return Conversation{turns: append([]message.Message(nil), turns...)}
}

// Len returns the number of messages in the conversation.
func (c *Conversation) Len() int {
	return len(c.turns)
}

// Messages returns a copy of all messages in the conversation.
// The returned slice is a shallow copy to prevent external modification.
func (c Conversation) Messages() []message.Message {
	return append([]message.Message(nil), c.turns...)
}

// Add returns a new Conversation with the given message appended.
// If the message has no content parts, ErrEmptyMessage is returned.
func (c *Conversation) Add(m message.Message) (int, error) {
	if len(m.Parts) == 0 {
		return -1, ErrEmptyMessage
	}
	c.turns = append(c.turns, m)
	return len(c.turns) - 1, nil
}

// WithRole returns a new Conversation containing only messages
// that have the specified role. The order of messages is preserved.
func (c Conversation) WithRole(role message.MessageRole) Conversation {
	out := make([]message.Message, 0, len(c.turns))
	for _, m := range c.turns {
		if m.Role == role {
			out = append(out, m)
		}
	}
	return New(out...)
}

// First returns a new Conversation containing at most the first n messages.
// If n is greater than the number of messages, the entire conversation is returned.
func (c Conversation) First(n int) Conversation {
	if n <= 0 {
		return Conversation{turns: nil}
	}
	if n >= len(c.turns) {
		return c
	}
	return New(c.turns[:n]...)
}
