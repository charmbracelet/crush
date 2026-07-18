// Package subagents implements parsing and validation of subagent definition
// files.
package subagents

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/charlievieth/fastwalk"
	"gopkg.in/yaml.v3"

	"github.com/charmbracelet/crush/internal/config"
)

const (
	// MaxNameLength is the max characters allowed in a subagent name.
	MaxNameLength = 64
	// MaxDescriptionLength is the max characters allowed in a subagent
	// description.
	MaxDescriptionLength = 1024
)

// namePattern matches valid subagent names: lowercase alphanumeric with single
// hyphens, no leading or trailing hyphens, no consecutive hyphens.
var namePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// reservedNames is the set of names that may not be used for subagents.
var reservedNames = map[string]bool{
	"agent": true,
	"task":  true,
	"coder": true,
	"bash":  true,
	"view":  true,
	"edit":  true,
	"grep":  true,
	"glob":  true,
	"write": true,
	"ls":    true,
	"mcp":   true,
}

// ToolList is a []string that YAML-unmarshals from either a comma-separated
// scalar string ("Read, Grep, Bash") or a YAML sequence (["Read","Grep"]).
// When the field is absent the value stays nil.
type ToolList []string

// UnmarshalYAML implements yaml.Unmarshaler for ToolList.
func (t *ToolList) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		if value.Value == "" || value.Tag == "!!null" {
			return nil
		}
		parts := strings.Split(value.Value, ",")
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		if len(result) > 0 {
			*t = result
		}
		return nil
	case yaml.SequenceNode:
		var items []string
		if err := value.Decode(&items); err != nil {
			return err
		}
		if len(items) > 0 {
			*t = items
		}
		return nil
	default:
		return nil
	}
}

// Subagent is a parsed subagent definition file.
type Subagent struct {
	Name            string   `yaml:"name"`
	Description     string   `yaml:"description"`
	Tools           ToolList `yaml:"tools"`
	DisallowedTools ToolList `yaml:"disallowedTools"`
	Model           string   `yaml:"model"`
	Effort          string   `yaml:"effort"`
	Skills          []string `yaml:"skills"`
	MCPServers      []string `yaml:"mcp_servers"`
	PermissionMode  string   `yaml:"permissionMode"`
	Color           string   `yaml:"color"`
	Provider        string   `yaml:"provider"`
	Body            string   // set from markdown body after frontmatter
	FilePath        string   // set from the file path passed to Parse
}

// ResolvedColor returns the subagent's explicit Color if set, or falls back to
// AutoColor(Name) for a deterministic palette assignment.
func (s Subagent) ResolvedColor() string {
	if s.Color != "" {
		return s.Color
	}
	return AutoColor(s.Name)
}

// PermissionMode values accepted in the PermissionMode field.
const (
	PermissionModeDefault           = "default"
	PermissionModeBypassPermissions = "bypassPermissions"
)

// ToConfigAgent converts the Subagent into a config.Agent by applying the
// subagent's tool restrictions and model preference on top of the provided
// base agent configuration.
func (s *Subagent) ToConfigAgent(base config.Agent) config.Agent {
	// Start with a copy of the base allowed tools — never mutate the original.
	pool := append([]string(nil), base.AllowedTools...)

	// Apply disallowed tools first.
	if len(s.DisallowedTools) > 0 {
		disallowed := make(map[string]bool, len(s.DisallowedTools))
		for _, t := range s.DisallowedTools {
			disallowed[t] = true
		}
		filtered := pool[:0:0]
		for _, t := range pool {
			if !disallowed[t] {
				filtered = append(filtered, t)
			}
		}
		pool = filtered
	}

	// Intersect with the explicit tools allowlist (cannot widen beyond base).
	if len(s.Tools) > 0 {
		allowed := make(map[string]bool, len(s.Tools))
		for _, t := range s.Tools {
			allowed[t] = true
		}
		filtered := pool[:0:0]
		for _, t := range pool {
			if allowed[t] {
				filtered = append(filtered, t)
			}
		}
		pool = filtered
	}

	// Build AllowedMCP only when MCP servers are specified.
	var allowedMCP map[string][]string
	if len(s.MCPServers) > 0 {
		allowedMCP = make(map[string][]string, len(s.MCPServers))
		for _, srv := range s.MCPServers {
			allowedMCP[srv] = nil
		}
	}

	return config.Agent{
		ID:           s.Name,
		Name:         s.Name,
		Description:  s.Description,
		AllowedTools: pool,
		AllowedMCP:   allowedMCP,
		// Model selection is driven by the coordinator from the raw `model:`
		// value (alias or specific id); inherit the base type here. This field
		// is no longer consumed for subagents.
		Model: base.Model,
	}
}

