package tools

import "strings"

type ToolExposure string

const (
	ToolSearchToolName = "tool_search"

	ToolExposureDefault  ToolExposure = "default"
	ToolExposureDeferred ToolExposure = "deferred"
)

type ToolMetadata struct {
	ReadOnly        bool         `json:"read_only,omitempty"`
	ConcurrencySafe bool         `json:"concurrency_safe,omitempty"`
	RiskHint        string       `json:"risk_hint,omitempty"`
	SearchHint      string       `json:"search_hint,omitempty"`
	SearchTags      []string     `json:"search_tags,omitempty"`
	Exposure        ToolExposure `json:"exposure,omitempty"`
	Direct          bool         `json:"direct,omitempty"`
}

func (m ToolMetadata) Normalized(defaultParallel bool) ToolMetadata {
	if m.Exposure == "" {
		m.Exposure = ToolExposureDefault
	}
	if !m.ConcurrencySafe {
		m.ConcurrencySafe = defaultParallel
	}
	return m
}

func (m ToolMetadata) IsDeferred() bool {
	return m.Exposure == ToolExposureDeferred
}

func (m ToolMetadata) SearchTerms(name, description string) []string {
	terms := []string{name, description, m.SearchHint, m.RiskHint, string(m.Exposure)}
	terms = append(terms, m.SearchTags...)
	filtered := make([]string, 0, len(terms))
	for _, term := range terms {
		term = strings.TrimSpace(term)
		if term == "" {
			continue
		}
		filtered = append(filtered, strings.ToLower(term))
	}
	return filtered
}
