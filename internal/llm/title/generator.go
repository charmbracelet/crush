package title

import (
	"context"
	"strings"

	"github.com/charmbracelet/crush/internal/llm/conversation"
	"github.com/charmbracelet/crush/internal/llm/provider"
	"github.com/charmbracelet/crush/internal/message"
)

type Strategy int

const (
	FromLastUser Strategy = iota
	FromRecentContext
)

type Generator interface {
	Generate(ctx context.Context, conv conversation.Conversation) (string, error)
}

type service struct {
	provider provider.Provider
}

func NewGenerator(prov provider.Provider) Generator {
	return &service{provider: prov}
}

func (s *service) Generate(ctx context.Context, conv conversation.Conversation) (string, error) {
	msgs := conv.WithRole(message.User).First(1).Messages()
	if len(msgs) == 0 {
		return "", nil
	}

	stream := s.provider.StreamResponse(ctx, msgs, nil)
	var final string
	for ev := range stream {
		if ev.Error != nil {
			return "", ev.Error
		}
		if ev.Response != nil {
			final = ev.Response.Content
		}
	}
	title := strings.TrimSpace(strings.ReplaceAll(final, "\n", " "))
	return title, nil
}
