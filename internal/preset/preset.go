package preset

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/bwl/cliffy/internal/config"
)

//go:embed presets/*.json
var presetsFS embed.FS

// Preset defines a curated configuration preset for common use cases
type Preset struct {
	// Metadata
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"` // "quality", "security", "refactoring", etc.

	// Model configuration
	Model           config.SelectedModelType `json:"model"`            // "large" or "small"
	MaxTokens       int64                    `json:"max_tokens,omitempty"`
	ReasoningEffort string                   `json:"reasoning_effort,omitempty"`
	Think           bool                     `json:"think,omitempty"`

	// Tool configuration
	AllowedTools []string `json:"allowed_tools,omitempty"` // If empty, allow all
	DisabledTools []string `json:"disabled_tools,omitempty"`

	// Context configuration
	ContextPaths    []string `json:"context_paths,omitempty"`
	SystemPromptPrefix string `json:"system_prompt_prefix,omitempty"`

	// Volley configuration
	MaxConcurrent int  `json:"max_concurrent,omitempty"`
	FailFast      bool `json:"fail_fast,omitempty"`

	// Examples
	Examples []string `json:"examples,omitempty"`
}

// Manager handles preset loading and application
type Manager struct {
	presets map[string]*Preset
}

// NewManager creates a new preset manager
func NewManager() (*Manager, error) {
	m := &Manager{
		presets: make(map[string]*Preset),
	}

	// Load all embedded presets
	if err := m.loadEmbeddedPresets(); err != nil {
		return nil, fmt.Errorf("failed to load presets: %w", err)
	}

	return m, nil
}

// loadEmbeddedPresets loads all preset files from the embedded filesystem
func (m *Manager) loadEmbeddedPresets() error {
	entries, err := presetsFS.ReadDir("presets")
	if err != nil {
		return fmt.Errorf("failed to read presets directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Read preset file
		data, err := presetsFS.ReadFile("presets/" + entry.Name())
		if err != nil {
			return fmt.Errorf("failed to read preset file %s: %w", entry.Name(), err)
		}

		// Parse preset
		var preset Preset
		if err := json.Unmarshal(data, &preset); err != nil {
			return fmt.Errorf("failed to parse preset file %s: %w", entry.Name(), err)
		}

		// Validate preset
		if preset.ID == "" {
			return fmt.Errorf("preset file %s has no ID", entry.Name())
		}

		m.presets[preset.ID] = &preset
	}

	return nil
}

// Get returns a preset by ID
func (m *Manager) Get(id string) (*Preset, error) {
	preset, ok := m.presets[id]
	if !ok {
		return nil, fmt.Errorf("preset not found: %s", id)
	}
	return preset, nil
}

// List returns all available presets sorted by category and name
func (m *Manager) List() []*Preset {
	presets := make([]*Preset, 0, len(m.presets))
	for _, preset := range m.presets {
		presets = append(presets, preset)
	}

	// Sort by category, then by name
	sort.Slice(presets, func(i, j int) bool {
		if presets[i].Category != presets[j].Category {
			return presets[i].Category < presets[j].Category
		}
		return presets[i].Name < presets[j].Name
	})

	return presets
}

// ListByCategory returns presets grouped by category
func (m *Manager) ListByCategory() map[string][]*Preset {
	result := make(map[string][]*Preset)

	for _, preset := range m.presets {
		category := preset.Category
		if category == "" {
			category = "general"
		}
		result[category] = append(result[category], preset)
	}

	// Sort presets within each category
	for _, presets := range result {
		sort.Slice(presets, func(i, j int) bool {
			return presets[i].Name < presets[j].Name
		})
	}

	return result
}

// ApplyToAgent applies a preset to an agent configuration
func (p *Preset) ApplyToAgent(agent *config.Agent) {
	// Apply model configuration
	agent.Model = p.Model

	// Apply tool configuration
	if len(p.AllowedTools) > 0 {
		agent.AllowedTools = p.AllowedTools
	}

	// Apply context paths if specified
	if len(p.ContextPaths) > 0 {
		agent.ContextPaths = p.ContextPaths
	}
}

// ApplyToOptions applies a preset to volley options
func (p *Preset) ApplyToOptions(opts *config.Options) {
	// Apply context paths
	if len(p.ContextPaths) > 0 {
		opts.ContextPaths = p.ContextPaths
	}

	// Apply disabled tools
	if len(p.DisabledTools) > 0 {
		opts.DisabledTools = p.DisabledTools
	}
}

// ApplyToSelectedModel applies a preset to a selected model
func (p *Preset) ApplyToSelectedModel(model *config.SelectedModel) {
	// Apply token limits
	if p.MaxTokens > 0 {
		model.MaxTokens = p.MaxTokens
	}

	// Apply reasoning effort
	if p.ReasoningEffort != "" {
		model.ReasoningEffort = p.ReasoningEffort
	}

	// Apply thinking mode
	if p.Think {
		model.Think = p.Think
	}
}
