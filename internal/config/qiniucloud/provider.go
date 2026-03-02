// Package qiniucloud defines the QiniuCloud AI inference provider for crush.
//
// QiniuCloud offers an OpenAI-compatible API (https://sufy.com) with two endpoints:
//   - Domestic (China): https://api.qnaigc.com/v1
//   - Overseas:         https://openai.sufy.com/v1
//
// Configuration via crush.toml:
//
//	[providers.qiniucloud]
//	api_key = "$QINIUCLOUD_API_KEY"
//	# Optional: override base URL for overseas access
//	# base_url = "https://openai.sufy.com/v1"
//	# Optional: append extra models (merged with the built-in list)
//	# models = [{ id = "your-model-id", name = "Your Model" }]
//
// Environment variables:
//   - QINIUCLOUD_API_KEY  — API key (required to activate this provider)
//   - QINIUCLOUD_BASE_URL — Override base URL (optional, defaults to domestic endpoint)
package qiniucloud

import "charm.land/catwalk/pkg/catwalk"

const (
	// ProviderID is the unique identifier for the QiniuCloud provider.
	ProviderID = "qiniucloud"

	// ProviderName is the display name shown in the model picker.
	ProviderName = "QiniuCloud"

	// BaseURLDomestic is the default endpoint for mainland China users.
	BaseURLDomestic = "https://api.qnaigc.com/v1"

	// BaseURLOverseas is the endpoint for users outside mainland China.
	BaseURLOverseas = "https://openai.sufy.com/v1"

	// DefaultBaseURL is the out-of-the-box endpoint (domestic).
	DefaultBaseURL = BaseURLDomestic

	// EnvAPIKey is the environment variable name for the API key.
	EnvAPIKey = "QINIUCLOUD_API_KEY"

	// EnvBaseURL is the environment variable name for overriding the base URL.
	EnvBaseURL = "QINIUCLOUD_BASE_URL"
)

// Provider returns the catwalk.Provider definition for QiniuCloud.
// baseURL overrides the endpoint; if empty, DefaultBaseURL (domestic) is used.
//
// The model list is sourced from https://sufy.com/zh-CN/services/ai-inference/models
// and updated periodically with crush releases. Users can extend or override the
// list via their crush.toml configuration.
func Provider(baseURL string) catwalk.Provider {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return catwalk.Provider{
		ID:          catwalk.InferenceProvider(ProviderID),
		Name:        ProviderName,
		APIEndpoint: baseURL,
		APIKey:      "$" + EnvAPIKey,
		Type:        catwalk.TypeOpenAICompat,
		Models:      models,
	}
}

// models is the built-in model list for QiniuCloud (text/code generation only).
// Last updated: 2026-03 from https://sufy.com/zh-CN/services/ai-inference/models
// To add newer models without waiting for a release, use the models array in crush.toml.
var models = []catwalk.Model{
	// DeepSeek
	{ID: "deepseek-r1", Name: "DeepSeek R1"},
	{ID: "deepseek-v3-0324", Name: "DeepSeek V3 (0324)"},
	{ID: "deepseek-v3", Name: "DeepSeek V3"},

	// Qwen (Alibaba)
	{ID: "qwen3-235b-a22b", Name: "Qwen3 235B A22B"},
	{ID: "qwen3-30b-a3b", Name: "Qwen3 30B A3B"},
	{ID: "qwen-max", Name: "Qwen Max"},
	{ID: "qwen-plus", Name: "Qwen Plus"},
	{ID: "qwen-turbo", Name: "Qwen Turbo"},

	// GLM (Z-AI / Zhipu)
	{ID: "glm-4-plus", Name: "GLM-4 Plus"},
	{ID: "glm-4-air", Name: "GLM-4 Air"},
	{ID: "glm-4-flash", Name: "GLM-4 Flash"},

	// Moonshot / Kimi
	{ID: "moonshot-v1-128k", Name: "Kimi (128K)"},
	{ID: "kimi-k2", Name: "Kimi K2"},

	// Minimax
	{ID: "minimax-m1", Name: "MiniMax M1"},
}
