package provider

import (
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/azure"
	"github.com/openai/openai-go/option"
)

type azureProvider struct {
	*openaiProvider
}

func NewAzureProvider(base *baseProvider) Provider {
	apiVersion := base.extraParams["apiVersion"]
	if apiVersion == "" {
		apiVersion = "2025-01-01-preview"
	}

	reqOpts := []option.RequestOption{
		azure.WithEndpoint(base.baseURL, apiVersion),
	}

	reqOpts = append(reqOpts, azure.WithAPIKey(base.apiKey))
	client := &openaiProvider{
		baseProvider: base,
		client:       openai.NewClient(reqOpts...),
	}

	return &azureProvider{openaiProvider: client}
}
