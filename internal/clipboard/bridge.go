package clipboard

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxBridgeImageBytes = 5 * 1024 * 1024

var errBridgeDisabled = errors.New("clipboard bridge is not configured")

type bridgeConfig struct {
	URL       string `json:"url"`
	Token     string `json:"token,omitempty"`
	TokenFile string `json:"token_file,omitempty"`
}

func readBridgeImage() ([]byte, error) {
	cfg, err := loadBridgeConfig()
	if err != nil {
		return nil, err
	}

	endpoint, err := url.Parse(strings.TrimRight(cfg.URL, "/") + "/v1/image")
	if err != nil {
		return nil, fmt.Errorf("invalid clipboard bridge URL: %w", err)
	}
	if !isLoopbackHost(endpoint.Hostname()) {
		return nil, errors.New("clipboard bridge URL must use a loopback host")
	}

	req, err := http.NewRequest(http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create clipboard bridge request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Token)

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("read clipboard bridge: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil, ErrEmpty
	case http.StatusUnauthorized:
		return nil, errors.New("clipboard bridge rejected its authentication token")
	case http.StatusRequestEntityTooLarge:
		return nil, errors.New("clipboard image exceeds the 5 MB limit")
	case http.StatusOK:
	default:
		return nil, fmt.Errorf("clipboard bridge returned HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBridgeImageBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read clipboard bridge response: %w", err)
	}
	if len(data) == 0 {
		return nil, ErrEmpty
	}
	if len(data) > maxBridgeImageBytes {
		return nil, errors.New("clipboard image exceeds the 5 MB limit")
	}

	mimeType := http.DetectContentType(data)
	switch mimeType {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		return data, nil
	default:
		return nil, fmt.Errorf("clipboard bridge returned unsupported content type %q", mimeType)
	}
}

func loadBridgeConfig() (bridgeConfig, error) {
	cfg := bridgeConfig{
		URL:       strings.TrimSpace(os.Getenv("CRUSH_CLIPBOARD_BRIDGE_URL")),
		Token:     strings.TrimSpace(os.Getenv("CRUSH_CLIPBOARD_BRIDGE_TOKEN")),
		TokenFile: strings.TrimSpace(os.Getenv("CRUSH_CLIPBOARD_BRIDGE_TOKEN_FILE")),
	}

	if cfg.URL == "" {
		configDir, err := os.UserConfigDir()
		if err != nil {
			return bridgeConfig{}, errBridgeDisabled
		}
		data, err := os.ReadFile(filepath.Join(configDir, "crush", "clip-bridge.json"))
		if errors.Is(err, os.ErrNotExist) {
			return bridgeConfig{}, errBridgeDisabled
		}
		if err != nil {
			return bridgeConfig{}, fmt.Errorf("read clipboard bridge config: %w", err)
		}
		if err := json.Unmarshal(data, &cfg); err != nil {
			return bridgeConfig{}, fmt.Errorf("parse clipboard bridge config: %w", err)
		}
	}

	if cfg.URL == "" {
		return bridgeConfig{}, errBridgeDisabled
	}
	if cfg.Token == "" && cfg.TokenFile != "" {
		tokenPath, err := expandHome(cfg.TokenFile)
		if err != nil {
			return bridgeConfig{}, err
		}
		data, err := os.ReadFile(tokenPath)
		if err != nil {
			return bridgeConfig{}, fmt.Errorf("read clipboard bridge token: %w", err)
		}
		cfg.Token = strings.TrimSpace(string(data))
	}
	if cfg.Token == "" {
		return bridgeConfig{}, errors.New("clipboard bridge token is empty")
	}
	return cfg, nil
}

func expandHome(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") && !strings.HasPrefix(path, `~\`) {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve clipboard bridge token home: %w", err)
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, path[2:]), nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
