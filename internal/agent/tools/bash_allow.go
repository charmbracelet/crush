package tools

import (
	"os"
	"strings"
)

func init() {
	v := strings.TrimSpace(os.Getenv("CRUSH_BASH_ALLOWED_COMMANDS"))
	if v == "" {
		return
	}
	if v == "*" {
		bannedCommands = nil
		return
	}
	allow := make(map[string]struct{})
	for _, c := range strings.Split(v, ",") {
		if c = strings.TrimSpace(c); c != "" {
			allow[c] = struct{}{}
		}
	}
	filtered := make([]string, 0, len(bannedCommands))
	for _, c := range bannedCommands {
		if _, ok := allow[c]; !ok {
			filtered = append(filtered, c)
		}
	}
	bannedCommands = filtered
}
