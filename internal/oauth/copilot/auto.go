package copilot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
// the first available model is enough; the server may still substitute a
// better fit.
type autoResolver struct {
	token func() *oauth.Token

	mu      sync.Mutex
	session *autoSession
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
		return r.session.AvailableModels[0], r.session.SessionToken, nil
	}

	session, err := fetchAutoSession(ctx, r.token())
	if err != nil {
		return "", "", err
	}
	r.session = session
	return session.AvailableModels[0], session.SessionToken, nil
}
