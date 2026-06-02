package toolcoach

import "encoding/json"

// parseJSON is a thin wrapper around json.Unmarshal so that other files in the
// package do not need to repeat the encoding/json import for one-liners.
func parseJSON(data string, v any) error {
	return json.Unmarshal([]byte(data), v)
}
