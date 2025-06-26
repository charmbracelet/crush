package provider

import (
	"context"
	"os"

	"github.com/charmbracelet/crush/internal/logging"
	"google.golang.org/genai"
)

type VertexAIClient ProviderClient

func newVertexAIClient(opts providerClientOptions) VertexAIClient {
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		Project:  os.Getenv("GOOGLE_CLOUD_PROJECT"),
		Location: os.Getenv("GOOGLE_CLOUD_LOCATION"),
		Backend:  genai.BackendVertexAI,
	})
	if err != nil {
		logging.Error("Failed to create VertexAI client", "error", err)
		return nil
	}

	return &geminiClient{
		providerOptions: opts,
		client:          client,
	}
}