// ParseContent parses a subagent definition from raw bytes.
func ParseContent(content []byte) (*Subagent, error) {
	frontmatter, body, err := splitFrontmatter(string(content))
	if err != nil {
		return nil, err
	}

	var agent Subagent
	if err := yaml.Unmarshal([]byte(frontmatter), &agent); err != nil {
		return nil, fmt.Errorf("parsing frontmatter: %w", err)
	}

	agent.Body = strings.TrimSpace(body)

	return &agent, nil
}

// Parse reads a subagent definition file from disk and sets FilePath.
func Parse(path string) (*Subagent, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	agent, err := ParseContent(content)
	if err != nil {
		return nil, err
	}

	agent.FilePath = path

	return agent, nil
}

// ValidateAgainst runs Validate plus model-resolution checks. When isKnownModel
// is non-nil and Model is a non-empty value other than "large"/"small", the
// resolver must return true or validation fails. A nil resolver skips the
// model check (used when the caller has no config context).
func (s *Subagent) ValidateAgainst(isKnownModel func(provider, model string) bool) error {
	err := s.Validate()
	if isKnownModel == nil {
		return err
	}
	if s.Model == "" || s.Model == "large" || s.Model == "small" {
		return err
	}
	if !isKnownModel(s.Provider, s.Model) {
		modelErr := fmt.Errorf("model %q is not a known model id; use \"large\", \"small\", or a valid provider model id", s.Model)
		if err == nil {
			return modelErr
		}
		return errors.Join(err, modelErr)
	}
	return err
}

// Validate checks that the subagent meets all specification requirements.
// Multiple errors are joined with errors.Join.
func (s *Subagent) Validate() error {
	var errs []error

	if s.Name == "" {
		errs = append(errs, errors.New("name is required"))
	} else {
		if len(s.Name) > MaxNameLength {
			errs = append(errs, fmt.Errorf("name exceeds %d characters", MaxNameLength))
		}
		if !namePattern.MatchString(s.Name) {
			errs = append(errs, errors.New("name must be lowercase alphanumeric with single hyphens (no leading, trailing, or consecutive hyphens)"))
		}
		if reservedNames[s.Name] {
			errs = append(errs, fmt.Errorf("name %q is reserved", s.Name))
		}
	}

	if s.Description == "" {
		errs = append(errs, errors.New("description is required"))
	} else if len(s.Description) > MaxDescriptionLength {
		errs = append(errs, fmt.Errorf("description exceeds %d characters", MaxDescriptionLength))
	}

	if len(s.Tools) > 0 && len(s.DisallowedTools) > 0 {
		disallowedSet := make(map[string]bool, len(s.DisallowedTools))
		for _, tool := range s.DisallowedTools {
			disallowedSet[tool] = true
		}
		for _, tool := range s.Tools {
			if disallowedSet[tool] {
				errs = append(errs, fmt.Errorf("tool %q appears in both tools and disallowedTools", tool))
			}
		}
	}

	switch s.Effort {
	case "", EffortNone, EffortMinimal, EffortLow, EffortMedium, EffortHigh, EffortXHigh, EffortMax:
	default:
		errs = append(errs, fmt.Errorf("effort %q is not valid; use one of: %q, %q, %q, %q, %q, %q, %q", s.Effort, EffortNone, EffortMinimal, EffortLow, EffortMedium, EffortHigh, EffortXHigh, EffortMax))
	}

	switch s.PermissionMode {
	case "", PermissionModeDefault, PermissionModeBypassPermissions:
	default:
		errs = append(errs, fmt.Errorf("permissionMode %q is not valid; use %q or %q", s.PermissionMode, PermissionModeDefault, PermissionModeBypassPermissions))
	}

	if s.Color != "" && !IsValidColor(s.Color) {
		errs = append(errs, fmt.Errorf("color %q is not valid; use one of: red, orange, yellow, green, cyan, blue, purple, pink", s.Color))
	}

	if s.Provider != "" && (s.Model == "" || s.Model == "large" || s.Model == "small") {
		errs = append(errs, fmt.Errorf("provider requires a specific model id; use a valid provider model id (not empty, %q, or %q)", "large", "small"))
	}

	return errors.Join(errs...)
}

