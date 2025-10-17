package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/anthropics/anthropic-sdk-go/option"
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
		// Check if using bearer token authentication
		if os.Getenv("AWS_BEARER_TOKEN_BEDROCK") != "" {
			// Use direct Anthropic client with bearer token in Authorization header
			anthropicOpts := opts
			// Note: Caching is enabled by default, will be used if the model supports it
			anthropicOpts.baseURL = fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com", region)

			// Set bearer token as Authorization header
			if anthropicOpts.extraHeaders == nil {
				anthropicOpts.extraHeaders = make(map[string]string)
			}
			anthropicOpts.extraHeaders["Authorization"] = fmt.Sprintf("Bearer %s", os.Getenv("AWS_BEARER_TOKEN_BEDROCK"))

			// Store the model ID and middleware flag for the Anthropic client
			anthropicOpts.extraParams["bedrockBearerToken"] = "true"
			anthropicOpts.extraParams["bedrockModelID"] = string(model.ID)

			return &bedrockClient{
				providerOptions: opts,
				childProvider:   newAnthropicClient(anthropicOpts, AnthropicClientTypeNormal),
			}
		}

		// Use standard AWS credentials with Bedrock SDK integration
		anthropicOpts := opts
		anthropicOpts.disableCache = true
		anthropicOpts.extraParams["bedrockRegion"] = region

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

// bedrockBearerTokenMiddleware transforms Anthropic API requests to Bedrock format
func bedrockBearerTokenMiddleware(modelID string) option.Middleware {
	return func(r *http.Request, next option.MiddlewareNext) (*http.Response, error) {
		// Only transform POST requests to /v1/messages
		if r.Method == http.MethodPost && r.URL.Path == "/v1/messages" {
			// Read the request body
			body, err := io.ReadAll(r.Body)
			if err != nil {
				return nil, err
			}
			r.Body.Close()

			// Parse the JSON to extract model and stream fields
			var reqData map[string]interface{}
			if err := json.Unmarshal(body, &reqData); err != nil {
				return nil, err
			}

			// Extract stream flag (default to false)
			stream, _ := reqData["stream"].(bool)

			// Remove model and stream from the body
			delete(reqData, "model")
			delete(reqData, "stream")

			// Add anthropic_version if not present
			if _, ok := reqData["anthropic_version"]; !ok {
				reqData["anthropic_version"] = "bedrock-2023-05-31"
			}

			// Re-encode the body
			modifiedBody, err := json.Marshal(reqData)
			if err != nil {
				return nil, err
			}

			// Determine the method
			method := "invoke"
			if stream {
				method = "invoke-with-response-stream"
			}

			// Update the URL path
			r.URL.Path = fmt.Sprintf("/model/%s/%s", modelID, method)
			r.URL.RawPath = fmt.Sprintf("/model/%s/%s", url.QueryEscape(modelID), method)

			// Set the new body
			r.Body = io.NopCloser(bytes.NewReader(modifiedBody))
			r.ContentLength = int64(len(modifiedBody))
			r.GetBody = func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(modifiedBody)), nil
			}
		}

		return next(r)
	}
}
