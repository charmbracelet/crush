package antigravity

import (
	"charm.land/catwalk/pkg/catwalk"
)

// GeminiEndpoint is the host the Gemini SDK is pointed at. The SDK builds
// `/v1beta/models/{model}:…` paths against it and the Transport rewrites them
// to the subscription gateway, so the address only has to look like the Gemini
// API for path construction to succeed.
const GeminiEndpoint = "https://generativelanguage.googleapis.com/"

// fallbackCatalog is what the provider offers before a signed-in account's real
// catalog has been fetched. It is written the way the gateway names its models
// — one id per reasoning tier — because CollapseModels then turns it into
// families exactly as the live catalog is turned into them.
var fallbackCatalog = []catwalk.Model{
	{ID: "gemini-3.8-flash-high", Name: "Gemini 3.8 Flash (High)", ContextWindow: 1048576, DefaultMaxTokens: 65536, CanReason: true, SupportsImages: true},
	{ID: "gemini-3.8-flash-medium", Name: "Gemini 3.8 Flash (Medium)", ContextWindow: 1048576, DefaultMaxTokens: 65536, CanReason: true, SupportsImages: true},
	{ID: "gemini-3.8-flash-low", Name: "Gemini 3.8 Flash (Low)", ContextWindow: 1048576, DefaultMaxTokens: 65536, CanReason: true, SupportsImages: true},
	{ID: "gemini-3.1-pro-high", Name: "Gemini 3.1 Pro (High)", ContextWindow: 1048576, DefaultMaxTokens: 65535, CanReason: true, SupportsImages: true},
	{ID: "gemini-3.1-pro-low", Name: "Gemini 3.1 Pro (Low)", ContextWindow: 1048576, DefaultMaxTokens: 65535, CanReason: true, SupportsImages: true},
	{ID: "gemini-3.1-flash-lite", Name: "Gemini 3.1 Flash Lite", ContextWindow: 1048576, DefaultMaxTokens: 8192},
	{ID: "cld-sonnet-4-6", Name: "Claude Sonnet 4.6 (Thinking)", ContextWindow: 250000, DefaultMaxTokens: 64000, CanReason: true, SupportsImages: true},
	{ID: "cld-opus-4-6-thinking", Name: "Claude Opus 4.6 (Thinking)", ContextWindow: 250000, DefaultMaxTokens: 64000, CanReason: true, SupportsImages: true},
}

// KeyPlaceholder satisfies the Gemini SDK's requirement that a client always
// has an API key. A subscription authenticates with an OAuth bearer instead,
// which the transport attaches; this string never reaches a server.
const KeyPlaceholder = "google-subscription"

// ProviderDefinition describes the subscription provider for crush's catalog.
//
// It is a distinct provider id rather than catwalk's `gemini` so an API key
// and a subscription can be configured side by side and picked independently.
// The model list is per-account, so what ships here is only a placeholder
// until login replaces it with Models for the signed-in account.
func ProviderDefinition() catwalk.Provider {
	return catwalk.Provider{
		ID:          catwalk.InferenceProvider(ProviderID),
		Name:        ProviderName,
		Type:        catwalk.TypeGoogle,
		APIEndpoint: GeminiEndpoint,
		// The reasoning selector, not the id, picks the tier: crush starts the
		// large agent on the middle tier and the small one on the model that has
		// no tiers to burn quota on.
		DefaultLargeModelID: "gemini-3.8-flash",
		DefaultSmallModelID: "gemini-3.1-flash-lite",
		Models:              CollapseModels(fallbackCatalog),
	}
}