// Filter removes subagents whose names appear in the disabled list.
func Filter(all []*Subagent, disabled []string) []*Subagent {
	if len(disabled) == 0 {
		return all
	}

	disabledSet := make(map[string]bool, len(disabled))
	for _, name := range disabled {
		disabledSet[name] = true
	}

	result := make([]*Subagent, 0, len(all))
	for _, s := range all {
		if !disabledSet[s.Name] {
			result = append(result, s)
		}
	}
	return result
}

// Deduplicate removes duplicate subagents by name. When duplicates exist, the
// last occurrence wins.
func Deduplicate(all []*Subagent) []*Subagent {
	if len(all) == 0 {
		return nil
	}

	seen := make(map[string]int, len(all))
	for i, s := range all {
		seen[s.Name] = i
	}

	result := make([]*Subagent, 0, len(seen))
	for i, s := range all {
		if seen[s.Name] == i {
			result = append(result, s)
		}
	}
	return result
}

// DiscoveryState represents the outcome of discovering a single subagent file.
type DiscoveryState int

const (
	// StateNormal indicates the subagent was parsed and validated successfully.
	StateNormal DiscoveryState = iota
	// StateError indicates discovery encountered a scan/parse/validate error.
	StateError
)

// SubagentState represents the latest discovery status of a subagent file.
type SubagentState struct {
	Name  string
	Path  string
	State DiscoveryState
	Err   error
}

// Event is published when subagent discovery completes.
type Event struct {
	States []*SubagentState
}

// cloneSubagents returns a shallow copy of the slice so callers cannot mutate
// the manager's internal slice header. The underlying *Subagent pointers are
// shared — subagents are immutable post-discovery.
func cloneSubagents(in []*Subagent) []*Subagent {
	if in == nil {
		return nil
	}
	out := make([]*Subagent, len(in))
	copy(out, in)
	return out
}

// cloneStates returns a deep copy of the given state slice so callers cannot
// accidentally mutate the source.
func cloneStates(states []*SubagentState) []*SubagentState {
	if states == nil {
		return nil
	}
	result := make([]*SubagentState, len(states))
	for i, s := range states {
		clone := *s
		result[i] = &clone
	}
	return result
}

// DeduplicateStates removes duplicate subagent states by name. When duplicates
// exist, the last occurrence wins (consistent with Deduplicate for subagents).
func DeduplicateStates(all []*SubagentState) []*SubagentState {
	seen := make(map[string]int, len(all))
	for i, s := range all {
		if s.Name != "" {
			seen[s.Name] = i
		}
	}

	result := make([]*SubagentState, 0, len(seen))
	for i, s := range all {
		// Keep the last occurrence of this name, or anything without a
		// name (error state).
		if s.Name == "" || seen[s.Name] == i {
			result = append(result, s)
		}
	}
	return result
}

