package spec

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// ChangeAnalyzer analyzes spec changes and their downstream impacts.
type ChangeAnalyzer struct {
	specPath    string
	specVersion string
}

// NewChangeAnalyzer creates a new change analyzer.
func NewChangeAnalyzer(specPath, specVersion string) *ChangeAnalyzer {
	return &ChangeAnalyzer{
		specPath:    specPath,
		specVersion: specVersion,
	}
}

// Analyze compares two specs and returns the changes with their impacts.
func (a *ChangeAnalyzer) Analyze(oldSpec, newSpec *Spec) []SpecChange {
	var changes []SpecChange

	// Compare basic fields
	changes = append(changes, a.compareBasicFields(oldSpec, newSpec)...)

	// Compare entities
	changes = append(changes, a.compareEntities(oldSpec, newSpec)...)

	// Compare API endpoints
	changes = append(changes, a.compareAPIEndpoints(oldSpec, newSpec)...)

	// Compare requirements
	changes = append(changes, a.compareRequirements(oldSpec, newSpec)...)

	// Set impact levels based on change type
	for i := range changes {
		changes[i].ImpactLevel = a.determineImpactLevel(changes[i])
	}

	return changes
}

func (a *ChangeAnalyzer) compareBasicFields(oldSpec, newSpec *Spec) []SpecChange {
	var changes []SpecChange

	if oldSpec.Description != newSpec.Description {
		changes = append(changes, SpecChange{
			Path:     "description",
			OldValue: oldSpec.Description,
			NewValue: newSpec.Description,
			Affected: []AffectedItem{
				{Type: "blueprint", Path: "README.md", Action: "regenerate"},
				{Type: "blueprint", Path: "architecture.md", Action: "regenerate"},
			},
		})
	}

	if !reflect.DeepEqual(oldSpec.Goals, newSpec.Goals) {
		changes = append(changes, SpecChange{
			Path:     "goals",
			OldValue: oldSpec.Goals,
			NewValue: newSpec.Goals,
			Affected: []AffectedItem{
				{Type: "blueprint", Path: "README.md", Action: "regenerate"},
			},
		})
	}

	if !reflect.DeepEqual(oldSpec.Constraints, newSpec.Constraints) {
		changes = append(changes, SpecChange{
			Path:     "constraints",
			OldValue: oldSpec.Constraints,
			NewValue: newSpec.Constraints,
			Affected: []AffectedItem{
				{Type: "blueprint", Path: "architecture.md", Action: "regenerate"},
				{Type: "construction", Path: "infrastructure/", Action: "regenerate"},
			},
		})
	}

	return changes
}

func (a *ChangeAnalyzer) compareEntities(oldSpec, newSpec *Spec) []SpecChange {
	var changes []SpecChange

	// Build maps for easier comparison
	oldEntities := make(map[string]Entity)
	for _, e := range oldSpec.Entities {
		oldEntities[e.Name] = e
	}
	newEntities := make(map[string]Entity)
	for _, e := range newSpec.Entities {
		newEntities[e.Name] = e
	}

	// Check for new entities
	for name, entity := range newEntities {
		if _, exists := oldEntities[name]; !exists {
			changes = append(changes, SpecChange{
				Path:     fmt.Sprintf("entities.%s", name),
				OldValue: nil,
				NewValue: entity,
				Affected: []AffectedItem{
					{Type: "blueprint", Path: "domain-model.md", Action: "regenerate"},
					{Type: "construction", Path: "prisma/schema.prisma", Action: "regenerate"},
				},
			})
		}
	}

	// Check for removed entities
	for name, entity := range oldEntities {
		if _, exists := newEntities[name]; !exists {
			changes = append(changes, SpecChange{
				Path:     fmt.Sprintf("entities.%s", name),
				OldValue: entity,
				NewValue: nil,
				Affected: []AffectedItem{
					{Type: "blueprint", Path: "domain-model.md", Action: "regenerate"},
					{Type: "construction", Path: "prisma/schema.prisma", Action: "regenerate"},
					{Type: "task", Path: "*", Action: "validate"},
				},
			})
		}
	}

	// Check for modified entities
	for name, newEntity := range newEntities {
		if oldEntity, exists := oldEntities[name]; exists {
			changes = append(changes, a.compareEntityFields(name, oldEntity, newEntity)...)
		}
	}

	return changes
}

