package mcpoauth

import (
	"log/slog"
	"sync"

	"golang.org/x/oauth2"
)

// NewSavingTokenSource wraps an oauth2.TokenSource and calls saver whenever
// the access token changes (i.e. on refresh). This ensures refreshed tokens
// are persisted automatically without re-prompting the user.
//
// Returns nil if wrapped is nil. Returns wrapped directly if saver is nil.
func NewSavingTokenSource(wrapped oauth2.TokenSource, config *oauth2.Config, initialToken *oauth2.Token, saver func(*oauth2.Config, *oauth2.Token)) oauth2.TokenSource {
	if wrapped == nil {
		return nil
	}
	if saver == nil {
		return wrapped
	}
	var accessToken string
	if initialToken != nil {
		accessToken = initialToken.AccessToken
	}
	return &savingTokenSource{
		src:         wrapped,
		saver:       saver,
		config:      config,
		accessToken: accessToken,
	}
}

type savingTokenSource struct {
	mu          sync.Mutex
	src         oauth2.TokenSource
	saver       func(*oauth2.Config, *oauth2.Token)
	config      *oauth2.Config
	accessToken string
}

func (s *savingTokenSource) Token() (*oauth2.Token, error) {
	tok, err := s.src.Token()
	if err != nil {
		slog.Debug("Token refresh failed", "error", err)
		return nil, err
	}
	s.mu.Lock()
	changed := s.accessToken != tok.AccessToken
	if changed {
		s.accessToken = tok.AccessToken
	}
	s.mu.Unlock()
	if changed {
		s.saver(s.config, tok)
	}
	return tok, nil
}
