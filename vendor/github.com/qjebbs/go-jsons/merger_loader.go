package jsons

import (
	"fmt"
	"strings"
)

// RegisterLoader register a new format loader.
func (m *Merger) RegisterLoader(name Format, extensions []string, fn LoadFunc) error {
	if name == FormatAuto {
		return fmt.Errorf("cannot register with reserved name: '%s'", FormatAuto)
	}
	if old, found := m.loadersByName[name]; found {
		for _, format := range old.Extensions {
			delete(m.loadersByExt, format)
		}
	}
	loader := newLoader(name, extensions, fn)
	m.loadersByName[name] = loader
	for _, ext := range extensions {
		lext := strings.ToLower(ext)
		if f, found := m.loadersByExt[lext]; found {
			return fmt.Errorf("file extension '%s' is already registered to '%s'", ext, f.Name)
		}
		m.loadersByExt[lext] = loader
	}
	return nil
}