func (a *ChangeAnalyzer) compareEntityFields(entityName string, oldEntity, newEntity Entity) []SpecChange {
	var changes []SpecChange

	// Build field maps
	oldFields := make(map[string]EntityField)
	for _, f := range oldEntity.Fields {
		oldFields[f.Name] = f
	}
	newFields := make(map[string]EntityField)
	for _, f := range newEntity.Fields {
		newFields[f.Name] = f
	}

	// Check for new fields
	for fieldName, field := range newFields {
		if _, exists := oldFields[fieldName]; !exists {
			changes = append(changes, SpecChange{
				Path:     fmt.Sprintf("entities.%s.fields.%s", entityName, fieldName),
				OldValue: nil,
				NewValue: field,
				Affected: []AffectedItem{
					{Type: "blueprint", Path: "domain-model.md", Action: "regenerate"},
					{Type: "construction", Path: "prisma/schema.prisma", Action: "regenerate"},
					{Type: "construction", Path: "api/openapi.yaml", Action: "regenerate"},
				},
			})
		}
	}

	// Check for removed fields
	for fieldName, field := range oldFields {
		if _, exists := newFields[fieldName]; !exists {
			changes = append(changes, SpecChange{
				Path:     fmt.Sprintf("entities.%s.fields.%s", entityName, fieldName),
				OldValue: field,
				NewValue: nil,
				Affected: []AffectedItem{
					{Type: "blueprint", Path: "domain-model.md", Action: "regenerate"},
					{Type: "construction", Path: "prisma/schema.prisma", Action: "regenerate"},
					{Type: "task", Path: "*", Action: "validate"},
				},
			})
		}
	}

	// Check for modified fields
	for fieldName, newField := range newFields {
		if oldField, exists := oldFields[fieldName]; exists {
			if oldField.Type != newField.Type {
				changes = append(changes, SpecChange{
					Path:     fmt.Sprintf("entities.%s.fields.%s.type", entityName, fieldName),
					OldValue: oldField.Type,
					NewValue: newField.Type,
					Affected: []AffectedItem{
						{Type: "construction", Path: "prisma/schema.prisma", Action: "regenerate"},
						{Type: "construction", Path: "api/openapi.yaml", Action: "regenerate"},
						{Type: "task", Path: "*", Action: "validate"},
					},
				})
			}
		}
	}

	return changes
}

func (a *ChangeAnalyzer) compareAPIEndpoints(oldSpec, newSpec *Spec) []SpecChange {
	var changes []SpecChange

	// Build maps for comparison
	oldEndpoints := make(map[string]APIEndpoint)
	for _, e := range oldSpec.APIEndpoints {
		key := fmt.Sprintf("%s %s", e.Method, e.Path)
		oldEndpoints[key] = e
	}
	newEndpoints := make(map[string]APIEndpoint)
	for _, e := range newSpec.APIEndpoints {
		key := fmt.Sprintf("%s %s", e.Method, e.Path)
		newEndpoints[key] = e
	}

	// Check for new endpoints
	for key, endpoint := range newEndpoints {
		if _, exists := oldEndpoints[key]; !exists {
			changes = append(changes, SpecChange{
				Path:     fmt.Sprintf("api_endpoints.%s", strings.ReplaceAll(key, " ", ".")),
				OldValue: nil,
				NewValue: endpoint,
				Affected: []AffectedItem{
					{Type: "blueprint", Path: "api-contracts.md", Action: "regenerate"},
					{Type: "construction", Path: "api/openapi.yaml", Action: "regenerate"},
				},
			})
		}
	}

	// Check for removed endpoints
	for key, endpoint := range oldEndpoints {
		if _, exists := newEndpoints[key]; !exists {
			changes = append(changes, SpecChange{
				Path:     fmt.Sprintf("api_endpoints.%s", strings.ReplaceAll(key, " ", ".")),
				OldValue: endpoint,
				NewValue: nil,
				Affected: []AffectedItem{
					{Type: "blueprint", Path: "api-contracts.md", Action: "regenerate"},
					{Type: "construction", Path: "api/openapi.yaml", Action: "regenerate"},
					{Type: "task", Path: "*", Action: "validate"},
				},
			})
		}
	}

	// Check for modified endpoints
	for key, newEndpoint := range newEndpoints {
		if oldEndpoint, exists := oldEndpoints[key]; exists {
			if !reflect.DeepEqual(oldEndpoint.Request, newEndpoint.Request) ||
				!reflect.DeepEqual(oldEndpoint.Response, newEndpoint.Response) {
				changes = append(changes, SpecChange{
					Path:     fmt.Sprintf("api_endpoints.%s", strings.ReplaceAll(key, " ", ".")),
					OldValue: oldEndpoint,
					NewValue: newEndpoint,
					Affected: []AffectedItem{
						{Type: "blueprint", Path: "api-contracts.md", Action: "regenerate"},
						{Type: "construction", Path: "api/openapi.yaml", Action: "regenerate"},
						{Type: "task", Path: "*", Action: "validate"},
					},
				})
			}
		}
	}

	return changes
}

