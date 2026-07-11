package memory

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

type ImportResult struct {
	Imported int
	Skipped  int
}

type jsonlEntity struct {
	Type         string   `json:"type"`
	Name         string   `json:"name"`
	EntityType   string   `json:"entityType"`
	Observations []string `json:"observations"`
}

func (s *Store) ImportJSONL(ctx context.Context, reader io.Reader, project Project) (ImportResult, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	var result ImportResult
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entity jsonlEntity
		if err := json.Unmarshal([]byte(line), &entity); err != nil {
			result.Skipped++
			continue
		}
		if !strings.EqualFold(entity.Type, "entity") || len(entity.Observations) == 0 {
			result.Skipped++
			continue
		}
		kind, scope := classifyLegacyEntity(entity.EntityType)
		for _, content := range entity.Observations {
			observation := Observation{
				Scope:       scope,
				Kind:        kind,
				Name:        entity.Name,
				Description: cleanLine(content, 180),
				Content:     content,
				Confidence:  0.80,
				SourceKind:  "jsonl-import",
				ObservedAt:  time.Now(),
				Status:      StatusActive,
			}
			if _, err := s.SaveObservation(ctx, project, observation); err != nil {
				if errors.Is(err, ErrRejected) {
					result.Skipped++
					continue
				}
				return result, fmt.Errorf("import memory entity %q: %w", entity.Name, err)
			}
			result.Imported++
		}
	}
	return result, scanner.Err()
}

func classifyLegacyEntity(entityType string) (Kind, Scope) {
	value := strings.ToLower(entityType)
	switch {
	case strings.Contains(value, "user"), strings.Contains(value, "person"):
		return KindUser, ScopeGlobal
	case strings.Contains(value, "feedback"), strings.Contains(value, "preference"), strings.Contains(value, "rule"):
		return KindFeedback, ScopeGlobal
	case strings.Contains(value, "project"):
		return KindProject, ScopeProject
	default:
		return KindReference, ScopeGlobal
	}
}
