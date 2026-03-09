package spec

import (
	"time"
)

// Spec represents the root spec sheet document.
type Spec struct {
	ID          string     `yaml:"id" json:"id"`
	Name        string     `yaml:"name" json:"name"`
	Version     string     `yaml:"version" json:"version"`
	Status      SpecStatus `yaml:"status" json:"status"`
	Owner       string     `yaml:"owner" json:"owner"`
	Created     Date       `yaml:"created" json:"created"`
	Description string     `yaml:"description" json:"description"`

	Goals     []string `yaml:"goals" json:"goals"`
	NonGoals  []string `yaml:"non_goals" json:"nonGoals"`
	UpdatedAt Date     `yaml:"updated_at,omitempty" json:"updatedAt,omitempty"`

	Constraints  Constraints   `yaml:"constraints" json:"constraints"`
	Requirements Requirements  `yaml:"requirements" json:"requirements"`
	Entities     []Entity      `yaml:"entities" json:"entities"`
	APIEndpoints []APIEndpoint `yaml:"api_endpoints" json:"apiEndpoints"`
	Dependencies Dependencies  `yaml:"dependencies" json:"dependencies"`
	Metadata     Metadata      `yaml:"metadata" json:"metadata"`
}

// SpecStatus represents the status of a spec.
type SpecStatus string

const (
	SpecStatusDraft     SpecStatus = "draft"
	SpecStatusActive    SpecStatus = "active"
	SpecStatusPaused    SpecStatus = "paused"
	SpecStatusCompleted SpecStatus = "completed"
	SpecStatusArchived  SpecStatus = "archived"
)

// Date is a custom date type for YAML/JSON unmarshaling.
type Date struct {
	time.Time
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (d *Date) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return err
	}
	d.Time = t
	return nil
}

// MarshalYAML implements yaml.Marshaler.
func (d Date) MarshalYAML() (interface{}, error) {
	return d.Time.Format("2006-01-02"), nil
}

// Constraints contains technical, business, and resource constraints.
type Constraints struct {
	Technical []string `yaml:"technical" json:"technical"`
	Business  []string `yaml:"business" json:"business"`
	Resources []string `yaml:"resources" json:"resources"`
}

// Requirements contains functional and non-functional requirements.
type Requirements struct {
	Functional    []FunctionalRequirement    `yaml:"functional" json:"functional"`
	NonFunctional []NonFunctionalRequirement `yaml:"non_functional" json:"nonFunctional"`
}

// FunctionalRequirement represents a functional requirement.
type FunctionalRequirement struct {
	ID                 string   `yaml:"id" json:"id"`
	Description        string   `yaml:"description" json:"description"`
	Priority           Priority `yaml:"priority" json:"priority"`
	Providers          []string `yaml:"providers,omitempty" json:"providers,omitempty"`
	AcceptanceCriteria []string `yaml:"acceptance_criteria" json:"acceptanceCriteria"`
}

// NonFunctionalRequirement represents a non-functional requirement.
type NonFunctionalRequirement struct {
	ID          string `yaml:"id" json:"id"`
	Category    string `yaml:"category" json:"category"`
	Description string `yaml:"description" json:"description"`
	Details     string `yaml:"details" json:"details"`
}

// Priority represents requirement priority.
type Priority string

const (
	PriorityP0 Priority = "P0" // Critical
	PriorityP1 Priority = "P1" // High
	PriorityP2 Priority = "P2" // Medium
	PriorityP3 Priority = "P3" // Low
)

// Entity represents a domain entity in the spec.
type Entity struct {
	Name        string        `yaml:"name" json:"name"`
	Description string        `yaml:"description" json:"description"`
	Fields      []EntityField `yaml:"fields" json:"fields"`
}

// EntityField represents a field in an entity.
type EntityField struct {
	Name        string   `yaml:"name" json:"name"`
	Type        string   `yaml:"type" json:"type"`
	Description string   `yaml:"description" json:"description"`
	Values      []string `yaml:"values,omitempty" json:"values,omitempty"`
	References  string   `yaml:"references,omitempty" json:"references,omitempty"`
}

// APIEndpoint represents an API endpoint definition.
type APIEndpoint struct {
	Path        string            `yaml:"path" json:"path"`
	Method      string            `yaml:"method" json:"method"`
	Requirement string            `yaml:"requirement" json:"requirement"`
	Description string            `yaml:"description" json:"description"`
	Request     map[string]string `yaml:"request" json:"request"`
	Response    map[string]string `yaml:"response" json:"response"`
	Errors      []APIError        `yaml:"errors" json:"errors"`
	Providers   []string          `yaml:"providers,omitempty" json:"providers,omitempty"`
}

// APIError represents an API error response.
type APIError struct {
	Code    string `yaml:"code" json:"code"`
	Message string `yaml:"message" json:"message"`
}

// Dependencies contains internal and external dependencies.
type Dependencies struct {
	Internal []string `yaml:"internal" json:"internal"`
	External []string `yaml:"external" json:"external"`
}

// Metadata contains spec metadata.
type Metadata struct {
	CreatedBy   string `yaml:"created_by" json:"createdBy"`
	ReviewedBy  string `yaml:"reviewed_by,omitempty" json:"reviewedBy,omitempty"`
	Approved    bool   `yaml:"approved" json:"approved"`
	LastUpdated string `yaml:"last_updated,omitempty" json:"lastUpdated,omitempty"`
}

// SpecChange represents a change to a spec.
type SpecChange struct {
	Path        string         `yaml:"path" json:"path"`
	OldValue    interface{}    `yaml:"old_value" json:"oldValue"`
	NewValue    interface{}    `yaml:"new_value" json:"newValue"`
	ImpactLevel ImpactLevel    `yaml:"impact_level" json:"impactLevel"`
	Affected    []AffectedItem `yaml:"affected" json:"affected"`
}

// ImpactLevel represents the severity of a spec change.
type ImpactLevel string

const (
	ImpactLow      ImpactLevel = "low"
	ImpactMedium   ImpactLevel = "medium"
	ImpactHigh     ImpactLevel = "high"
	ImpactBreaking ImpactLevel = "breaking"
)

// AffectedItem represents an item affected by a spec change.
type AffectedItem struct {
	Type   string `yaml:"type" json:"type"`     // blueprint, construction, task
	Path   string `yaml:"path" json:"path"`     // File path or task ID
	Field  string `yaml:"field" json:"field"`   // Specific field affected
	Action string `yaml:"action" json:"action"` // update, regenerate, rework, validate
}
