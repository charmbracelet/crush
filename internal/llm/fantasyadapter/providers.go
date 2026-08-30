package fantasyadapter

import (
	"os"
	"strings"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"
	"charm.land/fantasy/providers/openai"
)

func buildOpenAI(request Request) (fantasy.Provider, error) {
	opts := []openai.Option{
		openai.WithAPIKey(request.APIKey),
		openai.WithUseResponsesAPI(),
	}
	if request.HTTPClient != nil {
		opts = append(opts, openai.WithHTTPClient(request.HTTPClient))
	}
	if len(request.Headers) > 0 {
		opts = append(opts, openai.WithHeaders(request.Headers))
	}
	if request.BaseURL != "" {
		opts = append(opts, openai.WithBaseURL(request.BaseURL))
	}
	return openai.New(opts...)
}

func buildAnthropic(request Request) (fantasy.Provider, error) {
	var opts []anthropic.Option
	if request.Headers == nil {
		request.Headers = make(map[string]string)
	}
	switch {
	case strings.HasPrefix(request.APIKey, "Bearer "):
		_ = os.Setenv("ANTHROPIC_API_KEY", "")
		request.Headers["Authorization"] = request.APIKey
	case request.ProviderID == string(catwalk.InferenceProviderMiniMax) || request.ProviderID == string(catwalk.InferenceProviderMiniMaxChina):
		_ = os.Setenv("ANTHROPIC_API_KEY", "")
		request.Headers["Authorization"] = "Bearer " + request.APIKey
	case request.APIKey != "":
		opts = append(opts, anthropic.WithAPIKey(request.APIKey))
	}
	if len(request.Headers) > 0 {
		opts = append(opts, anthropic.WithHeaders(request.Headers))
	}
	if request.BaseURL != "" {
		opts = append(opts, anthropic.WithBaseURL(request.BaseURL))
	}
	if request.HTTPClient != nil {
		opts = append(opts, anthropic.WithHTTPClient(request.HTTPClient))
	}
	return anthropic.New(opts...)
}
