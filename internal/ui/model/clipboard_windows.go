//go:build windows && !arm && !386 && !ios && !android

package model

import (
	"encoding/base64"
	"fmt"
	"os/exec"
	"strings"

	nativeclipboard "github.com/aymanbagabas/go-nativeclipboard"
)

func readClipboard(f clipboardFormat) ([]byte, error) {
	switch f {
	case clipboardFormatText:
		data, err := nativeclipboard.Text.Read()
		if err == nil && len(data) > 0 {
			return data, nil
		}
		return readWindowsClipboardText()
	case clipboardFormatImage:
		data, err := nativeclipboard.Image.Read()
		if err == nil && len(data) > 0 {
			return data, nil
		}
		return readWindowsClipboardImage()
	}
	return nil, errClipboardUnknownFormat
}

func readClipboardFileList() ([]string, error) {
	output, err := runWindowsClipboardScript(`
Add-Type -AssemblyName System.Windows.Forms
if (-not [System.Windows.Forms.Clipboard]::ContainsFileDropList()) { exit 17 }
$list = [System.Windows.Forms.Clipboard]::GetFileDropList()
foreach ($item in $list) {
	Write-Output $item
}
`)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.ReplaceAll(strings.TrimSpace(string(output)), "\r\n", "\n"), "\n")
	paths := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			paths = append(paths, line)
		}
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("clipboard does not contain file paths")
	}
	return paths, nil
}

func readWindowsClipboardText() ([]byte, error) {
	output, err := runWindowsClipboardScript(`
$text = Get-Clipboard -Raw
if ([string]::IsNullOrEmpty($text)) { exit 18 }
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
Write-Output $text
`)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(string(output))
	if text == "" {
		return nil, fmt.Errorf("clipboard does not contain text")
	}
	return []byte(text), nil
}

func readWindowsClipboardImage() ([]byte, error) {
	output, err := runWindowsClipboardScript(`
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
if (-not [System.Windows.Forms.Clipboard]::ContainsImage()) { exit 19 }
$image = [System.Windows.Forms.Clipboard]::GetImage()
if ($null -eq $image) { exit 20 }
$stream = New-Object System.IO.MemoryStream
$image.Save($stream, [System.Drawing.Imaging.ImageFormat]::Png)
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
Write-Output ([Convert]::ToBase64String($stream.ToArray()))
`)
	if err != nil {
		return nil, err
	}
	encoded := strings.TrimSpace(string(output))
	if encoded == "" {
		return nil, fmt.Errorf("clipboard does not contain image data")
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode clipboard image: %w", err)
	}
	return data, nil
}

func runWindowsClipboardScript(script string) ([]byte, error) {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-STA", "-Command", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(output))
		if trimmed == "" {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %s", err, trimmed)
	}
	return output, nil
}
