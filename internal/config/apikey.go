package config

import (
	"encoding/json"
	"fmt"
)

// FlexibleAPIKey can hold either a string or a JSON object
type FlexibleAPIKey struct {
	value interface{}
}

// UnmarshalJSON implements custom JSON unmarshaling to handle both strings and objects
func (f *FlexibleAPIKey) UnmarshalJSON(data []byte) error {
	// First try to unmarshal as a string
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		f.value = str
		return nil
	}

	// If that fails, try as a raw JSON object
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err == nil {
		// Store the object as a JSON string for easy passing to providers
		jsonBytes, err := json.Marshal(obj)
		if err != nil {
			return err
		}
		f.value = string(jsonBytes)
		return nil
	}

	return fmt.Errorf("api_key must be either a string or a JSON object")
}

// MarshalJSON implements custom JSON marshaling
func (f FlexibleAPIKey) MarshalJSON() ([]byte, error) {
	if f.value == nil {
		return json.Marshal("")
	}

	// If it's a JSON string (from an object), try to marshal it as an object
	if str, ok := f.value.(string); ok {
		if len(str) > 0 && str[0] == '{' {
			// It's likely a JSON object stored as a string
			return []byte(str), nil
		}
		// Regular string
		return json.Marshal(str)
	}

	return json.Marshal(f.value)
}

// String returns the API key as a string (either the original string or JSON-encoded object)
func (f FlexibleAPIKey) String() string {
	if f.value == nil {
		return ""
	}
	if str, ok := f.value.(string); ok {
		return str
	}
	// This shouldn't happen based on our UnmarshalJSON implementation
	jsonBytes, _ := json.Marshal(f.value)
	return string(jsonBytes)
}

// NewFlexibleAPIKey creates a FlexibleAPIKey from a string value
func NewFlexibleAPIKey(value string) FlexibleAPIKey {
	return FlexibleAPIKey{value: value}
}
