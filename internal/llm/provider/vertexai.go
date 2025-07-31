package provider

import (
	"context"
	"log/slog"

	"google.golang.org/genai"
)

type VertexAIClient ProviderClient

func newVertexAIClient(opts providerClientOptions) VertexAIClient {
	project := opts.extraParams["project"]
	location := opts.extraParams["location"]
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		Project:  project,
		Location: location,
		Backend:  genai.BackendVertexAI,
	})
	if err != nil {
		slog.Error("Failed to create VertexAI client", "error", err)
		return nil
	}

	return &geminiClient{
		providerOptions: opts,
		client:          client,
	}
}
