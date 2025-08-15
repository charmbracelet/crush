package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/version"
)

const (
	githubAPIURL = "https://api.github.com/repos/charmbracelet/crush/releases/latest"
	userAgent    = "crush-update-check"
)

// Release represents a GitHub release.
type Release struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// UpdateInfo contains information about an available update.
type UpdateInfo struct {
	CurrentVersion string
	LatestVersion  string
	ReleaseURL     string
	Available      bool
}

// CheckForUpdate checks if a new version is available.
func CheckForUpdate(ctx context.Context) (*UpdateInfo, error) {
	info := &UpdateInfo{
		CurrentVersion: version.Version,
	}

	// Skip update check for development versions.
	if strings.Contains(version.Version, "unknown") {
		return info, nil
	}

	release, err := fetchLatestRelease(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest release: %w", err)
	}

	info.LatestVersion = strings.TrimPrefix(release.TagName, "v")
	info.ReleaseURL = release.HTMLURL

	// Compare versions.
	if compareVersions(info.CurrentVersion, info.LatestVersion) < 0 {
		info.Available = true
	}

	return info, nil
}

// fetchLatestRelease fetches the latest release information from GitHub.
func fetchLatestRelease(ctx context.Context) (*Release, error) {
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

// compareVersions compares two semantic versions.
// Returns -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2.
func compareVersions(v1, v2 string) int {
	// Remove 'v' prefix if present.
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	// Split versions into parts.
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	// Compare each part.
	for i := 0; i < len(parts1) && i < len(parts2); i++ {
		var n1, n2 int
		fmt.Sscanf(parts1[i], "%d", &n1)
		fmt.Sscanf(parts2[i], "%d", &n2)

		if n1 < n2 {
			return -1
		} else if n1 > n2 {
			return 1
		}
	}

	// If all parts are equal, compare lengths.
	if len(parts1) < len(parts2) {
		return -1
	} else if len(parts1) > len(parts2) {
		return 1
	}

	return 0
}

// CheckForUpdateAsync performs an update check in the background and returns immediately.
// If an update is available, it returns the update info through the channel.
func CheckForUpdateAsync(ctx context.Context, dataDir string) <-chan *UpdateInfo {
	ch := make(chan *UpdateInfo, 1)

	go func() {
		defer close(ch)

		// Perform the check.
		info, err := CheckForUpdate(ctx)
		if err != nil {
			// Log error but don't fail.
			fmt.Fprintf(os.Stderr, "Failed to check for updates: %v\n", err)
			return
		}

		// Send update info if available.
		if info.Available {
			ch <- info
		}
	}()

	return ch
}
