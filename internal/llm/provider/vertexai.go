package provider

import (
	"context"
	"log/slog"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/log"
	"google.golang.org/genai"
)

type VertexAIClient ProviderClient

func newVertexAIClient(opts providerClientOptions) VertexAIClient {
	project := opts.extraParams["project"]
	location := opts.extraParams["location"]
	cc := &genai.ClientConfig{
		Project:  project,
		Location: location,
		Backend:  genai.BackendVertexAI,
	}
	if config.Get().Options.Debug {
		cc.HTTPClient = log.NewHTTPClient(nil)
	}
	client, err := genai.NewClient(context.Background(), cc)
	if err != nil {
		slog.Error("Failed to create VertexAI client", "error", err)
		return nil
	}

	return &geminiClient{
		providerOptions: opts,
		client:          client,
	}
}