func (a *ChangeAnalyzer) compareRequirements(oldSpec, newSpec *Spec) []SpecChange {
	var changes []SpecChange

	// Build requirement maps
	oldFuncReqs := make(map[string]FunctionalRequirement)
	for _, r := range oldSpec.Requirements.Functional {
		oldFuncReqs[r.ID] = r
	}
	newFuncReqs := make(map[string]FunctionalRequirement)
	for _, r := range newSpec.Requirements.Functional {
		newFuncReqs[r.ID] = r
	}

	// Check for new requirements
	for id, req := range newFuncReqs {
		if _, exists := oldFuncReqs[id]; !exists {
			changes = append(changes, SpecChange{
				Path:     fmt.Sprintf("requirements.functional.%s", id),
				OldValue: nil,
				NewValue: req,
				Affected: []AffectedItem{
					{Type: "blueprint", Path: "user-stories.md", Action: "regenerate"},
				},
			})
		}
	}

	// Check for modified requirements
	for id, newReq := range newFuncReqs {
		if oldReq, exists := oldFuncReqs[id]; exists {
			if !reflect.DeepEqual(oldReq.AcceptanceCriteria, newReq.AcceptanceCriteria) {
				changes = append(changes, SpecChange{
					Path:     fmt.Sprintf("requirements.functional.%s.acceptance_criteria", id),
					OldValue: oldReq.AcceptanceCriteria,
					NewValue: newReq.AcceptanceCriteria,
					Affected: []AffectedItem{
						{Type: "task", Path: id, Action: "validate"},
					},
				})
			}
		}
	}

	return changes
}

func (a *ChangeAnalyzer) determineImpactLevel(change SpecChange) ImpactLevel {
	path := change.Path

	// Breaking changes - removing things
	if change.NewValue == nil {
		return ImpactBreaking
	}

	// High impact - entity field type changes
	if strings.Contains(path, ".type") {
		return ImpactHigh
	}

	// High impact - constraint changes
	if strings.HasPrefix(path, "constraints.") {
		return ImpactHigh
	}

	// Medium impact - new fields or endpoints
	if strings.Contains(path, "entities.") && strings.Contains(path, ".fields.") {
		return ImpactMedium
	}
	if strings.HasPrefix(path, "api_endpoints.") {
		return ImpactMedium
	}

	// Low impact - description or goal changes
	if path == "description" || path == "goals" {
		return ImpactLow
	}

	return ImpactMedium
}

// Diff returns a human-readable diff of changes.
func (a *ChangeAnalyzer) Diff(changes []SpecChange) string {
	var sb strings.Builder

	for _, change := range changes {
		sb.WriteString(fmt.Sprintf("## %s\n", change.Path))
		sb.WriteString(fmt.Sprintf("  Impact: %s\n", change.ImpactLevel))
		sb.WriteString(fmt.Sprintf("  Old: %s\n", formatValue(change.OldValue)))
		sb.WriteString(fmt.Sprintf("  New: %s\n", formatValue(change.NewValue)))
		sb.WriteString("  Affected:\n")
		for _, affected := range change.Affected {
			sb.WriteString(fmt.Sprintf("    - %s: %s (%s)\n", affected.Type, affected.Path, affected.Action))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func formatValue(v interface{}) string {
	if v == nil {
		return "(none)"
	}
	b, _ := json.Marshal(v)
	return string(b)
}
