// Package azure provides configuration options so you can connect and use Azure OpenAI using the [openai.Client].
//
// Typical usage of this package will look like this:
//
//	client := openai.NewClient(
//		azure.WithEndpoint(azureOpenAIEndpoint, azureOpenAIAPIVersion),
//		azure.WithTokenCredential(azureIdentityTokenCredential),
//		// or azure.WithAPIKey(azureOpenAIAPIKey),
//	)
//
// Or, if you want to construct a specific service:
//
//	client := openai.NewChatCompletionService(
//		azure.WithEndpoint(azureOpenAIEndpoint, azureOpenAIAPIVersion),
//		azure.WithTokenCredential(azureIdentityTokenCredential),
//		// or azure.WithAPIKey(azureOpenAIAPIKey),
//	)
package azure

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/openai/openai-go/internal/requestconfig"
	"github.com/openai/openai-go/option"
)

// WithEndpoint configures this client to connect to an Azure OpenAI endpoint.
//
//   - endpoint - the Azure OpenAI endpoint to connect to. Ex: https://<azure-openai-resource>.openai.azure.com
//   - apiVersion - the Azure OpenAI API version to target (ex: 2024-06-01). See [Azure OpenAI apiversions] for current API versions. This value cannot be empty.
//
// This function should be paired with a call to authenticate, like [azure.WithAPIKey] or [azure.WithTokenCredential], similar to this:
//
//	client := openai.NewClient(
//		azure.WithEndpoint(azureOpenAIEndpoint, azureOpenAIAPIVersion),
//		azure.WithTokenCredential(azureIdentityTokenCredential),
//		// or azure.WithAPIKey(azureOpenAIAPIKey),
//	)
//
// [Azure OpenAI apiversions]: https://learn.microsoft.com/en-us/azure/ai-services/openai/reference#rest-api-versioning
func WithEndpoint(endpoint string, apiVersion string) option.RequestOption {
	if !strings.HasSuffix(endpoint, "/") {
		endpoint += "/"
	}

	endpoint += "openai/"

	withQueryAdd := option.WithQueryAdd("api-version", apiVersion)
	withEndpoint := option.WithBaseURL(endpoint)

	withModelMiddleware := option.WithMiddleware(func(r *http.Request, mn option.MiddlewareNext) (*http.Response, error) {
		replacementPath, err := getReplacementPathWithDeployment(r)

		if err != nil {
			return nil, err
		}

		r.URL.Path = replacementPath
		return mn(r)
	})

	return requestconfig.RequestOptionFunc(func(rc *requestconfig.RequestConfig) error {
		if apiVersion == "" {
			return errors.New("apiVersion is an empty string, but needs to be set. See https://learn.microsoft.com/en-us/azure/ai-services/openai/reference#rest-api-versioning for details.")
		}

		if err := withQueryAdd.Apply(rc); err != nil {
			return err
		}

		if err := withEndpoint.Apply(rc); err != nil {
			return err
		}

		if err := withModelMiddleware.Apply(rc); err != nil {
			return err
		}

		return nil
	})
}

// WithTokenCredential configures this client to authenticate using an [Azure Identity] TokenCredential.
// This function should be paired with a call to [WithEndpoint] to point to your Azure OpenAI instance.
//
// [Azure Identity]: https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/azidentity
func WithTokenCredential(tokenCredential azcore.TokenCredential) option.RequestOption {
	bearerTokenPolicy := runtime.NewBearerTokenPolicy(tokenCredential, []string{"https://cognitiveservices.azure.com/.default"}, nil)

	// add in a middleware that uses the bearer token generated from the token credential
	return option.WithMiddleware(func(req *http.Request, next option.MiddlewareNext) (*http.Response, error) {
		pipeline := runtime.NewPipeline("azopenai-extensions", version, runtime.PipelineOptions{}, &policy.ClientOptions{
			InsecureAllowCredentialWithHTTP: true, // allow for plain HTTP proxies, etc..
			PerRetryPolicies: []policy.Policy{
				bearerTokenPolicy,
				policyAdapter(next),
			},
		})

		req2, err := runtime.NewRequestFromRequest(req)

		if err != nil {
			return nil, err
		}

		return pipeline.Do(req2)
	})
}

