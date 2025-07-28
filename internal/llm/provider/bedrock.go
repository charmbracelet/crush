package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/charmbracelet/crush/internal/llm/tools"
	"github.com/charmbracelet/crush/internal/message"
)

type bedrockProvider struct {
	*baseProvider
	region        string
	childProvider Provider
}

func NewBedrockProvider(base *baseProvider) Provider {
	// Get AWS region from environment
	region := base.extraParams["region"]
	if region == "" {
		region = "us-east-1" // default region
	}

	return &bedrockProvider{
		baseProvider:  base,
		childProvider: NewAnthropicProvider(base, true),
	}
}

func (b *bedrockProvider) Send(ctx context.Context, model string, messages []message.Message, tools []tools.BaseTool) (*ProviderResponse, error) {
	if len(b.region) < 2 {
		return nil, errors.New("no region selected")
	}
	regionPrefix := b.region[:2]
	modelName := model
	model = fmt.Sprintf("%s.%s", regionPrefix, modelName)
	messages = b.cleanMessages(messages)
	return b.childProvider.Send(ctx, model, messages, tools)
}

func (b *bedrockProvider) Stream(ctx context.Context, model string, messages []message.Message, tools []tools.BaseTool) <-chan ProviderEvent {
	if len(b.region) < 2 {
		eventChan := make(chan ProviderEvent)
		go func() {
			eventChan <- ProviderEvent{
				Type:  EventError,
				Error: errors.New("no region selected"),
			}
			close(eventChan)
		}()
		return eventChan
	}
	regionPrefix := b.region[:2]
	modelName := model
	model = fmt.Sprintf("%s.%s", regionPrefix, modelName)
	messages = b.cleanMessages(messages)
	return b.childProvider.Stream(ctx, model, messages, tools)
}
