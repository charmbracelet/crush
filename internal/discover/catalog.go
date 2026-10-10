package discover

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/jq"
	"github.com/charmbracelet/crush/internal/oauth"
)

// maxCatalogBody bounds a catalog response. A listing is a few hundred
// entries at most; anything larger is a mispointed URL, not a catalog.
const maxCatalogBody = 8 << 20

// CatalogConfig holds what reading a declared catalog needs.
type CatalogConfig struct {
	ID      string
	Spec    *oauth.CatalogSpec
	BaseURL string
	// Bearer is the credential sent with the request: a login's access
	// token, or the provider's API key when it has no login.
	Bearer  string
	Headers map[string]string
	// Existing models keep their own settings; the catalog fills the rest.
	Existing []catwalk.Model
}

// Catalog fetches the catalog a provider's config declares and maps it with
// the program the same config supplies, so reading a listing whose field
// names Crush has never seen stays the plugin's business.
//
// Declared models are kept: each one retains every field it sets and gains,
// from the catalog, the fields it left alone. Models the catalog lists that
// config never mentioned are appended.
func Catalog(ctx context.Context, cfg CatalogConfig, resolver Resolver) ([]catwalk.Model, error) {
	if !cfg.Spec.Usable() {
		return nil, fmt.Errorf("catalog for provider %s names no url or program", cfg.ID)
	}

	program, err := jq.Compile(cfg.Spec.Program)
	if err != nil {
		return nil, fmt.Errorf("catalog program for provider %s: %w", cfg.ID, err)
	}

	base, path := cfg.Spec.URL, ""
	if !strings.Contains(base, "://") {
		// A bare path is relative to wherever the provider is hosted.
		base, path = cfg.BaseURL, cfg.Spec.URL
	}

	headers := make(map[string]string, len(cfg.Headers)+len(cfg.Spec.Headers))
	for k, v := range cfg.Headers {
		headers[k] = v
	}
	for k, v := range cfg.Spec.Headers {
		headers[k] = v
	}

	// A gateway that serves a catalog at all usually serves it over POST, the
	// same envelope its quota endpoint takes: an empty JSON body says "give me
	// everything".
	method := strings.ToUpper(strings.TrimSpace(orDefault(cfg.Spec.Method, http.MethodGet)))
	var body any
	if method != http.MethodGet {
		if err := json.Unmarshal([]byte(orDefault(cfg.Spec.Body, "{}")), &body); err != nil {
			return nil, fmt.Errorf("catalog body for provider %s is not JSON: %w", cfg.ID, err)
		}
	}

	resp, err := doRequest(ctx, method, base, path, cfg.Bearer, headers, resolver, body)
	if err != nil {
		return nil, fmt.Errorf("fetch catalog for provider %s: %w", cfg.ID, err)
	}
	defer resp.Body.Close() //nolint:errcheck // the body is read below in all cases

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch catalog for provider %s: %s", cfg.ID, resp.Status)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxCatalogBody))
	if err != nil {
		return nil, fmt.Errorf("read catalog for provider %s: %w", cfg.ID, err)
	}

	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("catalog for provider %s is not JSON: %w", cfg.ID, err)
	}

	output, err := program.Run(doc)
	if err != nil {
		return nil, fmt.Errorf("catalog program for provider %s: %w", cfg.ID, err)
	}
	// A program that maps one entry rather than the whole listing returns an
	// object; reading either shape costs one line and spares the author a
	// needless wrap.
	if !isJSONArray(output) {
		output = []any{output}
	}

	encoded, err := json.Marshal(output)
	if err != nil {
		return nil, fmt.Errorf("encode catalog for provider %s: %w", cfg.ID, err)
	}
	var found []catwalk.Model
	if err := json.Unmarshal(encoded, &found); err != nil {
		return nil, fmt.Errorf("catalog program for provider %s produced no model list: %w", cfg.ID, err)
	}

	return merge(cfg.Existing, found), nil
}

func isJSONArray(value any) bool {
	_, ok := value.([]any)
	return ok
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// merge folds a catalog into the models config declares. A declared entry
// keeps what it sets and adopts the rest; entries the catalog adds keep their
// reported order after the declared ones, which is how the model list reads
// when config pins a favourite first.
func merge(declared, found []catwalk.Model) []catwalk.Model {
	byID := make(map[string]int, len(declared))
	out := make([]catwalk.Model, 0, len(declared))
	for _, model := range declared {
		byID[model.ID] = len(out)
		out = append(out, model)
	}

	for _, model := range found {
		if model.ID == "" {
			continue
		}
		at, ok := byID[model.ID]
		if !ok {
			byID[model.ID] = len(out)
			out = append(out, model)
			continue
		}
		out[at] = keepDeclared(out[at], model)
	}
	return out
}

// keepDeclared layers a declared model over the catalog's report of the same
// id. Only fields the declaration actually set win, which is what lets a
// plugin pin a name or a window and still inherit the rest. Booleans cannot
// express "unset", so the catalog decides those.
func keepDeclared(declared, reported catwalk.Model) catwalk.Model {
	merged := reported
	if declared.Name != "" {
		merged.Name = declared.Name
	}
	if declared.CostPer1MIn != 0 {
		merged.CostPer1MIn = declared.CostPer1MIn
	}
	if declared.CostPer1MOut != 0 {
		merged.CostPer1MOut = declared.CostPer1MOut
	}
	if declared.CostPer1MInCached != 0 {
		merged.CostPer1MInCached = declared.CostPer1MInCached
	}
	if declared.CostPer1MOutCached != 0 {
		merged.CostPer1MOutCached = declared.CostPer1MOutCached
	}
	if declared.ContextWindow != 0 {
		merged.ContextWindow = declared.ContextWindow
	}
	if declared.DefaultMaxTokens != 0 {
		merged.DefaultMaxTokens = declared.DefaultMaxTokens
	}
	if len(declared.ReasoningLevels) > 0 {
		merged.ReasoningLevels = declared.ReasoningLevels
	}
	if declared.DefaultReasoningEffort != "" {
		merged.DefaultReasoningEffort = declared.DefaultReasoningEffort
	}
	if declared.CanReason {
		merged.CanReason = true
	}
	if declared.SupportsImages {
		merged.SupportsImages = true
	}
	return merged
}
