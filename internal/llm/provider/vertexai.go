package provider

import (
	"context"
	"log/slog"
	"strings"

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
		cc.HTTPClient = log.NewHTTPClient()
	}
	client, err := genai.NewClient(context.Background(), cc)
	if err != nil {
		slog.Error("Failed to create VertexAI client", "error", err)
		return nil
	}

	model := opts.model(opts.modelType)
	if strings.Contains(model.ID, "anthropic") || strings.Contains(model.ID, "claude-sonnet") {
		return newAnthropicClient(opts, AnthropicClientTypeVertex)
	}
	return &geminiClient{
		providerOptions: opts,
		client:          client,
	}
}
