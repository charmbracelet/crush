package discover

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"reflect"
	"strings"

	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/jq"
	"github.com/charmbracelet/crush/internal/oauth"
)

// maxCatalogBody bounds a catalog response. A listing is a few hundred
// entries at most; anything larger is a mispointed URL, not a catalog.
const maxCatalogBody = 8 << 20

// CatalogConfig holds what reading a declared catalog needs.
type CatalogConfig struct {
	ID       string
	Spec     *oauth.CatalogSpec
	BaseURL  string
	Bearer   string
	Headers  map[string]string
	Existing []catwalk.Model

	program *jq.Program
}

// Compile validates the catalog and prepares its jq program for reuse.
func (cfg *CatalogConfig) Compile() error {
	if !cfg.Spec.Usable() {
		return errors.New("names no url or program")
	}
	program, err := jq.Compile(cfg.Spec.Program)
	if err != nil {
		return err
	}
	cfg.program = program
	return nil
}

// Catalog fetches the listing, maps it with jq, and folds in declared models.
func Catalog(ctx context.Context, cfg CatalogConfig, resolver Resolver) ([]catwalk.Model, error) {
	if cfg.program == nil {
		if err := cfg.Compile(); err != nil {
			return nil, fmt.Errorf("catalog program for provider %s: %w", cfg.ID, err)
		}
	}

	base, path := cfg.Spec.URL, ""
	if !strings.Contains(base, "://") {
		// A bare path is relative to wherever the provider is hosted.
		base, path = cfg.BaseURL, cfg.Spec.URL
	}

	headers := make(map[string]string)
	maps.Copy(headers, cfg.Headers)
	maps.Copy(headers, cfg.Spec.Headers)

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

	output, err := cfg.program.Run(ctx, doc)
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

	return MergeCatalogModels(cfg.Existing, found), nil
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

// MergeCatalogModels overlays declared fields and preserves declared order.
func MergeCatalogModels(declared, found []catwalk.Model) []catwalk.Model {
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

// keepDeclared overlays set fields, so false can never veto a reported capability.
func keepDeclared(declared, reported catwalk.Model) catwalk.Model {
	overlayFields(reflect.ValueOf(&reported).Elem(), reflect.ValueOf(declared))
	return reported
}

func overlayFields(dst, src reflect.Value) {
	for i := range src.NumField() {
		from, to := src.Field(i), dst.Field(i)
		if !to.CanSet() || from.IsZero() {
			continue
		}
		switch from.Kind() {
		case reflect.Struct:
			overlayFields(to, from)
		case reflect.Slice, reflect.Map:
			if from.Len() > 0 {
				to.Set(from)
			}
		default:
			to.Set(from)
		}
	}
}
