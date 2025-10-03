package provider

import (
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/log"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/azure"
	"github.com/openai/openai-go/v2/option"
)

type azureClient struct {
	*openaiClient
}

type AzureClient ProviderClient

func newAzureClient(opts providerClientOptions) AzureClient {
	apiVersion := opts.extraParams["apiVersion"]
	if apiVersion == "" {
		apiVersion = "2025-01-01-preview"
	}

	reqOpts := []option.RequestOption{
		azure.WithEndpoint(opts.baseURL, apiVersion),
	}

	if config.Get().Options.Debug {
		httpClient := log.NewHTTPClient()
		reqOpts = append(reqOpts, option.WithHTTPClient(httpClient))
	}

	reqOpts = append(reqOpts, azure.WithAPIKey(opts.apiKey))
	base := &openaiClient{
		providerOptions: opts,
		client:          openai.NewClient(reqOpts...),
	}

	return &azureClient{openaiClient: base}
}
