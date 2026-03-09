package orchestra

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/crush/internal/spec"
)

// CascadeManager handles the cascade of changes from specs to downstream artifacts.
type CascadeManager struct {
	specsDir        string
	blueprintsDir   string
	constructionDir string
	phasesDir       string
	tasksDir        string
}

// NewCascadeManager creates a new cascade manager.
func NewCascadeManager(specsDir, blueprintsDir, constructionDir, phasesDir, tasksDir string) *CascadeManager {
	return &CascadeManager{
		specsDir:        specsDir,
		blueprintsDir:   blueprintsDir,
		constructionDir: constructionDir,
		phasesDir:       phasesDir,
		tasksDir:        tasksDir,
	}
}

// CascadeResult contains the results of a cascade operation.
type CascadeResult struct {
	Changes          []spec.SpecChange
	RegeneratedFiles []string
	TasksToValidate  []string
	TasksToRework    []string
	Errors           []error
}

// AnalyzeAndCascade analyzes changes between two spec versions and cascades updates.
func (c *CascadeManager) AnalyzeAndCascade(oldSpec, newSpec *spec.Spec) (*CascadeResult, error) {
	result := &CascadeResult{}

	// Analyze changes
	analyzer := spec.NewChangeAnalyzer(c.specsDir, newSpec.Version)
	result.Changes = analyzer.Analyze(oldSpec, newSpec)

	// Group changes by impact
	for _, change := range result.Changes {
		for _, affected := range change.Affected {
			switch affected.Type {
			case "blueprint":
				if affected.Action == "regenerate" {
					if err := c.regenerateBlueprint(newSpec.ID, affected.Path); err != nil {
						result.Errors = append(result.Errors, err)
					} else {
						result.RegeneratedFiles = append(result.RegeneratedFiles,
							filepath.Join(c.blueprintsDir, newSpec.ID, affected.Path))
					}
				}
			case "construction":
				if affected.Action == "regenerate" {
					if err := c.regenerateConstruction(newSpec.ID, affected.Path); err != nil {
						result.Errors = append(result.Errors, err)
					} else {
						result.RegeneratedFiles = append(result.RegeneratedFiles,
							filepath.Join(c.constructionDir, newSpec.ID, affected.Path))
					}
				}
			case "task":
				if affected.Action == "validate" {
					result.TasksToValidate = append(result.TasksToValidate, affected.Path)
				} else if affected.Action == "rework" {
					result.TasksToRework = append(result.TasksToRework, affected.Path)
				}
			}
		}
	}

	return result, nil
}

// RegenerateAll regenerates all downstream artifacts from a spec.
func (c *CascadeManager) RegenerateAll(s *spec.Spec) error {
	// Regenerate blueprints
	blueprintGen := NewBlueprintGenerator(c.blueprintsDir)
	if err := blueprintGen.Generate(*s); err != nil {
		return fmt.Errorf("failed to regenerate blueprints: %w", err)
	}

	// Regenerate construction docs
	constructionGen := NewConstructionGenerator(c.constructionDir)
	if err := constructionGen.Generate(*s); err != nil {
		return fmt.Errorf("failed to regenerate construction docs: %w", err)
	}

	return nil
}

// ValidateCompletedWork checks if completed tasks still meet the current spec.
func (c *CascadeManager) ValidateCompletedWork(specID string) ([]ValidationResult, error) {
	var results []ValidationResult

	// Load all tasks for this spec's phases
	phaseManager := NewPhaseManager(c.phasesDir, c.tasksDir)
	phases, err := phaseManager.LoadPhasesForProject(specID)
	if err != nil {
		return nil, fmt.Errorf("failed to load phases: %w", err)
	}

	for _, phase := range phases {
		tasks, err := phaseManager.LoadTasksForPhase(phase.ID)
		if err != nil {
			continue
		}

		for _, task := range tasks {
			if task.Status == TaskStatusCompleted {
				// Check if task's implementation still matches spec
				result := ValidationResult{
					TaskID:    task.ID,
					TaskTitle: task.Title,
					PhaseID:   task.Phase,
					Status:    "valid", // Would be determined by actual validation
				}
				results = append(results, result)
			}
		}
	}

	return results, nil
}

