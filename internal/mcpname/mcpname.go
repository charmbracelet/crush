// Package mcpname builds and parses the wire names of MCP tools.
package mcpname

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"slices"
	"strings"
)

// MaxLen is the longest tool name every provider accepts (MCP SEP-986).
const MaxLen = 64

// maxServerLen keeps a pathological server name from using up the budget.
const maxServerLen = 32

// placeholder stands in for a segment that cleans down to nothing.
const placeholder = "x"

// hashLen is how many hex characters of the digest a changed name carries.
const hashLen = 6

// disallowed matches characters providers commonly reject in tool names.
var disallowed = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// underscoreRun matches a run of underscores.
var underscoreRun = regexp.MustCompile(`_{2,}`)

// wireValid matches a tool name every provider accepts.
var wireValid = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// hashSegment matches the digest segment of a hashed name.
var hashSegment = regexp.MustCompile(`^[0-9a-f]{6}$`)

// Tool returns "mcp__<server>__<tool>", or "mcp__<server>__<tool>__<hash>"
// when cleaning changed a segment or the name was shortened to [MaxLen].
// Segments never contain "__", so clean names have two delimiters and hashed
// names three, and the two can never collide.
func Tool(server, tool string) string {
	s, sameServer := clean(server, maxServerLen)
	t, sameTool := clean(tool, 0)
	name := "mcp__" + s + "__" + t
	if sameServer && sameTool && len(name) <= MaxLen {
		return name
	}
	suffix := "__" + digest(server+"\x00"+tool)
	budget := MaxLen - len("mcp__") - len(s) - len("__") - len(suffix)
	if len(t) > budget {
		t = strings.TrimRight(t[:budget], "_")
		if t == "" {
			t = placeholder
		}
	}
	return "mcp__" + s + "__" + t + suffix
}

// clean makes s a delimiter-safe segment of at most maxLen characters (no
// limit when 0) and reports whether it came back unchanged.
func clean(s string, maxLen int) (string, bool) {
	c := disallowed.ReplaceAllString(s, "_")
	c = underscoreRun.ReplaceAllString(c, "_")
	c = strings.Trim(c, "_")
	if maxLen > 0 && len(c) > maxLen {
		c = strings.TrimRight(c[:maxLen], "_")
	}
	if c == "" {
		c = placeholder
	}
	return c, c == s
}

func digest(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:hashLen]
}

// Parts is an MCP tool name split into its halves.
type Parts struct {
	Server string
	Tool   string
	// Hashed means Server and Tool are cleaned text, so nothing should be
	// keyed on them.
	Hashed bool
}

// Parse splits a current name, or a legacy "mcp_<server>_<tool>" name from
// an old saved session (split at the first underscore).
func Parse(name string) (Parts, bool) {
	if rest, ok := strings.CutPrefix(name, "mcp__"); ok {
		segs := strings.Split(rest, "__")
		switch {
		case len(segs) == 2 && segs[0] != "" && segs[1] != "":
			return Parts{Server: segs[0], Tool: segs[1]}, true
		case len(segs) == 3 && segs[0] != "" && segs[1] != "" && hashSegment.MatchString(segs[2]):
			return Parts{Server: segs[0], Tool: segs[1], Hashed: true}, true
		}
	}
	if rest, ok := strings.CutPrefix(name, "mcp_"); ok {
		server, tool, ok := strings.Cut(rest, "_")
		if ok && server != "" && tool != "" {
			return Parts{Server: server, Tool: tool}, true
		}
	}
	return Parts{}, false
}

// IsServer reports whether name is an unhashed tool name from server.
func IsServer(name, server string) bool {
	p, ok := Parse(name)
	return ok && !p.Hashed && p.Server == server
}

// LegacyOf returns the pre-rename "mcp_<server>_<tool>" spelling of an
// unhashed current name. Hashed names had no valid old spelling.
func LegacyOf(name string) (string, bool) {
	if !strings.HasPrefix(name, "mcp__") {
		return "", false
	}
	p, ok := Parse(name)
	if !ok || p.Hashed {
		return "", false
	}
	return "mcp_" + p.Server + "_" + p.Tool, true
}

// UpgradeAllowList adds the current name for every legacy
// "mcp_<server>_<tool>[:action]" entry, splitting at the longest known
// server name. Original entries are kept.
func UpgradeAllowList(entries, servers []string) []string {
	out := slices.Clone(entries)
	for _, entry := range entries {
		rest, ok := strings.CutPrefix(entry, "mcp_")
		if !ok || strings.HasPrefix(rest, "_") {
			continue
		}
		action := ""
		if i := strings.LastIndex(rest, ":"); i >= 0 {
			rest, action = rest[:i], rest[i:]
		}
		server := ""
		for _, s := range servers {
			if len(s) > len(server) && len(rest) > len(s)+1 && strings.HasPrefix(rest, s+"_") {
				server = s
			}
		}
		if server == "" {
			continue
		}
		if upgraded := Tool(server, rest[len(server)+1:]) + action; upgraded != entry {
			out = append(out, upgraded)
		}
	}
	return out
}

// WireSafe returns name if every provider accepts it, otherwise a cleaned,
// shortened copy ending in a digest of the original.
func WireSafe(name string) string {
	if wireValid.MatchString(name) {
		return name
	}
	suffix := "_" + digest(name)
	c := disallowed.ReplaceAllString(name, "_")
	if len(c) > MaxLen-len(suffix) {
		c = c[:MaxLen-len(suffix)]
	}
	return c + suffix
}
