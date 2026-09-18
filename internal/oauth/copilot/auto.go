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

// fetchAutoSession opens an auto mode session: the server hands back the
// models the account may use right now and a session token that
// authorizes routing chat requests to them.
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

// autoResolver caches the auto session and resolves the pseudo-model to
// a concrete one. The session token routes the request server-side, so
// the server may still substitute a better fit than the model picked
// here.
type autoResolver struct {
	token func() *oauth.Token

	mu      sync.Mutex
	session *autoSession
	model   string
}

func newAutoResolver(token func() *oauth.Token) *autoResolver {
	return &autoResolver{token: token}
}

// resolve returns the model to send in place of "auto" and the session
// token to authorize it.
func (r *autoResolver) resolve(ctx context.Context) (string, string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Refresh the session once used past its expiry, with the same
	// five-minute safety margin the VS Code extension applies.
	if r.session != nil && r.session.ExpiresAt*1000-time.Now().UnixMilli() > 5*60*1000 {
		return r.model, r.session.SessionToken, nil
	}

	token := r.token()
	session, err := fetchAutoSession(ctx, token)
	if err != nil {
		return "", "", err
	}
	model := pickAutoModel(ctx, token, session.AvailableModels)
	if model == "" {
		return "", "", fmt.Errorf("the Copilot auto session granted no model accessible via the /chat/completions endpoint")
	}
	r.session = session
	r.model = model
	return model, session.SessionToken, nil
}

// chatCompletionsEndpoint is the catalog's name for the Chat Completions
// API, the endpoint the provider selects for the "auto" pseudo-model.
const chatCompletionsEndpoint = "/chat/completions"

// pickAutoModel chooses the first session-granted model the account can
// serve over the Chat Completions API, skipping Responses-only models
// such as gpt-5.4-mini, which the server rejects on /chat/completions.
// This mirrors the VS Code extension, which intersects the session's
// available models with the catalog's supported endpoints.
func pickAutoModel(ctx context.Context, token *oauth.Token, available []string) string {
	catalog, err := fetchCatalog(ctx, token)
	if err != nil {
		slog.Warn("Failed to fetch Copilot catalog for auto model selection, using the first session model", "error", err)
		return available[0]
	}
	endpoints := make(map[string][]string, len(catalog))
	for _, m := range catalog {
		endpoints[m.ID] = m.SupportedEndpoints
	}
	for _, id := range available {
		supported, ok := endpoints[id]
		// Models absent from the catalog or without endpoint metadata
		// predate supported_endpoints and default to Chat Completions.
		if !ok || len(supported) == 0 || slices.Contains(supported, chatCompletionsEndpoint) {
			return id
		}
	}
	return ""
}
