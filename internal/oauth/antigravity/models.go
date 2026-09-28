package antigravity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"charm.land/catwalk/pkg/catwalk"

	"github.com/charmbracelet/crush/internal/oauth"
)

// ProviderID is the crush provider this integration is attached to. It is a
// crush-owned id rather than catwalk's `gemini` so that an API key and a
// subscription can be configured side by side.
const ProviderID = "gemini-sub"

// ProviderName is the display name in the model picker.
const ProviderName = "Google AI Subscription"

// Models returns the models the signed-in subscription grants.
//
// Unlike a public API catalog, this one is per-account and includes
// Anthropic- and OpenAI-family models reached through the same gateway, with
// the reasoning effort baked into the id (`-high`, `-low`). Internal
// placeholders are dropped because offering them only produces confusing
// failures.
func Models(ctx context.Context, token *oauth.Token) ([]catwalk.Model, error) {
	if token == nil {
		return nil, fmt.Errorf("no subscription credentials")
	}

	body, _ := json.Marshal(map[string]any{})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, APIBase+":fetchAvailableModels", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", UserAgent)

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("read model catalog: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &oauth.TokenExchangeError{StatusCode: resp.StatusCode, Body: string(raw)}
	}

	return parseModels(raw)
}

type catalogEntry struct {
	DisplayName      string   `json:"displayName"`
	MaxTokens        int64    `json:"maxTokens"`
	MaxOutputTokens  int64    `json:"maxOutputTokens"`
	SupportsThinking *bool    `json:"supportsThinking"`
	SupportsImages   *bool    `json:"supportsImages"`
	ModelProvider    string   `json:"modelProvider"`
	APIProvider      string   `json:"apiProvider"`
	QuotaInfo        quotaRef `json:"quotaInfo"`
}

type quotaRef struct {
	RemainingFraction float64 `json:"remainingFraction"`
	ResetTime         string  `json:"resetTime"`
}

