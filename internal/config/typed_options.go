package config

import (
	"encoding/json"
	"fmt"
)

// ProviderOptionType defines types of provider configurations
type ProviderOptionType string

const (
	OptionTypeString  ProviderOptionType = "string"
	OptionTypeInt     ProviderOptionType = "int"
	OptionTypeFloat   ProviderOptionType = "float"
	OptionTypeBool    ProviderOptionType = "bool"
	OptionTypeArray   ProviderOptionType = "array"
	OptionTypeObject  ProviderOptionType = "object"
)

// TypedOption represents a strongly-typed provider option
type TypedOption struct {
	Name         string           `json:"name"`
	Type         ProviderOptionType `json:"type"`
	Value        interface{}      `json:"value"`
	Description  string           `json:"description,omitempty"`
	Required     bool             `json:"required,omitempty"`
	DefaultValue interface{}      `json:"default,omitempty"`
}

// TypedProviderOptions provides type-safe configuration for providers
type TypedProviderOptions struct {
	Options map[string]TypedOption `json:"options"`
}

// NewTypedProviderOptions creates a new typed options container
func NewTypedProviderOptions() *TypedProviderOptions {
	return &TypedProviderOptions{
		Options: make(map[string]TypedOption),
	}
}

// AddString adds a string option with validation
func (tpo *TypedProviderOptions) AddString(name, description string, required bool, defaultValue string) {
	tpo.Options[name] = TypedOption{
		Name:         name,
		Type:         OptionTypeString,
		Value:        defaultValue,
		Description:  description,
		Required:     required,
		DefaultValue: defaultValue,
	}
}

// AddInt adds an integer option with validation
func (tpo *TypedProviderOptions) AddInt(name, description string, required bool, defaultValue int) {
	tpo.Options[name] = TypedOption{
		Name:         name,
		Type:         OptionTypeInt,
		Value:        defaultValue,
		Description:  description,
		Required:     required,
		DefaultValue: defaultValue,
	}
}

// AddFloat adds a float option with validation
func (tpo *TypedProviderOptions) AddFloat(name, description string, required bool, defaultValue float64) {
	tpo.Options[name] = TypedOption{
		Name:         name,
		Type:         OptionTypeFloat,
		Value:        defaultValue,
		Description:  description,
		Required:     required,
		DefaultValue: defaultValue,
	}
}

// AddBool adds a boolean option with validation
func (tpo *TypedProviderOptions) AddBool(name, description string, required bool, defaultValue bool) {
	tpo.Options[name] = TypedOption{
		Name:         name,
		Type:         OptionTypeBool,
		Value:        defaultValue,
		Description:  description,
		Required:     required,
		DefaultValue: defaultValue,
	}
}

// GetString safely retrieves a string option
func (tpo *TypedProviderOptions) GetString(name string) (string, error) {
	option, exists := tpo.Options[name]
	if !exists {
		return "", fmt.Errorf("option '%s' not found", name)
	}
	if option.Type != OptionTypeString {
		return "", fmt.Errorf("option '%s' is not a string, got %s", name, option.Type)
	}
	if value, ok := option.Value.(string); ok {
		return value, nil
	}
	return "", fmt.Errorf("option '%s' has invalid string value", name)
}

// GetInt safely retrieves an integer option
func (tpo *TypedProviderOptions) GetInt(name string) (int, error) {
	option, exists := tpo.Options[name]
	if !exists {
		return 0, fmt.Errorf("option '%s' not found", name)
	}
	if option.Type != OptionTypeInt {
		return 0, fmt.Errorf("option '%s' is not an int, got %s", name, option.Type)
	}
	if value, ok := option.Value.(int); ok {
		return value, nil
	}
	return 0, fmt.Errorf("option '%s' has invalid int value", name)
}

// GetFloat safely retrieves a float option
func (tpo *TypedProviderOptions) GetFloat(name string) (float64, error) {
	option, exists := tpo.Options[name]
	if !exists {
		return 0, fmt.Errorf("option '%s' not found", name)
	}
	if option.Type != OptionTypeFloat {
		return 0, fmt.Errorf("option '%s' is not a float, got %s", name, option.Type)
	}
	if value, ok := option.Value.(float64); ok {
		return value, nil
	}
	return 0, fmt.Errorf("option '%s' has invalid float value", name)
}

// GetBool safely retrieves a boolean option
func (tpo *TypedProviderOptions) GetBool(name string) (bool, error) {
	option, exists := tpo.Options[name]
	if !exists {
		return false, fmt.Errorf("option '%s' not found", name)
	}
	if option.Type != OptionTypeBool {
		return false, fmt.Errorf("option '%s' is not a bool, got %s", name, option.Type)
	}
	if value, ok := option.Value.(bool); ok {
		return value, nil
	}
	return false, fmt.Errorf("option '%s' has invalid bool value", name)
}

// ValidateAll validates all options and returns errors for any invalid configurations
func (tpo *TypedProviderOptions) ValidateAll() []error {
	var errors []error
	
	for name, option := range tpo.Options {
		// Check required options
		if option.Required && option.Value == nil {
			errors = append(errors, fmt.Errorf("required option '%s' is missing", name))
			continue
		}
		
		// Validate type consistency
		switch option.Type {
		case OptionTypeString:
			if _, ok := option.Value.(string); !ok && option.Value != nil {
				errors = append(errors, fmt.Errorf("option '%s' should be string, got %T", name, option.Value))
			}
		case OptionTypeInt:
			if _, ok := option.Value.(int); !ok && option.Value != nil {
				errors = append(errors, fmt.Errorf("option '%s' should be int, got %T", name, option.Value))
			}
		case OptionTypeFloat:
			if _, ok := option.Value.(float64); !ok && option.Value != nil {
				errors = append(errors, fmt.Errorf("option '%s' should be float64, got %T", name, option.Value))
			}
		case OptionTypeBool:
			if _, ok := option.Value.(bool); !ok && option.Value != nil {
				errors = append(errors, fmt.Errorf("option '%s' should be bool, got %T", name, option.Value))
			}
		}
	}
	
	return errors
}

// ToMap converts typed options back to map[string]any for legacy compatibility
func (tpo *TypedProviderOptions) ToMap() map[string]any {
	result := make(map[string]any)
	for name, option := range tpo.Options {
		result[name] = option.Value
	}
	return result
}

// FromMap creates typed options from legacy map[string]any with best-effort type inference
func (tpo *TypedProviderOptions) FromMap(data map[string]any) {
	for name, value := range data {
		switch v := value.(type) {
		case string:
			tpo.AddString(name, "", false, v)
		case int:
			tpo.AddInt(name, "", false, v)
		case float64:
			tpo.AddFloat(name, "", false, v)
		case bool:
			tpo.AddBool(name, "", false, v)
		default:
			// For complex types, store as object
			tpo.Options[name] = TypedOption{
				Name:  name,
				Type:  OptionTypeObject,
				Value: v,
			}
		}
	}
}

// ToJSON converts typed options to JSON for serialization
func (tpo *TypedProviderOptions) ToJSON() ([]byte, error) {
	return json.Marshal(tpo)
}

// FromJSON creates typed options from JSON
func (tpo *TypedProviderOptions) FromJSON(data []byte) error {
	var options TypedProviderOptions
	if err := json.Unmarshal(data, &options); err != nil {
		return err
	}
	*tpo = options
	return nil
}