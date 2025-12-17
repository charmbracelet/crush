package util

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/atotto/clipboard"
	"golang.org/x/text/encoding/unicode"
)

// WriteClipboard writes text to the system clipboard.
// On Windows/WSL we ensure UTF-16 content is used so the Windows clipboard
// receives the correct encoding.
func WriteClipboard(text string) error {
	switch {
	case IsWindows():
		return writeClipboardWindows(text)
	case IsWSL():
		return writeClipboardWSL(text)
	default:
		return clipboard.WriteAll(text)
	}
}

// writeClipboardWindows ensures the text can be represented in UTF-16 before
// delegating to the native clipboard implementation.
func writeClipboardWindows(text string) error {
	if _, err := encodeUTF16LE(text); err != nil {
		return err
	}
	return clipboard.WriteAll(text)
}

// writeClipboardWSL writes to clipboard using PowerShell in WSL environment.
// This uses UTF-16 encoded content to match the Windows clipboard expectation.
func writeClipboardWSL(text string) error {
	encoded, err := encodeUTF16LEWithBOM(text)
	if err != nil {
		return clipboard.WriteAll(text)
	}

	tmpFile, err := os.CreateTemp("", "clipboard-*.txt")
	if err != nil {
		// Fall back to standard clipboard.
		return clipboard.WriteAll(text)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(encoded); err != nil {
		tmpFile.Close()
		return clipboard.WriteAll(text)
	}
	tmpFile.Close()

	// Convert to Windows path.
	winPath, err := wslPathToWindows(tmpFile.Name())
	if err != nil {
		return clipboard.WriteAll(text)
	}

	// Use PowerShell to read UTF-16 file and set clipboard.
	psCmd := `Get-Content -Path '` + winPath + `' -Encoding Unicode -Raw | Set-Clipboard`

	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", psCmd)
	if err := cmd.Run(); err != nil {
		// Fall back to standard clipboard if PowerShell fails.
		return clipboard.WriteAll(text)
	}

	return nil
}

// encodeUTF16LEWithBOM encodes the provided text to UTF-16LE including BOM.
func encodeUTF16LEWithBOM(text string) ([]byte, error) {
	enc := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM)
	return enc.NewEncoder().Bytes([]byte(text))
}

// encodeUTF16LE encodes text to UTF-16LE without BOM.
func encodeUTF16LE(text string) ([]byte, error) {
	enc := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
	return enc.NewEncoder().Bytes([]byte(text))
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
