package themes

import (
	_ "embed"
	"encoding/json"
	"sort"
	"sync"
)

//go:embed schemes.json
var schemesJSON []byte

var (
	catalogOnce sync.Once
	catalogList []Scheme
	catalogErr  error
)

func catalogSchemes() ([]Scheme, error) {
	catalogOnce.Do(func() {
		catalogList, catalogErr = parseSchemes(schemesJSON)
	})
	return catalogList, catalogErr
}

func first(m map[string]string, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != "" {
			return v
		}
	}
	return ""
}

// parseSchemes parses an iTerm2-Color-Schemes JSON array (Windows Terminal or
// iTerm2 key styles) into Scheme values, ordered by name.
func parseSchemes(data []byte) ([]Scheme, error) {
	var raw []map[string]string
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	schemes := make([]Scheme, 0, len(raw))
	for _, m := range raw {
		schemes = append(schemes, schemeFromMap(m))
	}

	sort.Slice(schemes, func(i, j int) bool {
		return schemes[i].Name < schemes[j].Name
	})
	return schemes, nil
}

// schemeFromMap builds a Scheme from a single iTerm2 JSON object, accepting
// both Windows Terminal and iTerm2 key styles.
func schemeFromMap(m map[string]string) Scheme {
	s := Scheme{
		Name:       m["name"],
		Background: parseHex(m["background"]),
		Foreground: parseHex(m["foreground"]),
	}
	s.ANSI[0] = parseHex(first(m, "black", "Black", "Ansi 0 Color"))
	s.ANSI[1] = parseHex(first(m, "red", "Red", "Ansi 1 Color"))
	s.ANSI[2] = parseHex(first(m, "green", "Green", "Ansi 2 Color"))
	s.ANSI[3] = parseHex(first(m, "yellow", "Yellow", "Ansi 3 Color"))
	s.ANSI[4] = parseHex(first(m, "blue", "Blue", "Ansi 4 Color"))
	s.ANSI[5] = parseHex(first(m, "purple", "magenta", "Purple", "Magenta", "Ansi 5 Color"))
	s.ANSI[6] = parseHex(first(m, "cyan", "Cyan", "Ansi 6 Color"))
	s.ANSI[7] = parseHex(first(m, "white", "White", "Ansi 7 Color"))
	s.ANSI[8] = parseHex(first(m, "brightBlack", "Bright Black", "Ansi 8 Color"))
	s.ANSI[9] = parseHex(first(m, "brightRed", "Bright Red", "Ansi 9 Color"))
	s.ANSI[10] = parseHex(first(m, "brightGreen", "Bright Green", "Ansi 10 Color"))
	s.ANSI[11] = parseHex(first(m, "brightYellow", "Bright Yellow", "Ansi 11 Color"))
	s.ANSI[12] = parseHex(first(m, "brightBlue", "Bright Blue", "Ansi 12 Color"))
	s.ANSI[13] = parseHex(first(m, "brightPurple", "brightMagenta", "Bright Purple", "Bright Magenta", "Ansi 13 Color"))
	s.ANSI[14] = parseHex(first(m, "brightCyan", "Bright Cyan", "Ansi 14 Color"))
	s.ANSI[15] = parseHex(first(m, "brightWhite", "Bright White", "Ansi 15 Color"))
	return s
}