// DiscoverWithStates finds all valid subagent definition files (*.md) in the
// given paths recursively, and returns both the discovered subagents and a
// per-file state slice describing parse/validation outcomes. When
// isKnownModel is non-nil it is used to validate non-alias model ids; nil
// skips that check.
//
// The returned agents preserve the caller's path order: all subagents from
// paths[0] (sorted by file path), then paths[1], and so on. Deduplicate keeps
// the last occurrence of a name, so this ordering is what makes later paths —
// the working directory, per ProjectSubagentsDir — override earlier ones
// (monorepo root, global dirs) on a name collision.
func DiscoverWithStates(paths []string, isKnownModel func(provider, model string) bool) ([]*Subagent, []*SubagentState) {
	var agents []*Subagent
	var states []*SubagentState
	var mu sync.Mutex
	seen := make(map[string]bool)

	addState := func(name, path string, state DiscoveryState, err error) {
		mu.Lock()
		states = append(states, &SubagentState{
			Name:  name,
			Path:  path,
			State: state,
			Err:   err,
		})
		mu.Unlock()
	}

	for _, base := range paths {
		var baseAgents []*Subagent
		conf := fastwalk.Config{
			Follow:  true,
			ToSlash: fastwalk.DefaultToSlash(),
		}
		err := fastwalk.Walk(&conf, base, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				slog.Warn("Failed to walk subagents path entry", "base", base, "path", path, "error", err)
				addState("", path, StateError, err)
				return nil
			}
			if d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
				return nil
			}
			mu.Lock()
			if seen[path] {
				mu.Unlock()
				return nil
			}
			seen[path] = true
			mu.Unlock()

			agent, err := Parse(path)
			if err != nil {
				slog.Warn("Failed to parse subagent file", "path", path, "error", err)
				addState("", path, StateError, err)
				return nil
			}
			if err := agent.ValidateAgainst(isKnownModel); err != nil {
				slog.Warn("Subagent validation failed", "path", path, "error", err)
				addState(agent.Name, path, StateError, err)
				return nil
			}
			slog.Debug("Successfully loaded subagent", "name", agent.Name, "path", path)
			mu.Lock()
			baseAgents = append(baseAgents, agent)
			mu.Unlock()
			addState(agent.Name, path, StateNormal, nil)
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			slog.Warn("Failed to walk subagents path", "path", base, "error", err)
		}

		// fastwalk traversal order within a base is non-deterministic, so
		// sort each base's results for stable output. Sorting per base (never
		// across bases) preserves the caller's path-order precedence.
		slices.SortStableFunc(baseAgents, func(a, b *Subagent) int {
			if c := strings.Compare(strings.ToLower(a.FilePath), strings.ToLower(b.FilePath)); c != 0 {
				return c
			}
			return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
		})
		agents = append(agents, baseAgents...)
	}

	return agents, states
}

// splitFrontmatter extracts YAML frontmatter and body from markdown content.
func splitFrontmatter(content string) (frontmatter, body string, err error) {
	// Strip UTF-8 BOM for compatibility with editors that include it.
	content = strings.TrimPrefix(content, "\ufeff")
	// Normalize line endings to \n for consistent parsing.
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	lines := strings.Split(content, "\n")
	start := slices.IndexFunc(lines, func(line string) bool {
		return strings.TrimSpace(line) != ""
	})
	if start == -1 || strings.TrimSpace(lines[start]) != "---" {
		return "", "", errors.New("no YAML frontmatter found")
	}

	endOffset := slices.IndexFunc(lines[start+1:], func(line string) bool {
		return strings.TrimSpace(line) == "---"
	})
	if endOffset == -1 {
		return "", "", errors.New("unclosed frontmatter")
	}
	end := start + 1 + endOffset

	frontmatter = strings.Join(lines[start+1:end], "\n")
	body = strings.Join(lines[end+1:], "\n")
	return frontmatter, body, nil
}