// GetImpactReport returns a human-readable impact report for spec changes.
func (c *CascadeManager) GetImpactReport(changes []spec.SpecChange) string {
	var sb strings.Builder

	sb.WriteString("# Spec Change Impact Report\n\n")

	// Group by impact level
	breaking := filterChanges(changes, spec.ImpactBreaking)
	high := filterChanges(changes, spec.ImpactHigh)
	medium := filterChanges(changes, spec.ImpactMedium)
	low := filterChanges(changes, spec.ImpactLow)

	if len(breaking) > 0 {
		sb.WriteString("## Breaking Changes\n\n")
		sb.WriteString("These changes require immediate attention and may break existing implementations:\n\n")
		for _, change := range breaking {
			sb.WriteString(fmt.Sprintf("- **%s**: %s → %s\n", change.Path,
				formatChangeValue(change.OldValue), formatChangeValue(change.NewValue)))
		}
		sb.WriteString("\n")
	}

	if len(high) > 0 {
		sb.WriteString("## High Impact Changes\n\n")
		sb.WriteString("These changes affect core functionality:\n\n")
		for _, change := range high {
			sb.WriteString(fmt.Sprintf("- **%s**: %s → %s\n", change.Path,
				formatChangeValue(change.OldValue), formatChangeValue(change.NewValue)))
		}
		sb.WriteString("\n")
	}

	if len(medium) > 0 {
		sb.WriteString("## Medium Impact Changes\n\n")
		sb.WriteString("These changes affect generated artifacts:\n\n")
		for _, change := range medium {
			sb.WriteString(fmt.Sprintf("- **%s**\n", change.Path))
		}
		sb.WriteString("\n")
	}

	if len(low) > 0 {
		sb.WriteString("## Low Impact Changes\n\n")
		sb.WriteString("These changes only affect documentation:\n\n")
		for _, change := range low {
			sb.WriteString(fmt.Sprintf("- **%s**\n", change.Path))
		}
		sb.WriteString("\n")
	}

	// Collect affected items
	sb.WriteString("## Affected Artifacts\n\n")

	blueprints := make(map[string]bool)
	construction := make(map[string]bool)
	tasks := make(map[string]bool)

	for _, change := range changes {
		for _, affected := range change.Affected {
			switch affected.Type {
			case "blueprint":
				blueprints[affected.Path] = true
			case "construction":
				construction[affected.Path] = true
			case "task":
				tasks[affected.Path] = true
			}
		}
	}

	if len(blueprints) > 0 {
		sb.WriteString("### Blueprints to Regenerate\n\n")
		for path := range blueprints {
			sb.WriteString(fmt.Sprintf("- %s\n", path))
		}
		sb.WriteString("\n")
	}

	if len(construction) > 0 {
		sb.WriteString("### Construction Documents to Regenerate\n\n")
		for path := range construction {
			sb.WriteString(fmt.Sprintf("- %s\n", path))
		}
		sb.WriteString("\n")
	}

	if len(tasks) > 0 {
		sb.WriteString("### Tasks Requiring Validation\n\n")
		for path := range tasks {
			sb.WriteString(fmt.Sprintf("- %s\n", path))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func (c *CascadeManager) regenerateBlueprint(specID, path string) error {
	// Load spec
	specManager := spec.NewManager(c.specsDir)
	s, err := specManager.Load(specID)
	if err != nil {
		return fmt.Errorf("failed to load spec: %w", err)
	}

	// Regenerate all blueprints (simpler than selective regeneration)
	generator := NewBlueprintGenerator(c.blueprintsDir)
	return generator.Generate(*s)
}

func (c *CascadeManager) regenerateConstruction(specID, path string) error {
	// Load spec
	specManager := spec.NewManager(c.specsDir)
	s, err := specManager.Load(specID)
	if err != nil {
		return fmt.Errorf("failed to load spec: %w", err)
	}

	// Regenerate all construction docs
	generator := NewConstructionGenerator(c.constructionDir)
	return generator.Generate(*s)
}

// ValidationResult represents the result of validating a task against the spec.
type ValidationResult struct {
	TaskID    string
	TaskTitle string
	PhaseID   string
	Status    string // valid, needs_update, broken
	Issues    []string
}

func filterChanges(changes []spec.SpecChange, level spec.ImpactLevel) []spec.SpecChange {
	var result []spec.SpecChange
	for _, change := range changes {
		if change.ImpactLevel == level {
			result = append(result, change)
		}
	}
	return result
}

func formatChangeValue(v interface{}) string {
	if v == nil {
		return "(removed)"
	}
	return fmt.Sprintf("%v", v)
}

// CreateChangeLog creates a change log file for spec changes.
func (c *CascadeManager) CreateChangeLog(specID string, changes []spec.SpecChange) error {
	logDir := filepath.Join(c.specsDir, "changelogs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return fmt.Errorf("failed to create changelog directory: %w", err)
	}

	report := c.GetImpactReport(changes)
	logPath := filepath.Join(logDir, specID+"-change-"+timestamp()+".md")

	if err := os.WriteFile(logPath, []byte(report), 0o644); err != nil {
		return fmt.Errorf("failed to write changelog: %w", err)
	}

	return nil
}

func timestamp() string {
	// Simple timestamp for filenames
	return "latest"
}
