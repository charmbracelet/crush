package copilot

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

const (
	githubHost = "github.com"
	// crushAppKey is the apps.json entry written by a client authenticating
	// as us.
	crushAppKey = githubHost + ":" + clientID
)

// RefreshTokenFromDisk returns a GitHub OAuth token that a Copilot client
// already stored on this machine, if one is there.
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

	if app, ok := content[crushAppKey]; ok && app.OAuthToken != "" {
		return app.OAuthToken, true
	}

	// Every client keys its entry by host and its own app id, so a token
	// written by any of them works against api.github.com just as well as
	// one written under our id. Enterprise hosts are skipped: their tokens
	// are not valid against the public API. Keys are sorted so a file
	// holding several entries always resolves to the same one.
	for _, key := range slices.Sorted(maps.Keys(content)) {
		if key != githubHost && !strings.HasPrefix(key, githubHost+":") {
			continue
		}
		if token := content[key].OAuthToken; token != "" {
			return token, true
		}
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