// WithAPIKey configures this client to authenticate using an API key.
// This function should be paired with a call to [WithEndpoint] to point to your Azure OpenAI instance.
func WithAPIKey(apiKey string) option.RequestOption {
	// NOTE: there is an option.WithApiKey(), but that adds the value into
	// the Authorization header instead so we're doing this instead.
	return option.WithHeader("Api-Key", apiKey)
}

// jsonRoutes have JSON payloads - we'll deserialize looking for a .model field in there
// so we won't have to worry about individual types for completions vs embeddings, etc...
var jsonRoutes = map[string]bool{
	"/openai/completions":        true,
	"/openai/chat/completions":   true,
	"/openai/embeddings":         true,
	"/openai/audio/speech":       true,
	"/openai/images/generations": true,
}

// audioMultipartRoutes have mime/multipart payloads. These are less generic - we're very much
// expecting a transcription or translation payload for these.
var audioMultipartRoutes = map[string]bool{
	"/openai/audio/transcriptions": true,
	"/openai/audio/translations":   true,
}

// getReplacementPathWithDeployment parses the request body to extract out the Model parameter (or equivalent)
// (note, the req.Body is fully read as part of this, and is replaced with a bytes.Reader)
func getReplacementPathWithDeployment(req *http.Request) (string, error) {
	if jsonRoutes[req.URL.Path] {
		return getJSONRoute(req)
	}

	if audioMultipartRoutes[req.URL.Path] {
		return getAudioMultipartRoute(req)
	}

	// No need to relocate the path. We've already tacked on /openai when we setup the endpoint.
	return req.URL.Path, nil
}

func getJSONRoute(req *http.Request) (string, error) {
	// we need to deserialize the body, partly, in order to read out the model field.
	jsonBytes, err := io.ReadAll(req.Body)

	if err != nil {
		return "", err
	}

	// make sure we restore the body so it can be used in later middlewares.
	req.Body = io.NopCloser(bytes.NewReader(jsonBytes))

	var v *struct {
		Model string `json:"model"`
	}

	if err := json.Unmarshal(jsonBytes, &v); err != nil {
		return "", err
	}

	escapedDeployment := url.PathEscape(v.Model)
	return strings.Replace(req.URL.Path, "/openai/", "/openai/deployments/"+escapedDeployment+"/", 1), nil
}

func getAudioMultipartRoute(req *http.Request) (string, error) {
	// body is a multipart/mime body type instead.
	mimeBytes, err := io.ReadAll(req.Body)

	if err != nil {
		return "", err
	}

	// make sure we restore the body so it can be used in later middlewares.
	req.Body = io.NopCloser(bytes.NewReader(mimeBytes))

	_, mimeParams, err := mime.ParseMediaType(req.Header.Get("Content-Type"))

	if err != nil {
		return "", err
	}

	mimeReader := multipart.NewReader(
		io.NopCloser(bytes.NewReader(mimeBytes)),
		mimeParams["boundary"])

	for {
		mp, err := mimeReader.NextPart()

		if err != nil {
			if errors.Is(err, io.EOF) {
				return "", errors.New("unable to find the model part in multipart body")
			}

			return "", err
		}

		defer mp.Close()

		if mp.FormName() == "model" {
			modelBytes, err := io.ReadAll(mp)

			if err != nil {
				return "", err
			}

			escapedDeployment := url.PathEscape(string(modelBytes))
			return strings.Replace(req.URL.Path, "/openai/", "/openai/deployments/"+escapedDeployment+"/", 1), nil
		}
	}
}

type policyAdapter option.MiddlewareNext

func (mp policyAdapter) Do(req *policy.Request) (*http.Response, error) {
	return (option.MiddlewareNext)(mp)(req.Raw())
}

const version = "v.0.1.0"
