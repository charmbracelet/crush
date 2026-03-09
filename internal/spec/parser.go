package spec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Parser handles parsing and validation of spec files.
type Parser struct{}

// NewParser creates a new spec parser.
func NewParser() *Parser {
	return &Parser{}
}

// ParseFile parses a spec file from the given path.
func (p *Parser) ParseFile(path string) (*Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read spec file: %w", err)
	}

	return p.Parse(data)
}

// Parse parses a spec from YAML data.
func (p *Parser) Parse(data []byte) (*Spec, error) {
	var spec Spec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse spec YAML: %w", err)
	}

	if err := p.Validate(&spec); err != nil {
		return nil, fmt.Errorf("spec validation failed: %w", err)
	}

	return &spec, nil
}

// Validate validates a spec for required fields and consistency.
func (p *Parser) Validate(spec *Spec) error {
	var errors []string

	if spec.ID == "" {
		errors = append(errors, "spec ID is required")
	}
	if spec.Name == "" {
		errors = append(errors, "spec name is required")
	}
	if spec.Version == "" {
		errors = append(errors, "spec version is required")
	}
	if spec.Status == "" {
		spec.Status = SpecStatusDraft
	}
	if spec.Description == "" {
		errors = append(errors, "spec description is required")
	}

	// Validate entities have required fields
	for i, entity := range spec.Entities {
		if entity.Name == "" {
			errors = append(errors, fmt.Sprintf("entity[%d] name is required", i))
		}
		for j, field := range entity.Fields {
			if field.Name == "" {
				errors = append(errors, fmt.Sprintf("entity[%d].fields[%d] name is required", i, j))
			}
			if field.Type == "" {
				errors = append(errors, fmt.Sprintf("entity[%d].fields[%d] type is required", i, j))
			}
		}
	}

	// Validate API endpoints
	for i, endpoint := range spec.APIEndpoints {
		if endpoint.Path == "" {
			errors = append(errors, fmt.Sprintf("api_endpoints[%d] path is required", i))
		}
		if endpoint.Method == "" {
			errors = append(errors, fmt.Sprintf("api_endpoints[%d] method is required", i))
		}
	}

	// Validate functional requirements
	for i, req := range spec.Requirements.Functional {
		if req.ID == "" {
			errors = append(errors, fmt.Sprintf("requirements.functional[%d] ID is required", i))
		}
		if req.Description == "" {
			errors = append(errors, fmt.Sprintf("requirements.functional[%d] description is required", i))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation errors:\n  - %s", strings.Join(errors, "\n  - "))
	}

	return nil
}

// Manager handles spec file operations.
type Manager struct {
	parser   *Parser
	specsDir string
}

// NewManager creates a new spec manager.
func NewManager(specsDir string) *Manager {
	return &Manager{
		parser:   NewParser(),
		specsDir: specsDir,
	}
}

// Load loads a spec by name.
func (m *Manager) Load(name string) (*Spec, error) {
	path := filepath.Join(m.specsDir, name+".yaml")
	return m.parser.ParseFile(path)
}

// LoadAll loads all specs in the specs directory.
func (m *Manager) LoadAll() ([]*Spec, error) {
	entries, err := os.ReadDir(m.specsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read specs directory: %w", err)
	}

	var specs []*Spec
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		path := filepath.Join(m.specsDir, entry.Name())
		spec, err := m.parser.ParseFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", entry.Name(), err)
		}
		specs = append(specs, spec)
	}

	return specs, nil
}

// Save saves a spec to a file.
func (m *Manager) Save(spec *Spec) error {
	if err := os.MkdirAll(m.specsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create specs directory: %w", err)
	}

	data, err := yaml.Marshal(spec)
	if err != nil {
		return fmt.Errorf("failed to marshal spec: %w", err)
	}

	path := filepath.Join(m.specsDir, spec.ID+".yaml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write spec file: %w", err)
	}

	return nil
}

// Create creates a new spec with default values.
func (m *Manager) Create(name, description string) (*Spec, error) {
	spec := &Spec{
		ID:           generateSpecID(name),
		Name:         name,
		Version:      "1.0.0",
		Status:       SpecStatusDraft,
		Description:  description,
		Created:      Date{Time: timeNow()},
		Goals:        []string{},
		NonGoals:     []string{},
		Entities:     []Entity{},
		APIEndpoints: []APIEndpoint{},
	}

	if err := m.Save(spec); err != nil {
		return nil, err
	}

	return spec, nil
}

// generateSpecID generates a spec ID from a name.
func generateSpecID(name string) string {
	// Convert to lowercase and replace spaces with hyphens
	id := strings.ToLower(name)
	id = strings.ReplaceAll(id, " ", "-")
	// Remove non-alphanumeric characters except hyphens
	var result strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	// Collapse consecutive hyphens into a single hyphen
	id = result.String()
	for strings.Contains(id, "--") {
		id = strings.ReplaceAll(id, "--", "-")
	}
	return id
}

// timeNow is a variable for testing.
var timeNow = func() time.Time {
	return time.Now()
}
