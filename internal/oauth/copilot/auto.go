package copilot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/oauth"
)

// AutoModelID is the pseudo-model GitHub offers accounts that cannot pick
// models directly, such as Copilot Free and student plans.
const AutoModelID = "auto"

// autoSessionURL opens an auto mode session.
var autoSessionURL = copilotBaseURL + "/models/session"

type autoSession struct {
	AvailableModels []string `json:"available_models"`
	SessionToken    string   `json:"session_token"`
	ExpiresAt       int64    `json:"expires_at"`
}

type autoEndpoint uint8

const (
	autoEndpointUnknown autoEndpoint = iota
	autoEndpointChatCompletions
	autoEndpointResponses
)

const (
	chatCompletionsEndpoint = "/chat/completions"
	responsesEndpoint       = "/responses"
)

type autoSelection struct {
	model        string
	sessionToken string
	endpoint     autoEndpoint
}

// fetchAutoSession opens an auto mode session: the server hands back the
// models the account may use right now and a session token that
// authorizes routing inference requests to them.
func fetchAutoSession(ctx context.Context, token *oauth.Token) (*autoSession, error) {
	if token == nil {
		return nil, fmt.Errorf("an OAuth token is required to open a Copilot auto session")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, autoSessionURL,
		bytes.NewReader([]byte(`{"auto_mode":{"model_hints":["auto"]}}`)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	for k, v := range Headers() {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("open Copilot auto session: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read Copilot auto session: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &oauth.TokenExchangeError{
			StatusCode: resp.StatusCode,
			Body:       string(body),
		}
	}

	var session autoSession
	if err := json.Unmarshal(body, &session); err != nil {
		return nil, fmt.Errorf("decode Copilot auto session: %w", err)
	}
	if session.SessionToken == "" || len(session.AvailableModels) == 0 {
		return nil, fmt.Errorf("the Copilot auto session granted no usable model")
	}
	return &session, nil
}

// AutoResolver caches the auto session and resolves the pseudo-model to
// a concrete one. The session token routes the request server-side, so
// the server may still substitute a better fit than the model picked
// here.
type AutoResolver struct {
	token                func() *oauth.Token
	defaultUsesResponses func(string) bool

	mu        sync.Mutex
	session   *autoSession
	selection autoSelection
	routes    map[string]autoEndpoint
}

// NewAutoResolver creates a resolver whose fallback routing predicate handles
// concrete models without endpoint metadata.
func NewAutoResolver(token func() *oauth.Token, defaultUsesResponses func(string) bool) *AutoResolver {
	return &AutoResolver{
		token:                token,
		defaultUsesResponses: defaultUsesResponses,
		routes:               make(map[string]autoEndpoint),
	}
}

// resolve returns the model to send in place of "auto" and the session
// token to authorize it.
func (r *AutoResolver) resolve(ctx context.Context) (autoSelection, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Refresh the session once used past its expiry, with the same
	// five-minute safety margin the VS Code extension applies.
	if r.session != nil && r.session.ExpiresAt*1000-time.Now().UnixMilli() > 5*60*1000 {
		return r.selection, nil
	}

	token := r.token()
	session, err := fetchAutoSession(ctx, token)
	if err != nil {
		return autoSelection{}, err
	}
	model, endpoint := pickAutoModel(ctx, token, session.AvailableModels)
	if model == "" {
		return autoSelection{}, fmt.Errorf("the Copilot auto session granted no model accessible via a supported endpoint")
	}
	selection := autoSelection{
		model:        model,
		sessionToken: session.SessionToken,
		endpoint:     endpoint,
	}
	r.session = session
	r.selection = selection
	r.routes[model] = endpoint
	return selection, nil
}

// UsesResponsesAPI reports whether modelID should use the Responses API.
// Session catalog metadata takes precedence over the static model fallback.
func (r *AutoResolver) UsesResponsesAPI(modelID string) bool {
	r.mu.Lock()
	endpoint, ok := r.routes[modelID]
	r.mu.Unlock()

	if ok && endpoint != autoEndpointUnknown {
		return endpoint == autoEndpointResponses
	}
	return r.defaultUsesResponses != nil && r.defaultUsesResponses(modelID)
}

// pickAutoModel chooses the first session-granted model that supports an API
// Crush can serve. Models supporting both APIs retain the provider's default
// routing preference.
func pickAutoModel(ctx context.Context, token *oauth.Token, available []string) (string, autoEndpoint) {
	catalog, err := fetchCatalog(ctx, token)
	if err != nil {
		slog.Warn("Failed to fetch Copilot catalog for auto model selection, using the first session model", "error", err)
		return available[0], autoEndpointUnknown
	}
	endpoints := make(map[string][]string, len(catalog))
	for _, m := range catalog {
		endpoints[m.ID] = m.SupportedEndpoints
	}
	for _, id := range available {
		supported, ok := endpoints[id]
		// Models absent from the catalog or without endpoint metadata
		// predate supported_endpoints and use the provider's default route.
		if !ok || len(supported) == 0 {
			return id, autoEndpointUnknown
		}

		supportsChat := slices.Contains(supported, chatCompletionsEndpoint)
		supportsResponses := slices.Contains(supported, responsesEndpoint)
		switch {
		case supportsResponses && !supportsChat:
			return id, autoEndpointResponses
		case supportsChat && !supportsResponses:
			return id, autoEndpointChatCompletions
		case supportsChat && supportsResponses:
			return id, autoEndpointUnknown
		}
	}
	return "", autoEndpointUnknown
}
