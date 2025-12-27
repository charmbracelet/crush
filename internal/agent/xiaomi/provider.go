// Package xiaomi provides a fantasy.Provider for Xiaomi API.
package xiaomi

import (
	"net/http"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/openaicompat"
	openaisdk "github.com/openai/openai-go/v2/option"
)

const (
	// Name is the provider type name for Xiaomi.
	Name = "xiaomi"
)

type options struct {
	baseURL    string
	apiKey     string
	headers    map[string]string
	httpClient *http.Client
	extraBody  map[string]any
}

// Option configures the Xiaomi provider.
type Option = func(*options)

// WithBaseURL sets the base URL for the Xiaomi provider.
func WithBaseURL(baseURL string) Option {
	return func(o *options) {
		o.baseURL = baseURL
	}
}

// WithAPIKey sets the API key for the Xiaomi provider.
func WithAPIKey(apiKey string) Option {
	return func(o *options) {
		o.apiKey = apiKey
	}
}

// WithHeaders sets the headers for the Xiaomi provider.
func WithHeaders(headers map[string]string) Option {
	return func(o *options) {
		o.headers = headers
	}
}

// WithHTTPClient sets the HTTP client for the Xiaomi provider.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(o *options) {
		o.httpClient = httpClient
	}
}

// WithExtraBody sets the extra body parameters for the Xiaomi provider.
func WithExtraBody(extraBody map[string]any) Option {
	return func(o *options) {
		o.extraBody = extraBody
	}
}

// New creates a new Xiaomi provider.
func New(opts ...Option) (fantasy.Provider, error) {
	o := options{
		baseURL:   "https://api.xiaomimimo.com/v1",
		headers:   make(map[string]string),
		extraBody: make(map[string]any),
	}
	for _, opt := range opts {
		opt(&o)
	}

	// Build OpenAI-compatible provider with Xiaomi-specific configuration
	openaiOpts := []openaicompat.Option{
		openaicompat.WithBaseURL(o.baseURL),
		openaicompat.WithAPIKey(o.apiKey),
	}

	if len(o.headers) > 0 {
		openaiOpts = append(openaiOpts, openaicompat.WithHeaders(o.headers))
	}

	if o.httpClient != nil {
		openaiOpts = append(openaiOpts, openaicompat.WithHTTPClient(o.httpClient))
	}

	// Xiaomi thinking logic is handled via coordinator's buildXiaomiProvider
	// which passes extraBody to WithSDKOptions.
	for k, v := range o.extraBody {
		openaiOpts = append(openaiOpts, openaicompat.WithSDKOptions(openaisdk.WithJSONSet(k, v)))
	}

	return openaicompat.New(openaiOpts...)
}
