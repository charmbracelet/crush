package themes

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

func userThemesDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "crush", "themes"), nil
}

var (
	userSchemesMu  sync.Mutex
	userSchemesDir string
	userSchemesVal []Scheme
)

// userSchemes discovers user-authored themes from the user themes directory,
// caching the result per directory. Each *.json file is a single iTerm2-format
// scheme object whose theme name is the filename minus the extension.
// Malformed or unreadable files are skipped and logged; a missing directory is
// not an error.
func userSchemes() []Scheme {
	dir, err := userThemesDir()
	if err != nil {
		return nil
	}

	userSchemesMu.Lock()
	defer userSchemesMu.Unlock()
	if dir == userSchemesDir {
		return userSchemesVal
	}
	userSchemesDir = dir
	userSchemesVal = loadUserSchemes(dir)
	return userSchemesVal
}

func loadUserSchemes(dir string) []Scheme {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var schemes []Scheme
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			slog.Warn("Failed to read theme file", "file", path, "error", err)
			continue
		}
		var raw map[string]string
		if err := json.Unmarshal(data, &raw); err != nil {
			slog.Warn("Failed to parse theme file", "file", path, "error", err)
			continue
		}
		s := schemeFromMap(raw)
		s.Name = strings.TrimSuffix(e.Name(), ".json")
		schemes = append(schemes, s)
	}

	sort.Slice(schemes, func(i, j int) bool {
		return schemes[i].Name < schemes[j].Name
	})
	return schemes
}
