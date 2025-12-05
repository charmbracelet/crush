package util

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/atotto/clipboard"
)

var (
	isWSL     bool
	isWSLOnce sync.Once
)

// detectWSL checks if we're running in WSL environment.
func detectWSL() bool {
	if runtime.GOOS != "linux" {
		return false
	}

	// Check for WSL-specific environment variable.
	if os.Getenv("WSL_DISTRO_NAME") != "" {
		return true
	}

	// Check /proc/version for Microsoft/WSL indicators.
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}

	version := strings.ToLower(string(data))
	return strings.Contains(version, "microsoft") || strings.Contains(version, "wsl")
}

// IsWSL returns true if running in WSL environment.
func IsWSL() bool {
	isWSLOnce.Do(func() {
		isWSL = detectWSL()
	})
	return isWSL
}

// WriteClipboard writes text to the system clipboard.
// In WSL2, it uses PowerShell to correctly handle UTF-8 text.
func WriteClipboard(text string) error {
	if IsWSL() {
		return writeClipboardWSL(text)
	}
	return clipboard.WriteAll(text)
}

// writeClipboardWSL writes to clipboard using PowerShell in WSL environment.
// This correctly handles UTF-8 encoded text including CJK characters.
func writeClipboardWSL(text string) error {
	// Create a temporary file with UTF-8 content.
	tmpFile, err := os.CreateTemp("", "clipboard-*.txt")
	if err != nil {
		// Fall back to standard clipboard.
		return clipboard.WriteAll(text)
	}
	defer os.Remove(tmpFile.Name())

	// Write UTF-8 content to temp file.
	if _, err := tmpFile.WriteString(text); err != nil {
		tmpFile.Close()
		return clipboard.WriteAll(text)
	}
	tmpFile.Close()

	// Convert to Windows path.
	winPath, err := wslPathToWindows(tmpFile.Name())
	if err != nil {
		return clipboard.WriteAll(text)
	}

	// Use PowerShell to read UTF-8 file and set clipboard.
	psCmd := `Get-Content -Path '` + winPath + `' -Encoding UTF8 -Raw | Set-Clipboard`

	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", psCmd)
	if err := cmd.Run(); err != nil {
		// Fall back to standard clipboard if PowerShell fails.
		return clipboard.WriteAll(text)
	}

	return nil
}

// wslPathToWindows converts a WSL path to Windows path.
func wslPathToWindows(path string) (string, error) {
	// Try using wslpath command.
	cmd := exec.Command("wslpath", "-w", path)
	output, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(output)), nil
	}

	// Manual conversion as fallback.
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	// Convert /mnt/c/... to C:\...
	if strings.HasPrefix(absPath, "/mnt/") && len(absPath) > 6 {
		drive := strings.ToUpper(string(absPath[5]))
		rest := strings.ReplaceAll(absPath[6:], "/", "\\")
		return drive + ":" + rest, nil
	}

	// For paths in WSL filesystem, use \\wsl$\ path.
	distro := os.Getenv("WSL_DISTRO_NAME")
	if distro == "" {
		distro = "Ubuntu"
	}
	return `\\wsl$\` + distro + strings.ReplaceAll(absPath, "/", "\\"), nil
}
