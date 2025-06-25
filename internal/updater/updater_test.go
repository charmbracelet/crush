package updater

import (
	"context"
	"testing"
	"time"
)

func TestCheckForUpdate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	updateInfo, err := CheckForUpdate(ctx)
	if err != nil {
		t.Logf("Update check failed (this is expected if network is unavailable): %v", err)
		return
	}

	if updateInfo != nil {
		t.Logf("Update available: %s -> %s", updateInfo.CurrentVersion, updateInfo.LatestVersion)
		if updateInfo.CurrentVersion == "" {
			t.Error("Current version should not be empty")
		}
		if updateInfo.LatestVersion == "" {
			t.Error("Latest version should not be empty")
		}
		if updateInfo.DownloadURL == "" {
			t.Error("Download URL should not be empty")
		}
	} else {
		t.Log("No update available")
	}
}

func TestBuildFilename(t *testing.T) {
	filename := buildFilename()
	if filename == "" {
		t.Error("Filename should not be empty")
	}
	t.Logf("Generated filename: %s", filename)
}