package tools

import (
	"encoding/json"
	"strings"
)

func SearchRegistryEntries(entries []RegistryEntry, query string, opts RegistrySearchOptions) []RegistryEntry {
	limit := opts.Limit
	if limit <= 0 {
		limit = 10
	}

	query = strings.TrimSpace(strings.ToLower(query))
	matches := make([]RegistryEntry, 0, len(entries))
	for _, entry := range entries {
		if opts.ExposedOnly && !entry.Exposed {
			continue
		}
		if !opts.IncludeDeferred && entry.Metadata.IsDeferred() {
			continue
		}
		if query == "" || registryEntryMatches(entry, query) {
			matches = append(matches, entry)
			if len(matches) == limit {
				break
			}
		}
	}
	return matches
}

func registryEntryMatches(entry RegistryEntry, query string) bool {
	for _, term := range entry.Metadata.SearchTerms(entry.Name, entry.Description) {
		if strings.Contains(term, query) {
			return true
		}
	}
	if strings.Contains(strings.ToLower(entry.Source), query) {
		return true
	}
	data, err := json.Marshal(entry.Parameters)
	if err == nil && strings.Contains(strings.ToLower(string(data)), query) {
		return true
	}
	return false
}
