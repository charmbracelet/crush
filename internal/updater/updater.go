package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/version"
)

const (
	githubAPIURL = "https://api.github.com/repos/charmbracelet/crush-internal/releases/latest"
	downloadURL  = "https://github.com/charmbracelet/crush-internal/releases/download"
	timeout      = 30 * time.Second
)

// Release represents a GitHub release
type Release struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Body    string `json:"body"`
}

// UpdateInfo contains information about an available update
type UpdateInfo struct {
	CurrentVersion string
	LatestVersion  string
	ReleaseNotes   string
	DownloadURL    string
}

// CheckForUpdate checks if a new version is available
func CheckForUpdate(ctx context.Context) (*UpdateInfo, error) {
	client := &http.Client{Timeout: timeout}

	req, err := http.NewRequestWithContext(ctx, "GET", githubAPIURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to decode release info: %w", err)
	}

	currentVersion := version.Version
	latestVersion := strings.TrimPrefix(release.TagName, "v")

	// If versions are the same, no update needed
	if currentVersion == latestVersion || currentVersion == "unknown" {
		return nil, nil
	}

	// Build download URL
	filename := buildFilename()
	downloadURL := fmt.Sprintf("%s/%s/%s", downloadURL, release.TagName, filename)

	return &UpdateInfo{
		CurrentVersion: currentVersion,
		LatestVersion:  latestVersion,
		ReleaseNotes:   release.Body,
		DownloadURL:    downloadURL,
	}, nil
}

// PerformUpdate downloads and installs the new version
func PerformUpdate(ctx context.Context, updateInfo *UpdateInfo) error {
	// Get current executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Create temporary directory for download
	tempDir, err := os.MkdirTemp("", "crush-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Download the new version
	filename := buildFilename()
	tempFile := filepath.Join(tempDir, filename)
	if err := downloadFile(ctx, updateInfo.DownloadURL, tempFile); err != nil {
		return fmt.Errorf("failed to download update: %w", err)
	}

	// Extract the binary
	extractedBinary := filepath.Join(tempDir, "crush")
	if strings.HasSuffix(filename, ".zip") {
		if err := extractBinaryFromZip(tempFile, extractedBinary); err != nil {
			return fmt.Errorf("failed to extract binary from zip: %w", err)
		}
	} else {
		if err := extractBinaryFromTarGz(tempFile, extractedBinary); err != nil {
			return fmt.Errorf("failed to extract binary from tar.gz: %w", err)
		}
	}

	// Make the extracted binary executable
	if err := os.Chmod(extractedBinary, 0o755); err != nil {
		return fmt.Errorf("failed to make binary executable: %w", err)
	}

	// Create backup of current binary
	backupPath := execPath + ".backup"
	if err := copyFile(execPath, backupPath); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	// Replace the current binary
	if err := copyFile(extractedBinary, execPath); err != nil {
		// Restore backup on failure
		copyFile(backupPath, execPath)
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	// Remove backup on success
	os.Remove(backupPath)

	return nil
}

// buildFilename constructs the filename for the current platform using GoReleaser naming convention
func buildFilename() string {
	os := runtime.GOOS
	arch := runtime.GOARCH

	// Convert OS names to match GoReleaser template
	var osName string
	switch os {
	case "darwin":
		osName = "mac"
	case "windows":
		osName = "windows"
	case "linux":
		osName = "linux"
	default:
		osName = os
	}

	// Convert arch names to match GoReleaser template
	var archName string
	switch arch {
	case "amd64":
		archName = "x86_64"
	case "386":
		archName = "i386"
	default:
		archName = arch
	}

	// Build filename: crush-{os}-{arch}.tar.gz (or .zip for windows)
	if os == "windows" {
		return fmt.Sprintf("crush-%s-%s.zip", osName, archName)
	}
	return fmt.Sprintf("crush-%s-%s.tar.gz", osName, archName)
}

// downloadFile downloads a file from URL to the specified path
func downloadFile(ctx context.Context, url, filepath string) error {
	client := &http.Client{Timeout: timeout}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// extractBinaryFromTarGz extracts the crush binary from a tar.gz file
func extractBinaryFromTarGz(tarGzPath, outputPath string) error {
	file, err := os.Open(tarGzPath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if header.Name == "crush" && header.Typeflag == tar.TypeReg {
			outFile, err := os.Create(outputPath)
			if err != nil {
				return err
			}
			defer outFile.Close()

			_, err = io.Copy(outFile, tr)
			return err
		}
	}

	return fmt.Errorf("crush binary not found in archive")
}

// extractBinaryFromZip extracts the crush binary from a zip file
func extractBinaryFromZip(zipPath, outputPath string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		if f.Name == "crush.exe" || f.Name == "crush" {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()

			outFile, err := os.Create(outputPath)
			if err != nil {
				return err
			}
			defer outFile.Close()

			_, err = io.Copy(outFile, rc)
			return err
		}
	}

	return fmt.Errorf("crush binary not found in zip archive")
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	// Copy file permissions
	sourceInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.Chmod(dst, sourceInfo.Mode())
}