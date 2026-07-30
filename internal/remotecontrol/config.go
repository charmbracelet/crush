package remotecontrol

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// Config holds relay connection settings. Password must come from the
// environment (CRUSH_REMOTE_PASS); it is never loaded from crush.json.
type Config struct {
	RelayURL string
	Username string
	Password string
}

// ResolveConfig merges explicit values, crush.json remote_control block
// fields (url/user only), and environment variables.
//
// Precedence for URL/user: explicit non-empty > env > file.
// Password: explicit non-empty > env only.
func ResolveConfig(fileURL, fileUser, explicitURL, explicitUser, explicitPass string) (Config, error) {
	cfg := Config{
		RelayURL: firstNonEmpty(explicitURL, os.Getenv("CRUSH_REMOTE_URL"), fileURL),
		Username: firstNonEmpty(explicitUser, os.Getenv("CRUSH_REMOTE_USER"), fileUser),
		Password: firstNonEmpty(explicitPass, os.Getenv("CRUSH_REMOTE_PASS")),
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate checks required fields and refuses insecure defaults.
func (c Config) Validate() error {
	if strings.TrimSpace(c.RelayURL) == "" {
		return fmt.Errorf("remote control relay URL is required (set CRUSH_REMOTE_URL or remote_control.relay_url)")
	}
	if strings.TrimSpace(c.Username) == "" {
		return fmt.Errorf("remote control username is required (set CRUSH_REMOTE_USER or remote_control.username)")
	}
	if c.Password == "" {
		return fmt.Errorf("remote control password is required (set CRUSH_REMOTE_PASS)")
	}
	// Reject the historical demo password so a forgotten env cannot ship open.
	if c.Password == "crushsecret" {
		return fmt.Errorf("remote control password must not be the insecure default; set a unique CRUSH_REMOTE_PASS")
	}
	u, err := url.Parse(c.RelayURL)
	if err != nil {
		return fmt.Errorf("invalid relay URL: %w", err)
	}
	switch u.Scheme {
	case "ws", "wss":
	default:
		return fmt.Errorf("relay URL must use ws:// or wss:// scheme")
	}
	host := u.Hostname()
	if u.Scheme == "ws" && !isLoopbackHost(host) {
		return fmt.Errorf("plain ws:// is only allowed for localhost; use wss:// for %q", host)
	}
	return nil
}

func isLoopbackHost(host string) bool {
	h := strings.ToLower(host)
	return h == "localhost" || h == "127.0.0.1" || h == "::1" || h == "[::1]"
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
