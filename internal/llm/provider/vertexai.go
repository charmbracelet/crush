package provider

import (
	"context"
	"log/slog"

	"google.golang.org/genai"
)

func NewVertexAIProvider(base *baseProvider) Provider {
	project := base.extraHeaders["project"]
	location := base.extraHeaders["location"]
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		Project:  project,
		Location: location,
		Backend:  genai.BackendVertexAI,
	})
	if err != nil {
		slog.Error("Failed to create VertexAI client", "error", err)
		return nil
	}

	return &geminiProvider{
		baseProvider: base,
		client:       client,
	}
}
