package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

// validateProviderConfig validates provider configuration and reports unknown fields
type validationResult struct {
	UnknownFields []string
	Valid         bool
}

// validateProviderJSON validates JSON configuration against known provider fields
func validateProviderJSON(providerName string, rawJSON json.RawMessage) validationResult {
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(rawJSON, &rawMap); err != nil {
		return validationResult{Valid: false}
	}

	// Known valid fields for ProviderConfig
	knownFields := map[string]bool{
		"id":                   true,
		"name":                 true,
		"base_url":             true,
		"type":                 true,
		"api_key":              true,
		"disable":              true,
		"system_prompt_prefix": true,
		"extra_headers":        true,
		"extra_body":           true,
		"models":               true,
	}

	// Known valid fields for Model within models array
	knownModelFields := map[string]bool{
		"id":                       true,
		"name":                     true,
		"cost_per_1m_in":           true,
		"cost_per_1m_out":          true,
		"cost_per_1m_in_cached":    true,
		"cost_per_1m_out_cached":   true,
		"context_window":           true,
		"default_max_tokens":       true,
		"can_reason":               true,
		"has_reasoning_efforts":    true,
		"default_reasoning_effort": true,
		"supports_attachments":     true,
	}

	var unknownFields []string

	// Check provider-level fields
	for field := range rawMap {
		if !knownFields[field] {
			unknownFields = append(unknownFields, fmt.Sprintf("provider.%s", field))
		}
	}

	// Check model fields if models array exists
	if modelsRaw, exists := rawMap["models"]; exists {
		var models []map[string]json.RawMessage
		if err := json.Unmarshal(modelsRaw, &models); err == nil {
			for i, model := range models {
				for field := range model {
					if !knownModelFields[field] {
						unknownFields = append(unknownFields, fmt.Sprintf("provider.models[%d].%s", i, field))
					}
				}
			}
		}
	}

	return validationResult{
		UnknownFields: unknownFields,
		Valid:         len(unknownFields) == 0,
	}
}

// logValidationWarnings logs warnings for unknown configuration fields
func logValidationWarnings(providerName string, result validationResult) {
	if len(result.UnknownFields) > 0 {
		for _, field := range result.UnknownFields {
			context := "provider"
			if strings.Contains(field, "models[") {
				context = "model"
			}
			
			validFields := getValidFields(context)
			slog.Warn("Unknown configuration field",
				"provider", providerName,
				"field", field,
				"valid_fields", strings.Join(validFields, ", "),
			)
		}
	}
}

// getValidFields returns all valid configuration fields for the given context
func getValidFields(context string) []string {
	switch context {
	case "provider":
		return []string{
			"id", "name", "base_url", "type", "api_key", "disable",
			"system_prompt_prefix", "extra_headers", "extra_body", "models",
		}
	case "model":
		return []string{
			"id", "name", "cost_per_1m_in", "cost_per_1m_out", "cost_per_1m_in_cached",
			"cost_per_1m_out_cached", "context_window", "default_max_tokens",
			"can_reason", "has_reasoning_efforts", "default_reasoning_effort",
			"supports_attachments",
		}
	default:
		return []string{}
	}
}
