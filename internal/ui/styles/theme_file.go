package styles

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/crush/internal/home"
)

// ThemeFile represents a standalone theme definition stored as JSON.
// It wraps a Palette with an optional base theme reference. Missing
// palette fields inherit from the base.
type ThemeFile struct {
	Base string `json:"base,omitempty"`
	Palette
}

// knownThemeFields is the set of valid JSON field names in a theme file.
// Derived from PaletteFields at init time so it stays in sync with the
// Palette struct automatically.
var knownThemeFields map[string]bool

func init() {
	knownThemeFields = make(map[string]bool, len(PaletteFields())+1)
	knownThemeFields["base"] = true
	for _, f := range PaletteFields() {
		knownThemeFields[f.Name] = true
	}
}

// LoadThemeFile reads and validates a theme file from disk. Unknown
// fields are logged as warnings but otherwise ignored. Returns an error
// if the file cannot be read or contains invalid color values.
func LoadThemeFile(path string) (*ThemeFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read theme file: %w", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse theme file: %w", err)
	}

	for key := range raw {
		if !knownThemeFields[key] {
			slog.Warn("Unknown field in theme file", "field", key, "path", path)
		}
	}

	var tf ThemeFile
	if err := json.Unmarshal(data, &tf); err != nil {
		return nil, fmt.Errorf("decode theme file: %w", err)
	}

	if err := tf.Palette.Validate(); err != nil {
		return nil, fmt.Errorf("theme file %s: %w", filepath.Base(path), err)
	}

	return &tf, nil
}

// SaveThemeFile writes a theme file to disk with indented JSON. Parent
// directories are created automatically.
func SaveThemeFile(path string, tf *ThemeFile) error {
	data, err := json.MarshalIndent(tf, "", "  ")
	if err != nil {
		return fmt.Errorf("encode theme file: %w", err)
	}
	data = append(data, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create theme directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write theme file: %w", err)
	}
	return nil
}

// themeDirsOverride allows tests to replace the default theme directories.
var themeDirsOverride []string

// ThemeDirs returns the theme search paths in priority order. The first
// directory that contains a matching theme file wins.
//
//   - Project-local: ./.crush/themes/
//   - User global: ~/.config/crush/themes/ (or platform equivalent)
func ThemeDirs() []string {
	if themeDirsOverride != nil {
		return themeDirsOverride
	}

	dirs := make([]string, 0, 2)

	// Project-local themes.
	dirs = append(dirs, filepath.Join(".crush", "themes"))

	// User global themes.
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
		}
		dirs = append(dirs, filepath.Join(localAppData, "crush", "themes"))
	} else {
		dirs = append(dirs, filepath.Join(home.Config(), "crush", "themes"))
	}

	return dirs
}

// FindThemeFile locates a theme file by name across all theme
// directories. Returns the full path to the first match, or an error if
// no file is found.
func FindThemeFile(name string) (string, error) {
	filename := strings.ToLower(name) + ".json"
	for _, dir := range ThemeDirs() {
		path := filepath.Join(dir, filename)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("theme file %q not found in %v", name, ThemeDirs())
}

// ListUserThemes returns the names of all user-defined themes found
// across all theme directories. Names are lowercased and deduplicated;
// earlier directories take priority.
func ListUserThemes() ([]string, error) {
	seen := make(map[string]bool)
	var names []string

	for _, dir := range ThemeDirs() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("list themes in %s: %w", dir, err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".json")
			lower := strings.ToLower(name)
			if !seen[lower] {
				seen[lower] = true
				names = append(names, lower)
			}
		}
	}
	return names, nil
}
