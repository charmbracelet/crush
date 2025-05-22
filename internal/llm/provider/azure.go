package provider

import (
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/azure"
	"github.com/openai/openai-go/option"
)

type azureClient struct {
	*openaiClient
}

type AzureClient ProviderClient

func newAzureClient(opts providerClientOptions) AzureClient {
	endpoint := os.Getenv("AZURE_OPENAI_ENDPOINT")      // ex: https://foo.openai.azure.com
	apiVersion := os.Getenv("AZURE_OPENAI_API_VERSION") // ex: 2025-04-01-preview

	if endpoint == "" || apiVersion == "" {
		return &azureClient{openaiClient: newOpenAIClient(opts).(*openaiClient)}
	}

	reqOpts := []option.RequestOption{
		azure.WithEndpoint(endpoint, apiVersion),
	}

	if opts.apiKey != "" || os.Getenv("AZURE_OPENAI_API_KEY") != "" {
		key := opts.apiKey
		if key == "" {
			key = os.Getenv("AZURE_OPENAI_API_KEY")
		}
		reqOpts = append(reqOpts, azure.WithAPIKey(key))
	} else if cred, err := azidentity.NewDefaultAzureCredential(nil); err == nil {
		reqOpts = append(reqOpts, azure.WithTokenCredential(cred))
	}

	base := &openaiClient{
		providerOptions: opts,
		client:          openai.NewClient(reqOpts...),
	}

	return &azureClient{openaiClient: base}
}
