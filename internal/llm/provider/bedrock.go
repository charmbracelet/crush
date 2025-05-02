package provider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/message"
)

type bedrockOptions struct {
	// Bedrock specific options can be added here
}

type BedrockOption func(*bedrockOptions)

type bedrockClient struct {
	providerOptions providerClientOptions
	options         bedrockOptions
	childProvider   ProviderClient
}

type BedrockClient ProviderClient

func newBedrockClient(opts providerClientOptions) BedrockClient {
	bedrockOpts := bedrockOptions{}
	// Apply bedrock specific options if they are added in the future

	// Get AWS region from environment
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}

	if region == "" {
		region = "us-east-1" // default region
	}
	if len(region) < 2 {
		return &bedrockClient{
			providerOptions: opts,
			options:         bedrockOpts,
			childProvider:   nil, // Will cause an error when used
		}
	}

	// Prefix the model name with region
	regionPrefix := region[:2]
	modelName := opts.model.APIModel
	opts.model.APIModel = fmt.Sprintf("%s.%s", regionPrefix, modelName)

	// Determine which provider to use based on the model
	if strings.Contains(string(opts.model.APIModel), "anthropic") {
		// Create Anthropic client with Bedrock configuration
		anthropicOpts := opts
		anthropicOpts.anthropicOptions = append(anthropicOpts.anthropicOptions,
			WithAnthropicBedrock(true),
			WithAnthropicDisableCache(),
		)
		return &bedrockClient{
			providerOptions: opts,
			options:         bedrockOpts,
			childProvider:   newAnthropicClient(anthropicOpts),
		}
	}

	// Return client with nil childProvider if model is not supported
	// This will cause an error when used
	return &bedrockClient{
		providerOptions: opts,
		options:         bedrockOpts,
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

