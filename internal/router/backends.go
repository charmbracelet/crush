package router

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

// SystemOnePath is where every System One-compatible backend other than
// OpenRouter serves the contract: TypeSafe itself, OpenCode Zen, and any
// self-hosted server implementing the same contract.
const SystemOnePath = "/v1/systemone"

// Preset is a known base URL and model combination offered in the TUI as
// an editable starting point. BaseURL is empty when the backend's own
// default URL applies.
type Preset struct {
	Label   string
	BaseURL string
	Model   string
}

// Backend describes one router provider value: where it is served, which
// model to use when none is configured, and which presets to suggest.
// Values come from each backend's own docs, not from local experiments.
type Backend struct {
	BaseURL      string
	Path         string
	DefaultModel string
	// RequiresBaseURL is true when there is no public default URL, so the
	// user must configure one.
	RequiresBaseURL bool
	// FixedBaseURL is true when a configured base_url is ignored.
	FixedBaseURL bool
	// NestedInput is true for backends that wrap the request's state and
	// questions under an "input" object instead of the flat System One
	// shape. Cloudflare Workers AI is the only such backend today.
	NestedInput bool
	Presets     []Preset
}

// Backends maps every supported options.router.provider value to its
// backend. Every entry except openrouter and cloudflare speaks System One
// at POST {base_url}/v1/systemone.
var Backends = map[string]Backend{
	"openrouter": {
		BaseURL:      "https://openrouter.ai",
		Path:         "/api/alpha/decisions",
		DefaultModel: "~typesafe/jev-latest",
		FixedBaseURL: true,
	},
	"opencode-zen": {
		BaseURL:      "https://opencode.ai/zen",
		Path:         SystemOnePath,
		DefaultModel: "jev-1.13",
		Presets: []Preset{
			{Label: "Jev 1.13", Model: "jev-1.13"},
			{Label: "Jev 1.13 Free", Model: "jev-1.13-free"},
		},
	},
	"typesafe": {
		BaseURL:      "https://api.typesafe.ai",
		Path:         SystemOnePath,
		DefaultModel: "jev-latest",
		Presets: []Preset{
			{Label: "Jev Latest", Model: "jev-latest"},
		},
	},
	"vercel": {
		// Vercel AI Gateway's TypeSafe-compatible API: the same System
		// One request and response shapes at a different path, with its
		// own gateway key rather than a TypeSafe key.
		BaseURL:      "https://ai-gateway.vercel.sh",
		Path:         "/typesafe/v1/systemone",
		DefaultModel: "typesafe-ai/jev",
		Presets: []Preset{
			{Label: "TypeSafe: Jev", Model: "typesafe-ai/jev"},
		},
	},
	"cloudflare": {
		// Cloudflare Workers AI serves the same Jev model, but its run
		// URL is account-scoped and it nests state/questions under an
		// "input" object, so both base_url and the request shape differ.
		// base_url must be the full run URL, e.g.
		// https://api.cloudflare.com/client/v4/accounts/<id>/ai/run
		RequiresBaseURL: true,
		NestedInput:     true,
		DefaultModel:    "typesafe/jev",
		Presets: []Preset{
			{Label: "TypeSafe: Jev", Model: "typesafe/jev"},
		},
	},
	// "local" has no built-in presets: any self-hosted System
	// One-compatible server works, but naming specific ones here would
	// bake unmaintained third-party defaults into Crush. The user supplies
	// base_url and model directly. See docs for examples of compatible
	// servers.
	"local": {
		Path:            SystemOnePath,
		RequiresBaseURL: true,
	},
}

// ProviderNames returns every supported provider value, sorted, with
// openrouter (the default) first.
func ProviderNames() []string {
	names := make([]string, 0, len(Backends))
	for name := range Backends {
		if name != "openrouter" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return append([]string{"openrouter"}, names...)
}

// openRouterModelsURL lists only models whose output is decisions (System
// One models such as Jev or Kev), not regular text LLMs.
var openRouterModelsURL = "https://openrouter.ai/api/v1/models?output_modalities=decisions"

// ListOpenRouterDecisionModels fetches OpenRouter's live catalog of
// decision models, so the TUI never has to hardcode which ones exist.
func ListOpenRouterDecisionModels(ctx context.Context, client *http.Client) ([]Preset, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, openRouterModelsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("router: build models request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("router: models request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("router: unexpected models status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var decoded struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("router: decode models: %w", err)
	}
	presets := make([]Preset, 0, len(decoded.Data))
	for _, m := range decoded.Data {
		presets = append(presets, Preset{Label: m.Name, Model: m.ID})
	}
	sort.Slice(presets, func(i, j int) bool { return presets[i].Model < presets[j].Model })
	return presets, nil
}
