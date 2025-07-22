package protocol

import (
	"fmt"
	"log/slog"
)

// PatternInfo is an interface for types that represent glob patterns
type PatternInfo interface {
	GetPattern() string
	GetBasePath() string
	isPattern() // marker method
}

// StringPattern implements PatternInfo for string patterns
type StringPattern struct {
	Pattern string
}

func (p StringPattern) GetPattern() string  { return p.Pattern }
func (p StringPattern) GetBasePath() string { return "" }
func (p StringPattern) isPattern()          {}

// RelativePatternInfo implements PatternInfo for RelativePattern
type RelativePatternInfo struct {
	RP       RelativePattern
	BasePath string
}

func (p RelativePatternInfo) GetPattern() string  { return string(p.RP.Pattern) }
func (p RelativePatternInfo) GetBasePath() string { return p.BasePath }
func (p RelativePatternInfo) isPattern()          {}

// AsPattern converts GlobPattern to a PatternInfo object
func (g *GlobPattern) AsPattern() (PatternInfo, error) {
	if g.Value == nil {
		return nil, fmt.Errorf("nil pattern")
	}

	var err error

	switch v := g.Value.(type) {
	case string:
		return StringPattern{Pattern: v}, nil

	case RelativePattern:
		// Handle BaseURI which could be string or DocumentUri
		basePath := ""
		switch baseURI := v.BaseURI.Value.(type) {
		case string:
			basePath, err = DocumentURI(baseURI).Path()
			if err != nil {
				slog.Error("Failed to convert URI to path", "uri", baseURI, "error", err)
				return nil, fmt.Errorf("invalid URI: %s", baseURI)
			}

		case DocumentURI:
			basePath, err = baseURI.Path()
			if err != nil {
				slog.Error("Failed to convert DocumentURI to path", "uri", baseURI, "error", err)
				return nil, fmt.Errorf("invalid DocumentURI: %s", baseURI)
			}

		default:
			return nil, fmt.Errorf("unknown BaseURI type: %T", v.BaseURI.Value)
		}

		return RelativePatternInfo{RP: v, BasePath: basePath}, nil

	default:
		return nil, fmt.Errorf("unknown pattern type: %T", g.Value)
	}
}
