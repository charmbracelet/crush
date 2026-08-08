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

// reservedNames is the set of names that may not be used for subagents: the
// built-in agent names, every built-in tool name (so a subagent can shadow
// neither in prompts nor dispatch), and "mcp". Both source lists are fixed,
// so the set is built once.
var reservedNames = func() map[string]bool {
	set := map[string]bool{
		"coder": true,
		"task":  true,
		"mcp":   true,
	}
	for _, name := range config.AllToolNames() {
		set[name] = true
	}
	return set
}()

// ToolList is a []string that YAML-unmarshals from either a comma-separated
// scalar string ("view, grep, bash") or a YAML sequence (["view","grep"]).
//
// A nil ToolList means the field was absent; a non-nil empty one means the
// author explicitly wrote an empty list. The two are not interchangeable:
// absent `tools:` inherits the base agent's whole tool pool, while `tools: []`
// requests no tools at all.
type ToolList []string

// UnmarshalYAML implements yaml.Unmarshaler for ToolList.
func (t *ToolList) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		// A bare `tools:` (null) is absent. An empty string is an explicit
		// empty list, like `tools: []`.
		if value.Tag == "!!null" {
			return nil
		}
		parts := strings.Split(value.Value, ",")
		result := make(ToolList, 0, len(parts))
		for _, p := range parts {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		*t = result
		return nil
	case yaml.SequenceNode:
		var items []string
		if err := value.Decode(&items); err != nil {
			return err
		}
		result := make(ToolList, 0, len(items))
		for _, item := range items {
			if trimmed := strings.TrimSpace(item); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		*t = result
		return nil
	case yaml.AliasNode:
		return t.UnmarshalYAML(value.Alias)
	default:
		// A mapping (or other structure) is a user error; treating it as
		// absent would silently inherit the base tool pool.
		return fmt.Errorf("expected a list or comma-separated string, got %s", yamlKindName(value.Kind))
	}
}

