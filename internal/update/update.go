package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	githubAPIURL = "https://api.github.com/repos/charmbracelet/crush/releases/latest"
	userAgent    = "crush/1.0"
)

// Default is the default [Client].
var Default Client = &github{}

// Info contains information about an available update.
type Info struct {
	Current string
	Latest  string
	URL     string
}

// Available returns true if there's an update available.
//
// If both current and latest are stable versions, returns true if versions are
// different.
// If current is a pre-release and latest isn't, returns true.
// If latest is a pre-release and current isn't, returns false.
func (i Info) Available() bool {
	cpr := strings.Contains(i.Current, "-")
	lpr := strings.Contains(i.Latest, "-")
	// current is pre release
	if cpr {
		// latest isn't a prerelease
		if !lpr {
			return true
		}
	}
	if lpr && !cpr {
		return false
	}
	return i.Current != i.Latest
}

// Check checks if a new version is available.
func Check(ctx context.Context, current string, client Client) (Info, error) {
	info := Info{
		Current: current,
		Latest:  current,
	}

	if info.Current == "devel" || info.Current == "unknown" {
		return info, nil
	}

	release, err := client.Latest(ctx)
	if err != nil {
		return info, fmt.Errorf("failed to fetch latest release: %w", err)
	}

	info.Latest = strings.TrimPrefix(release.TagName, "v")
	info.URL = release.HTMLURL
	return info, nil
}

// Release represents a GitHub release.
type Release struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// Client is a client that can get the latest release.
type Client interface {
	Latest(ctx context.Context) (*Release, error)
}

type github struct{}

// Latest implements [Client].
func (c *github) Latest(ctx context.Context) (*Release, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", githubAPIURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(body))
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}
