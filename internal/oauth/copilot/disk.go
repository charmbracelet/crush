package copilot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

func RefreshTokenFromDisk() (string, bool) {
	data, err := os.ReadFile(tokenFilePath())
	if err != nil {
		return "", false
	}
	var content map[string]struct {
		User        string `json:"user"`
		OAuthToken  string `json:"oauth_token"`
		GitHubAppID string `json:"githubAppId"`
	}
	if err := json.Unmarshal(data, &content); err != nil {
		return "", false
	}
	if app, ok := content["github.com:Iv1.b507a08c87ecfe98"]; ok {
		return app.OAuthToken, true
	}
	return "", false
}

func tokenFilePath() string {
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(os.Getenv("LOCALAPPDATA"), "github-copilot/apps.json")
	default:
		return filepath.Join(os.Getenv("HOME"), ".config/github-copilot/apps.json")
	}
}
