package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/llm/tools"
	"github.com/charmbracelet/crush/internal/message"
)

type bedrockClient struct {
	providerOptions providerClientOptions
	childProvider   ProviderClient
}

type BedrockClient ProviderClient

func newBedrockClient(opts providerClientOptions) BedrockClient {
	// Get AWS region from environment
	region := opts.extraParams["region"]
	if region == "" {
		region = "us-east-1" // default region
	}
	if len(region) < 2 {
		return &bedrockClient{
			providerOptions: opts,
			childProvider:   nil, // Will cause an error when used
		}
	}

	opts.model = func(modelType config.SelectedModelType) catwalk.Model {
		model := config.Get().GetModelByType(modelType)

		// Prefix the model name with region
		regionPrefix := region[:2]
		modelName := model.ID
		model.ID = fmt.Sprintf("%s.%s", regionPrefix, modelName)
		return *model
	}

	model := opts.model(opts.modelType)

	// Determine which provider to use based on the model
	if strings.Contains(string(model.ID), "anthropic") {
		// Create Anthropic client with Bedrock configuration
		anthropicOpts := opts
		// TODO: later find a way to check if the AWS account has caching enabled
		opts.disableCache = true // Disable cache for Bedrock
		return &bedrockClient{
			providerOptions: opts,
			childProvider:   newAnthropicClient(anthropicOpts, AnthropicClientTypeBedrock),
		}
	}

	// Return client with nil childProvider if model is not supported
	// This will cause an error when used
	return &bedrockClient{
		providerOptions: opts,
		childProvider:   nil,
	}
}

func (b *bedrockClient) send(ctx context.Context, messages []message.Message, tools []tools.BaseTool) (*ProviderResponse, error) {
	if b.childProvider == nil {
		return nil, errors.New("unsupported model for bedrock provider")
	}
	return b.childProvider.send(ctx, messages, tools)
}

func (b *bedrockClient) stream(ctx context.Context, messages []message.Message, tools []tools.BaseTool) <-chan ProviderEvent {
	eventChan := make(chan ProviderEvent)

	if b.childProvider == nil {
		go func() {
			eventChan <- ProviderEvent{
				Type:  EventError,
				Error: errors.New("unsupported model for bedrock provider"),
			}
			close(eventChan)
		}()
		return eventChan
	}

	return b.childProvider.stream(ctx, messages, tools)
}

func (b *bedrockClient) Model() catwalk.Model {
	return b.providerOptions.model(b.providerOptions.modelType)
}
