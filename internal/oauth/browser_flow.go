package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/oauth/callback"
)

// PKCE holds a code verifier and its S256 code challenge.
type PKCE struct {
	Verifier  string
	Challenge string
}

// GeneratePKCE creates cryptographically secure PKCE values.
func GeneratePKCE() (PKCE, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return PKCE{}, fmt.Errorf("generate PKCE random bytes: %w", err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(b)
	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])
	return PKCE{
		Verifier:  verifier,
		Challenge: challenge,
	}, nil
}

// BrowserFlowConfig defines the parameters for a browser-based PKCE flow.
type BrowserFlowConfig struct {
	Port         int
	CallbackPath string
	Subject      string
	AuthURL      func(redirectURI string, pkce PKCE, state string) string
	Exchange     func(ctx context.Context, code, redirectURI string, pkce PKCE) (*Token, error)
}

type browserSession struct {
	url      string
	server   *http.Server
	listener net.Listener
	result   chan browserResult
	ctx      context.Context
	cancel   context.CancelFunc
	closeMu  sync.Mutex
	closed   bool
}

type browserResult struct {
	token *Token
	err   error
}

// StartBrowserFlow starts a loopback HTTP listener and prepares the browser session.
func StartBrowserFlow(ctx context.Context, cfg BrowserFlowConfig) (BrowserSession, error) {
	pkce, err := GeneratePKCE()
	if err != nil {
		return nil, err
	}

	stateBytes := make([]byte, 24)
	if _, err := rand.Read(stateBytes); err != nil {
		return nil, fmt.Errorf("generate state: %w", err)
	}
	state := base64.RawURLEncoding.EncodeToString(stateBytes)

	addr := fmt.Sprintf("127.0.0.1:%d", cfg.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		if cfg.Port != 0 {
			return nil, fmt.Errorf("start OAuth callback server on port %d: %w (port may be in use)", cfg.Port, err)
		}
		return nil, fmt.Errorf("start OAuth callback server: %w", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	cbPath := cfg.CallbackPath
	if cbPath == "" {
		cbPath = "/auth/callback"
	}

	redirectURI := fmt.Sprintf("http://localhost:%d%s", port, cbPath)
	authURL := cfg.AuthURL(redirectURI, pkce, state)

	flowCtx, cancel := context.WithCancel(ctx)
	session := &browserSession{
		url:      authURL,
		listener: listener,
		result:   make(chan browserResult, 1),
		ctx:      flowCtx,
		cancel:   cancel,
	}

	mux := http.NewServeMux()
	mux.HandleFunc(cbPath, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		queryState := r.URL.Query().Get("state")
		if state == "" || queryState != state {
			_ = callback.Write(w, callback.Result{
				Subject:          cfg.Subject,
				ErrorCode:        "invalid_state",
				ErrorDescription: "State parameter mismatch or missing.",
			})
			return
		}

		if errParam := r.URL.Query().Get("error"); errParam != "" {
			errDesc := r.URL.Query().Get("error_description")
			_ = callback.Write(w, callback.Result{
				Subject:          cfg.Subject,
				ErrorCode:        errParam,
				ErrorDescription: errDesc,
			})
			session.finish(nil, fmt.Errorf("OAuth error: %s - %s", errParam, errDesc))
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			_ = callback.Write(w, callback.Result{
				Subject:          cfg.Subject,
				ErrorCode:        "missing_code",
				ErrorDescription: "Authorization code not returned by provider.",
			})
			session.finish(nil, errors.New("missing authorization code"))
			return
		}

		token, exchangeErr := cfg.Exchange(session.ctx, code, redirectURI, pkce)
		if exchangeErr != nil {
			_ = callback.Write(w, callback.Result{
				Subject:          cfg.Subject,
				ErrorCode:        "exchange_failed",
				ErrorDescription: exchangeErr.Error(),
			})
			session.finish(nil, exchangeErr)
			return
		}

		_ = callback.Write(w, callback.Result{
			Subject: cfg.Subject,
		})
		session.finish(token, nil)
	})

	session.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	go func() {
		if serveErr := session.server.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			session.finish(nil, serveErr)
		}
	}()

	return session, nil
}

func (s *browserSession) URL() string {
	return s.url
}

func (s *browserSession) Wait(ctx context.Context) (*Token, error) {
	select {
	case res := <-s.result:
		s.Close()
		return res.token, res.err
	case <-s.ctx.Done():
		s.Close()
		return nil, s.ctx.Err()
	case <-ctx.Done():
		s.Close()
		return nil, ctx.Err()
	}
}

func (s *browserSession) finish(token *Token, err error) {
	select {
	case s.result <- browserResult{token: token, err: err}:
	default:
	}
}

func (s *browserSession) Close() {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	s.cancel()
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.server.Shutdown(ctx)
	}
}
