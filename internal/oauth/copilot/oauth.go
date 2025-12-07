package copilot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/oauth"
)

const (
	clientID = "Iv1.b507a08c87ecfe98"

	deviceCodeURL   = "https://github.com/login/device/code"
	accessTokenURL  = "https://github.com/login/oauth/access_token"
	copilotTokenURL = "https://api.github.com/copilot_internal/v2/token"

	userAgent           = "GitHubCopilotChat/0.32.4"
	editorVersion       = "vscode/1.105.1"
	editorPluginVersion = "copilot-chat/0.32.4"
	integrationID       = "vscode-chat"
)

type DeviceCode struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

func RequestDeviceCode(ctx context.Context) (*DeviceCode, error) {
	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("scope", "read:user")

	req, err := http.NewRequestWithContext(ctx, "POST", deviceCodeURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", userAgent)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("device code request failed: %s - %s", resp.Status, string(body))
	}

	var dc DeviceCode
	if err := json.NewDecoder(resp.Body).Decode(&dc); err != nil {
		return nil, err
	}
	return &dc, nil
}

// PollForToken polls GitHub for the access token after user authorization.
func PollForToken(ctx context.Context, dc *DeviceCode) (*oauth.Token, error) {
	interval := max(dc.Interval, 5)
	deadline := time.Now().Add(time.Duration(dc.ExpiresIn) * time.Second)
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}

		token, err := tryGetToken(ctx, dc.DeviceCode)
		if err == errPending {
			continue
		}
		if err == errSlowDown {
			interval += 5
			ticker.Reset(time.Duration(interval) * time.Second)
			continue
		}
		if err != nil {
			return nil, err
		}
		return token, nil
	}

	return nil, fmt.Errorf("authorization timed out")
}

var (
	errPending  = fmt.Errorf("pending")
	errSlowDown = fmt.Errorf("slow_down")
)

func GetExtraHeaders() map[string]string {
	return map[string]string{
		"User-Agent":             userAgent,
		"Editor-Version":         editorVersion,
		"Editor-Plugin-Version":  editorPluginVersion,
		"Copilot-Integration-Id": integrationID,
	}
}

func tryGetToken(ctx context.Context, deviceCode string) (*oauth.Token, error) {
	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("device_code", deviceCode)
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

	req, err := http.NewRequestWithContext(ctx, "POST", accessTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", userAgent)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	switch result.Error {
	case "":
		if result.AccessToken == "" {
			return nil, errPending
		}
		return getCopilotToken(ctx, result.AccessToken)
	case "authorization_pending":
		return nil, errPending
	case "slow_down":
		return nil, errSlowDown
	default:
		return nil, fmt.Errorf("authorization failed: %s", result.Error)
	}
}

func getCopilotToken(ctx context.Context, githubToken string) (*oauth.Token, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", copilotTokenURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+githubToken)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Editor-Version", editorVersion)
	req.Header.Set("Editor-Plugin-Version", editorPluginVersion)
	req.Header.Set("Copilot-Integration-Id", integrationID)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("copilot not available for this account\n\n" +
				"Please ensure you have GitHub Copilot enabled at:\n" +
				"https://github.com/settings/copilot")
		}
		return nil, fmt.Errorf("copilot token request failed: %s - %s", resp.Status, string(body))
	}

	var result struct {
		Token     string `json:"token"`
		ExpiresAt int64  `json:"expires_at"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	copilotToken := &oauth.Token{
		AccessToken:  result.Token,
		RefreshToken: githubToken,
		ExpiresAt:    result.ExpiresAt,
	}
	copilotToken.SetExpiresIn()

	return copilotToken, nil
}

func RefreshToken(ctx context.Context, githubToken string) (*oauth.Token, error) {
	return getCopilotToken(ctx, githubToken)
}