func parseModels(raw []byte) ([]catwalk.Model, error) {
	var payload struct {
		Models map[string]catalogEntry `json:"models"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode model catalog: %w", err)
	}

	models := make([]catwalk.Model, 0, len(payload.Models))
	for id, entry := range payload.Models {
		if !usableModel(id, entry) {
			continue
		}
		maxOut := entry.MaxOutputTokens
		if maxOut <= 0 {
			maxOut = 8192
		}
		_, level := SplitEffort(id)
		models = append(models, catwalk.Model{
			ID: SafeModelID(id),
			// The display name is untouched, so the picker still reads
			// "Claude Sonnet 4.6" even though the id is mangled.
			Name:                   orDefault(entry.DisplayName, id),
			ContextWindow:          entry.MaxTokens,
			DefaultMaxTokens:       maxOut,
			CanReason:              entry.SupportsThinking != nil && *entry.SupportsThinking,
			SupportsImages:         entry.SupportsImages != nil && *entry.SupportsImages,
			DefaultReasoningEffort: level,
		})
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("subscription catalog listed no usable models")
	}
	// One entry per model, with the tiers the gateway sells as separate ids
	// folded into reasoning levels.
	return CollapseModels(models), nil
}

// usableModel filters catalog noise: internal chat stubs (`chat_20706`),
// preview surfaces, and entries with no declared context window.
func usableModel(id string, e catalogEntry) bool {
	switch {
	case e.MaxTokens <= 0:
		return false
	case strings.HasPrefix(id, "chat_"):
		return false
	case strings.Contains(id, "preview"):
		return false
	case strings.Contains(strings.ToUpper(e.DisplayName), "PLACEHOLDER"):
		return false
	}
	return true
}

// effortLevels lists the reasoning levels the subscription gateway sells, and
// the model-id suffix each one is hidden behind.
//
// Antigravity has no reasoning parameter: `gemini-3.8-flash-high` and
// `gemini-3.8-flash-low` are separate models, and there is no unsuffixed
// `gemini-3.8-flash` at all. crush has a reasoning-effort selector built for
// exactly this, so the catalog is collapsed onto one entry per family and the
// Transport puts the suffix back when it builds the request envelope. The order
// matters: `extra-low` has to be tested before `low`.
var effortLevels = []struct {
	level  string
	suffix string
}{
	{level: "minimal", suffix: "extra-low"},
	{level: "low", suffix: "low"},
	{level: "medium", suffix: "medium"},
	{level: "high", suffix: "high"},
}

// SplitEffort separates a gateway model id into its family and the reasoning
// level encoded in its suffix. level is empty when the id carries none.
func SplitEffort(id string) (family, level string) {
	for _, e := range effortLevels {
		if strings.HasSuffix(id, "-"+e.suffix) {
			return strings.TrimSuffix(id, "-"+e.suffix), e.level
		}
	}
	return id, ""
}

// effortSuffix returns the gateway suffix that sells a reasoning level.
func effortSuffix(level string) (string, bool) {
	level = strings.ToLower(strings.TrimSpace(level))
	for _, e := range effortLevels {
		if e.level == level {
			return e.suffix, true
		}
	}
	return "", false
}

// modelFamily accumulates the catalog entries that share one family id.
type modelFamily struct {
	// base is the unsuffixed entry, for the families the gateway also lists
	// without a level. It is nil for most.
	base *catwalk.Model
	// variants holds the effort-suffixed entries, in id order.
	variants []catwalk.Model
	levels   []string
}

// CollapseModels folds the gateway's per-effort model list into one entry per
// family, exposing the efforts it sells as crush reasoning levels.
//
// The result is what makes "Gemini 3.8 Flash" a single picker entry with
// low/medium/high behind the reasoning selector, rather than three entries that
// differ only in how much quota they burn.
//
// It is idempotent, which lets configuration load reuse it to migrate catalogs
// persisted by earlier builds that listed every effort separately.
func CollapseModels(models []catwalk.Model) []catwalk.Model {
	// The catalog arrives from a JSON object, so sort first: grouping a map
	// twice must not produce two different pickers.
	input := slices.Clone(models)
	slices.SortFunc(input, func(a, b catwalk.Model) int { return strings.Compare(a.ID, b.ID) })

	families := make(map[string]*modelFamily, len(input))
	order := make([]string, 0, len(input))
	for _, m := range input {
		id, level := SplitEffort(m.ID)
		f := families[id]
		if f == nil {
			f = &modelFamily{}
			families[id] = f
			order = append(order, id)
		}
		if level == "" {
			base := m
			f.base = &base
			continue
		}
		f.variants = append(f.variants, m)
		if !slices.Contains(f.levels, level) {
			f.levels = append(f.levels, level)
		}
	}

	out := make([]catwalk.Model, 0, len(order))
	for _, id := range order {
		out = append(out, families[id].collapse(id))
	}
	return out
}

// collapse renders one family as a single catalog entry.
func (f *modelFamily) collapse(id string) catwalk.Model {
	levels := f.sortedLevels()
	if f.base != nil && len(f.variants) == 0 {
		// Nothing to fold: keep the entry exactly as the gateway described it,
		// including any reasoning level it may already carry.
		base := *f.base
		base.ID = id
		return base
	}

	// Capabilities come from the unsuffixed entry when there is one, since that
	// is the gateway describing the model itself rather than one of its tiers.
	source := f.base
	if source == nil {
		source = &f.variants[0]
	}
	m := *source
	m.ID = id
	m.Name = f.displayName(source.Name)
	m.ReasoningLevels = levels
	if def := defaultEffort(levels); def != "" {
		m.CanReason = true
		m.DefaultReasoningEffort = def
		return m
	}
	m.DefaultReasoningEffort = ""
	return m
}

// displayName is the family name with the effort qualifier removed, so the
// picker reads "Gemini 3.8 Flash" rather than inheriting one tier's label.
//
// The parenthetical is dropped whenever the entry it came from was a tier,
// because the gateway's labels do not reliably match its own suffixes: the
// `extra-low` variant is sold as "Gemini 3.5 Flash (Low)".
func (f *modelFamily) displayName(fallback string) string {
	if f.base != nil {
		return f.base.Name
	}
	name, hadParen := trimTrailingParen(fallback)
	if !hadParen {
		return fallback
	}
	return name
}

// trimTrailingParen removes a trailing " (…)" group, reporting whether one was
// present.
func trimTrailingParen(name string) (string, bool) {
	open := strings.LastIndex(name, " (")
	if open <= 0 || !strings.HasSuffix(name, ")") {
		return name, false
	}
	return strings.TrimRight(name[:open], " "), true
}

// sortedLevels returns the family's levels from least to most thinking.
func (f *modelFamily) sortedLevels() []string {
	levels := make([]string, 0, len(f.levels))
	for _, e := range effortLevels {
		if slices.Contains(f.levels, e.level) {
			levels = append(levels, e.level)
		}
	}
	return levels
}

// defaultEffort chooses the level a collapsed model starts on. The gateway has
// no notion of a default, so take the middle of what it sells, falling back to
// the strongest tier — which is what the suffixed model list pointed at before.
func defaultEffort(levels []string) string {
	for _, want := range []string{"medium", "high", "low", "minimal"} {
		if slices.Contains(levels, want) {
			return want
		}
	}
	return ""
}

// Anthropic prefixes that must not appear in a model id.
//
// The Gemini provider in fantasy diverts any model id containing "claude" or
// "anthropic" to the Anthropic-on-Vertex client, which then demands real
// Anthropic credentials even though the model is reachable through this
// gateway. Ids are therefore mangled on the way out and mapped back before the
// request is sent; display names are unaffected.
var idReplacements = []struct{ real, safe string }{
	{"claude", "cld"},
	{"anthropic", "antr"},
}

// SafeModelID maps a subscription model id to one crush will not intercept.
func SafeModelID(id string) string {
	out := id
	for _, r := range idReplacements {
		out = strings.ReplaceAll(out, r.real, r.safe)
	}
	return out
}

// RealModelID reverses SafeModelID. It is applied to every outgoing model
// name, so ids that were never mangled pass through unchanged.
func RealModelID(id string) string {
	out := id
	for _, r := range idReplacements {
		out = strings.ReplaceAll(out, r.safe, r.real)
	}
	return out
}

// DiscoverProject reads the Cloud Code Assist project id bound to the
// account. Requests work without it (the backend derives it from the
// credential), so it is only used for display and for accounts that need the
// project pinned.
func DiscoverProject(ctx context.Context, token *oauth.Token) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"metadata": map[string]string{"ideType": "ANTIGRAVITY"},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, APIBase+":loadCodeAssist", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", UserAgent)

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", &oauth.TokenExchangeError{StatusCode: resp.StatusCode, Body: string(raw)}
	}
	var payload struct {
		Project string `json:"cloudaicompanionProject"`
		Current struct {
			ID string `json:"id"`
		} `json:"currentTier"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", fmt.Errorf("decode code assist setup: %w", err)
	}
	return payload.Project, nil
}

func orDefault(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
