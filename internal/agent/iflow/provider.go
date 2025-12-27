// Package iflow provides a fantasy.Provider for iFlow API.
package iflow

import (
	"bytes"
	"encoding/json"
	"io"
	"maps"
	"net/http"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/openaicompat"
	openaisdk "github.com/openai/openai-go/v2/option"
)

const (
	// Name is the provider type name for iFlow.
	Name = "iflow"
)

type options struct {
	baseURL    string
	apiKey     string
	headers    map[string]string
	httpClient *http.Client
}

// Option configures the iFlow provider.
type Option = func(*options)

type iflowTransport struct {
	base http.RoundTripper
}

func (t *iflowTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body == nil || req.Body == http.NoBody || req.Method != http.MethodPost {
		return t.base.RoundTrip(req)
	}

	// iFlow doesn't like max_tokens in the payload
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	_ = req.Body.Close()

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err == nil {
		// Some providers/models fail if max_tokens is present
		delete(payload, "max_tokens")
		delete(payload, "max_token")
		body, _ = json.Marshal(payload)
	}

	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	return t.base.RoundTrip(req)
}

// New creates a new iFlow provider.
// iFlow is based on OpenAI-compatible API but requires special User-Agent header.
func New(opts ...Option) (fantasy.Provider, error) {
	o := options{
		baseURL: "https://apis.iflow.cn/v1",
		headers: make(map[string]string),
	}
	for _, opt := range opts {
		opt(&o)
	}

	// iFlow requires "iFlow-Cli" User-Agent for premium models
	o.headers["User-Agent"] = "iFlow-Cli"

	// Build OpenAI-compatible provider with iFlow-specific configuration
	openaiOpts := []openaicompat.Option{
		openaicompat.WithBaseURL(o.baseURL),
		openaicompat.WithAPIKey(o.apiKey),
	}

	if len(o.headers) > 0 {
		openaiOpts = append(openaiOpts, openaicompat.WithHeaders(o.headers))
	}

	httpClient := o.httpClient
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	baseTransport := httpClient.Transport
	if baseTransport == nil {
		baseTransport = http.DefaultTransport
	}
	httpClient = &http.Client{
		Transport: &iflowTransport{base: baseTransport},
		Timeout:   httpClient.Timeout,
	}
	openaiOpts = append(openaiOpts, openaicompat.WithHTTPClient(httpClient))

	return openaicompat.New(openaiOpts...)
}

// WithBaseURL sets the base URL.
func WithBaseURL(url string) Option { return func(o *options) { o.baseURL = url } }

// WithAPIKey sets the API key.
func WithAPIKey(key string) Option { return func(o *options) { o.apiKey = key } }

// WithHeaders sets custom headers.
func WithHeaders(headers map[string]string) Option {
	return func(o *options) {
		maps.Copy(o.headers, headers)
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(o *options) { o.httpClient = client }
}

// WithSDKOptions passes extra SDK options (not typically needed for iFlow).
func WithSDKOptions(opts ...openaisdk.RequestOption) Option {
	// For compatibility, but not used in basic iFlow setup
	return func(o *options) {}
}
