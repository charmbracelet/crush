package hooks

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

// parseShellEnv parses hook results from environment variables.
func parseShellEnv(env []string) *HookResult {
	result := &HookResult{Continue: true}

	for _, line := range env {
		if !strings.HasPrefix(line, "CRUSH_") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		switch key {
		case "CRUSH_CONTINUE":
			result.Continue = value == "true"

		case "CRUSH_PERMISSION":
			result.Permission = value

		case "CRUSH_MESSAGE":
			result.Message = value

		case "CRUSH_MODIFIED_PROMPT":
			result.ModifiedPrompt = &value

		case "CRUSH_CONTEXT_CONTENT":
			if decoded, err := base64.StdEncoding.DecodeString(value); err == nil {
				result.ContextContent = string(decoded)
			} else {
				result.ContextContent = value
			}

		case "CRUSH_CONTEXT_FILES":
			if value != "" {
				result.ContextFiles = strings.Split(value, ":")
			}

		case "CRUSH_MODIFIED_INPUT":
			if value != "" {
				result.ModifiedInput = parseKeyValuePairs(value)
			}

		case "CRUSH_MODIFIED_OUTPUT":
			if value != "" {
				result.ModifiedOutput = parseKeyValuePairs(value)
			}
		}
	}

	return result
}

// parseJSONResult parses hook results from JSON output.
func parseJSONResult(data []byte) (*HookResult, error) {
	result := &HookResult{Continue: true}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	if v, ok := raw["continue"].(bool); ok {
		result.Continue = v
	}

	if v, ok := raw["permission"].(string); ok {
		result.Permission = v
	}

	if v, ok := raw["message"].(string); ok {
		result.Message = v
	}

	if v, ok := raw["modified_prompt"].(string); ok {
		result.ModifiedPrompt = &v
	}

	if v, ok := raw["modified_input"].(map[string]any); ok {
		result.ModifiedInput = v
	}

	if v, ok := raw["modified_output"].(map[string]any); ok {
		result.ModifiedOutput = v
	}

	if v, ok := raw["context_content"].(string); ok {
		result.ContextContent = v
	}

	if v, ok := raw["context_files"].([]any); ok {
		for _, file := range v {
			if s, ok := file.(string); ok {
				result.ContextFiles = append(result.ContextFiles, s)
			}
		}
	}

	return result, nil
}

// mergeJSONResult merges JSON-parsed result into env-parsed result.
func mergeJSONResult(base *HookResult, jsonResult *HookResult) {
	if !jsonResult.Continue {
		base.Continue = false
	}

	if jsonResult.Permission != "" {
		base.Permission = jsonResult.Permission
	}

	if jsonResult.Message != "" {
		if base.Message == "" {
			base.Message = jsonResult.Message
		} else {
			base.Message += "; " + jsonResult.Message
		}
	}

	if jsonResult.ModifiedPrompt != nil {
		base.ModifiedPrompt = jsonResult.ModifiedPrompt
	}

	if len(jsonResult.ModifiedInput) > 0 {
		if base.ModifiedInput == nil {
			base.ModifiedInput = make(map[string]any)
		}
		for k, v := range jsonResult.ModifiedInput {
			base.ModifiedInput[k] = v
		}
	}

	if len(jsonResult.ModifiedOutput) > 0 {
		if base.ModifiedOutput == nil {
			base.ModifiedOutput = make(map[string]any)
		}
		for k, v := range jsonResult.ModifiedOutput {
			base.ModifiedOutput[k] = v
		}
	}

	if jsonResult.ContextContent != "" {
		if base.ContextContent == "" {
			base.ContextContent = jsonResult.ContextContent
		} else {
			base.ContextContent += "\n\n" + jsonResult.ContextContent
		}
	}

	base.ContextFiles = append(base.ContextFiles, jsonResult.ContextFiles...)
}

// parseKeyValuePairs parses "key=value:key2=value2" format into a map.
// Values are parsed as JSON when possible, otherwise treated as strings.
func parseKeyValuePairs(encoded string) map[string]any {
	result := make(map[string]any)
	pairs := strings.Split(encoded, ":")
	for _, pair := range pairs {
		key, value, ok := strings.Cut(pair, "=")
		if !ok {
			continue
		}

		// Try to parse value as JSON to support numbers, booleans, arrays, objects
		var jsonValue any
		if err := json.Unmarshal([]byte(value), &jsonValue); err == nil {
			result[key] = jsonValue
		} else {
			// Fall back to string if not valid JSON
			result[key] = value
		}
	}
	return result
}
