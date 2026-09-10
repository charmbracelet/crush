package styles

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
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

	if err := tf.Validate(); err != nil {
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

// ThemeDirs returns the user theme directory. Theme files are global so
// launching Crush from different working directories always resolves the same
// themes.
func ThemeDirs() []string {
	if themeDirsOverride != nil {
		return themeDirsOverride
	}
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
		}
		return []string{filepath.Join(localAppData, "crush", "themes")}
	}
	return []string{filepath.Join(home.Config(), "crush", "themes")}
}

// ThemePath returns the path for a validated global user theme name.
func ThemePath(name string) (string, error) {
	name = strings.ToLower(name)
	if err := validateThemeNameFormat(name); err != nil {
		return "", err
	}
	return filepath.Join(ThemeDirs()[0], name+".json"), nil
}

// FindThemeFile locates a global theme file by name. Returns an error for an
// invalid name or when no matching file exists.
func FindThemeFile(name string) (string, error) {
	path, err := ThemePath(name)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("theme file %q not found", strings.ToLower(name))
}

// validThemeName matches safe theme names: lowercase alphanumerics
// separated by single hyphens or underscores.
var validThemeName = regexp.MustCompile(`^[a-z0-9]+([-_][a-z0-9]+)*$`)

func validateThemeNameFormat(name string) error {
	if name == "" {
		return errors.New("theme name cannot be empty")
	}
	if !validThemeName.MatchString(strings.ToLower(name)) {
		return errors.New("use lowercase letters, numbers, hyphens, and underscores only")
	}
	return nil
}

// ValidateThemeName reports whether name is usable as a theme file name.
// It rejects empty names, names with characters that are unsafe in file
// paths, and names that collide with an existing theme.
func ValidateThemeName(name string) error {
	if err := validateThemeNameFormat(name); err != nil {
		return err
	}
	if IsBuiltinTheme(name) {
		return fmt.Errorf("%q is a built-in theme; edit it instead", name)
	}
	if _, err := FindThemeFile(name); err == nil {
		return fmt.Errorf("theme %q already exists", name)
	}
	return nil
}

// ValidateThemeRename is like ValidateThemeName but allows renaming a
// theme to itself (case-insensitive). It still rejects builtins and
// collisions with other themes.
func ValidateThemeRename(oldName, newName string) error {
	if newName == "" {
		return errors.New("theme name cannot be empty")
	}
	lower := strings.ToLower(newName)
	if !validThemeName.MatchString(lower) {
		return errors.New("use lowercase letters, numbers, hyphens, and underscores only")
	}
	if IsBuiltinTheme(lower) {
		return fmt.Errorf("%q is a built-in theme", lower)
	}
	if lower != strings.ToLower(oldName) {
		if _, err := FindThemeFile(lower); err == nil {
			return fmt.Errorf("theme %q already exists", lower)
		}
	}
	return nil
}

// RenameThemeFile renames a user theme file on disk. It locates the
// existing file via FindThemeFile, validates the new name, and moves
// the file in the same directory. Returns an error if the source file
// is not found, the new name is invalid, or a theme with the new name
// already exists.
func RenameThemeFile(oldName, newName string) error {
	if err := ValidateThemeRename(oldName, newName); err != nil {
		return err
	}
	oldPath, err := FindThemeFile(oldName)
	if err != nil {
		return fmt.Errorf("rename theme: %w", err)
	}
	newPath := filepath.Join(filepath.Dir(oldPath), strings.ToLower(newName)+".json")
	if err := os.Rename(oldPath, newPath); err != nil {
		return fmt.Errorf("rename theme file: %w", err)
	}
	return nil
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
			if name != lower || validateThemeNameFormat(lower) != nil {
				slog.Warn("Ignoring invalid theme filename", "file", e.Name(), "directory", dir)
				continue
			}
			if !seen[lower] {
				seen[lower] = true
				names = append(names, lower)
			}
		}
	}
	return names, nil
}
