package jsons

import (
	"fmt"
)

// Extensions get supported extensions of given format.
// If format is empty or FormatAuto, it returns all extensions.
func (m *Merger) Extensions(formatName Format) ([]string, error) {
	if formatName == "" || formatName == FormatAuto {
		return m.getAllExtensions(), nil
	}
	f, found := m.loadersByName[formatName]
	if !found {
		return nil, fmt.Errorf("%s not found", formatName)
	}
	return f.Extensions, nil
}

// getAllExtensions get all extensions supported
func (m *Merger) getAllExtensions() []string {
	extensions := make([]string, 0)
	for ext := range m.loadersByExt {
		extensions = append(extensions, ext)
	}
	return extensions
}