// yamlKindName names a yaml.Kind for error messages. yaml.Kind is a bare
// uint32 with no String method, so formatting one directly renders an opaque
// number — and these errors are surfaced to users in the Library tab.
func yamlKindName(k yaml.Kind) string {
	switch k {
	case yaml.DocumentNode:
		return "a document"
	case yaml.SequenceNode:
		return "a list"
	case yaml.MappingNode:
		return "a mapping"
	case yaml.ScalarNode:
		return "a scalar"
	case yaml.AliasNode:
		return "an alias"
	default:
		return "an unknown value"
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

// Model aliases accepted in the `model:` frontmatter field. They map a
// subagent onto the globally selected model of the same name instead of a
// specific provider model id, and deliberately match the
// config.SelectedModelType values.
const (
	ModelAliasLarge = string(config.SelectedModelTypeLarge)
	ModelAliasSmall = string(config.SelectedModelTypeSmall)
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
		filtered := pool[:0]
		for _, t := range pool {
			if !disallowed[t] {
				filtered = append(filtered, t)
			}
		}
		pool = filtered
	}

	// Intersect with the explicit tools allowlist (cannot widen beyond base).
	// Tested for nil, not length: an explicitly empty `tools: []` must yield no
	// tools, whereas an absent field leaves the base pool untouched.
	if s.Tools != nil {
		allowed := make(map[string]bool, len(s.Tools))
		for _, t := range s.Tools {
			allowed[t] = true
		}
		filtered := pool[:0]
		for _, t := range pool {
			if allowed[t] {
				filtered = append(filtered, t)
			}
		}
		pool = filtered
	}

	// AllowedMCP nil means "no restrictions configured" (full access to every
	// MCP server) to config.Agent's consumer; a subagent with no mcp_servers:
	// frontmatter must default to the locked-down empty map instead, matching
	// the built-in task agent's secure-by-default AllowedMCP.
	allowedMCP := make(map[string][]string, len(s.MCPServers))
	for _, srv := range s.MCPServers {
		allowedMCP[srv] = nil
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

// ValidateAgainst runs Validate plus model- and skill-resolution checks. When
// isKnownModel is non-nil and Model is a non-empty value other than the
// "large"/"small" aliases, the resolver must return true or validation fails.
// When isKnownSkill is non-nil, every name in Skills must resolve to a known
// skill. A nil resolver skips the corresponding check (used when the caller
// has no config or skills context).
func (s *Subagent) ValidateAgainst(isKnownModel func(provider, model string) bool, isKnownSkill func(name string) bool) error {
	errs := []error{s.Validate()}
	if isKnownModel != nil && s.Model != "" && s.Model != ModelAliasLarge && s.Model != ModelAliasSmall && !isKnownModel(s.Provider, s.Model) {
		errs = append(errs, fmt.Errorf("model %q is not a known model id; use %q, %q, or a valid provider model id", s.Model, ModelAliasLarge, ModelAliasSmall))
	}
	if isKnownSkill != nil {
		for _, name := range s.Skills {
			if !isKnownSkill(name) {
				errs = append(errs, fmt.Errorf("skill %q is not an invocable active skill (unknown or model-invocation disabled)", name))
			}
		}
	}
	return errors.Join(errs...)
}

// knownToolNames is the set of built-in tool names a subagent may reference.
// Built once: config.AllToolNames returns a fixed list.
var knownToolNames = func() map[string]bool {
	names := config.AllToolNames()
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return set
}()

// unknownToolErrors reports every name in list that is not a built-in tool.
// Without this an unknown name is not an error anywhere: `tools:` intersects
// against the base pool and simply matches nothing (leaving the subagent with
// no tools at all), while `disallowedTools:` fails open and removes nothing.
// Tool names are lowercase snake_case, so a list written in another tool's
// PascalCase convention is caught here rather than at dispatch.
func unknownToolErrors(field string, list ToolList) []error {
	var errs []error
	for _, tool := range list {
		if knownToolNames[tool] {
			continue
		}
		msg := fmt.Sprintf("%s references unknown tool %q", field, tool)
		if lower := strings.ToLower(tool); lower != tool && knownToolNames[lower] {
			msg += fmt.Sprintf("; tool names are lowercase, did you mean %q?", lower)
		}
		errs = append(errs, errors.New(msg))
	}
	return errs
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
			msg := "name must be lowercase alphanumeric with single hyphens (no leading, trailing, or consecutive hyphens)"
			// Definition files carried over from the cross-tool
			// .agents/subagents convention routinely use names like "Explore".
			// Those fail here and are dropped from discovery, so the fix has to
			// be in the message — the Library only shows this error text.
			if lower := strings.ToLower(s.Name); lower != s.Name && namePattern.MatchString(lower) {
				msg += fmt.Sprintf("; rename %q to %q", s.Name, lower)
			}
			errs = append(errs, errors.New(msg))
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

	errs = append(errs, unknownToolErrors("tools", s.Tools)...)
	errs = append(errs, unknownToolErrors("disallowedTools", s.DisallowedTools)...)

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

	if s.Provider != "" && (s.Model == "" || s.Model == ModelAliasLarge || s.Model == ModelAliasSmall) {
		errs = append(errs, fmt.Errorf("provider requires a specific model id; use a valid provider model id (not empty, %q, or %q)", ModelAliasLarge, ModelAliasSmall))
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
// exist, the last occurrence wins (consistent with Deduplicate for subagents),
// so the surviving state describes the file whose agent survived.
//
// Error states are exempt from that collapse. They are per-file diagnostics —
// paths are unique, names are not — and the Library renders one row per error
// state. Name-keying them means a valid definition elsewhere silently hides
// the broken file the user is trying to fix, which is the failure surfacing
// error states was meant to end.
func DeduplicateStates(all []*SubagentState) []*SubagentState {
	seen := make(map[string]int, len(all))
	for i, s := range all {
		if s.Name != "" && s.State != StateError {
			seen[s.Name] = i
		}
	}

	result := make([]*SubagentState, 0, len(all))
	for i, s := range all {
		// Keep every error state and anything without a name, plus the last
		// non-error occurrence of each name.
		if s.Name == "" || s.State == StateError || seen[s.Name] == i {
			result = append(result, s)
		}
	}
	return result
}

// DiscoverWithStates finds all valid subagent definition files (*.md) in the
// given paths recursively, and returns both the discovered subagents and a
// per-file state slice describing parse/validation outcomes. When
// isKnownModel is non-nil it is used to validate non-alias model ids; when
// isKnownSkill is non-nil it is used to validate skills references; a nil
// func skips the corresponding check.
//
// The returned agents preserve the caller's path order: all subagents from
// paths[0] (sorted by file path), then paths[1], and so on. Deduplicate keeps
// the last occurrence of a name, so this ordering is what makes later paths —
// the working directory, per ProjectSubagentsDir — override earlier ones
// (monorepo root, global dirs) on a name collision.
func DiscoverWithStates(paths []string, isKnownModel func(provider, model string) bool, isKnownSkill func(name string) bool) ([]*Subagent, []*SubagentState) {
	var agents []*Subagent
	var states []*SubagentState
	var mu sync.Mutex
	seen := make(map[string]bool)

	for _, base := range paths {
		var baseAgents []*Subagent
		var baseStates []*SubagentState
		addState := func(name, path string, state DiscoveryState, err error) {
			mu.Lock()
			baseStates = append(baseStates, &SubagentState{
				Name:  name,
				Path:  path,
				State: state,
				Err:   err,
			})
			mu.Unlock()
		}
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
			if err := agent.ValidateAgainst(isKnownModel, isKnownSkill); err != nil {
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
		// States are sorted and appended on the same per-base schedule as the
		// agents above. Deduplicate and DeduplicateStates both keep the last
		// occurrence of a name, so the two lists must agree on what "last"
		// means — otherwise a name collision can resolve to one file's agent
		// while the Library shows the other file's state. (Error states opt
		// out of that collapse entirely; see DeduplicateStates.)
		slices.SortStableFunc(baseStates, func(a, b *SubagentState) int {
			return strings.Compare(strings.ToLower(a.Path), strings.ToLower(b.Path))
		})
		agents = append(agents, baseAgents...)
		states = append(states, baseStates...)
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
