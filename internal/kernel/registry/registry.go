package registry

import (
	"sync"
)

// ToolCapability represents a capability that a tool can provide
type ToolCapability string

const (
	CapabilityRead       ToolCapability = "read"
	CapabilityWrite      ToolCapability = "write"
	CapabilityExecute    ToolCapability = "execute"
	CapabilitySearch     ToolCapability = "search"
	CapabilityAnalysis   ToolCapability = "analysis"
	CapabilityNetwork    ToolCapability = "network"
	CapabilityFileSystem ToolCapability = "filesystem"
)

// ToolRegistry implements Claude Code's Registry Discovery pattern
type ToolRegistry struct {
	mu sync.RWMutex

	// Core registry: tool name -> metadata
	tools map[string]*ToolMetadata

	// Capability index: capability -> tool names
	capabilityIndex map[ToolCapability][]string

	// Alias registry: alias -> canonical name
	aliases map[string]string

	// Pattern registry for dynamic discovery
	patterns []DiscoveryPattern
}

// ToolMetadata contains metadata about a registered tool
type ToolMetadata struct {
	Name         string
	Aliases      []string
	Description  string
	Capabilities []ToolCapability
	Version      string
	Author       string

	// Discovery configuration
	AutoDiscover bool
	Priority    int
}

// DiscoveryPattern defines a pattern for dynamic tool discovery
type DiscoveryPattern struct {
	Name        string
	Matcher     func(toolName string) bool
	Capabilities []ToolCapability
	Priority    int
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry() *ToolRegistry {
	tr := &ToolRegistry{
		tools:          make(map[string]*ToolMetadata),
		capabilityIndex: make(map[ToolCapability][]string),
		aliases:        make(map[string]string),
		patterns:       make([]DiscoveryPattern, 0),
	}

	// Register default patterns
	tr.registerDefaultPatterns()

	return tr
}

// Register adds a tool to the registry
func (tr *ToolRegistry) Register(meta *ToolMetadata) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	// Register tool
	tr.tools[meta.Name] = meta

	// Update capability index
	for _, cap := range meta.Capabilities {
		tr.capabilityIndex[cap] = append(tr.capabilityIndex[cap], meta.Name)
	}

	// Register aliases
	for _, alias := range meta.Aliases {
		tr.aliases[alias] = meta.Name
	}
}

// Get retrieves a tool by name or alias
func (tr *ToolRegistry) Get(name string) (*ToolMetadata, bool) {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	// Direct lookup
	if meta, ok := tr.tools[name]; ok {
		return meta, true
	}

	// Alias lookup
	if canonical, ok := tr.aliases[name]; ok {
		if meta, ok := tr.tools[canonical]; ok {
			return meta, true
		}
	}

	return nil, false
}

// FindByCapability finds all tools with a specific capability
func (tr *ToolRegistry) FindByCapability(cap ToolCapability) []*ToolMetadata {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	var results []*ToolMetadata
	for _, name := range tr.capabilityIndex[cap] {
		if meta, ok := tr.tools[name]; ok {
			results = append(results, meta)
		}
	}

	return results
}

// FindByPattern finds tools matching a discovery pattern
func (tr *ToolRegistry) FindByPattern(patternName string) []*ToolMetadata {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	var results []*ToolMetadata

	for _, pattern := range tr.patterns {
		if pattern.Name == patternName {
			for name, meta := range tr.tools {
				if pattern.Matcher(name) {
					results = append(results, meta)
				}
			}
			break
		}
	}

	return results
}

// Discover runs auto-discovery for matching tools
func (tr *ToolRegistry) Discover(toolNames []string) []*ToolMetadata {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	var results []*ToolMetadata

	for _, name := range toolNames {
		// Check direct match
		if meta, ok := tr.tools[name]; ok {
			if meta.AutoDiscover {
				results = append(results, meta)
			}
			continue
		}

		// Check patterns
		for _, pattern := range tr.patterns {
			if pattern.Matcher(name) {
				results = append(results, &ToolMetadata{
					Name:         name,
					Capabilities: pattern.Capabilities,
					Priority:    pattern.Priority,
				})
			}
		}
	}

	return results
}

// List returns all registered tools
func (tr *ToolRegistry) List() []*ToolMetadata {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	results := make([]*ToolMetadata, 0, len(tr.tools))
	for _, meta := range tr.tools {
		results = append(results, meta)
	}

	return results
}

// GetCapabilities returns all unique capabilities in the registry
func (tr *ToolRegistry) GetCapabilities() []ToolCapability {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	seen := make(map[ToolCapability]bool)
	var caps []ToolCapability

	for cap := range tr.capabilityIndex {
		if !seen[cap] {
			seen[cap] = true
			caps = append(caps, cap)
		}
	}

	return caps
}

// registerDefaultPatterns registers built-in discovery patterns
func (tr *ToolRegistry) registerDefaultPatterns() {
	tr.patterns = []DiscoveryPattern{
		{
			Name:    "read_pattern",
			Matcher: func(name string) bool { return containsAny(name, "read", "view", "cat", "fetch", "get") },
			Capabilities: []ToolCapability{CapabilityRead, CapabilityFileSystem},
			Priority:     50,
		},
		{
			Name:    "write_pattern",
			Matcher: func(name string) bool { return containsAny(name, "write", "edit", "create", "save", "put") },
			Capabilities: []ToolCapability{CapabilityWrite, CapabilityFileSystem},
			Priority:     50,
		},
		{
			Name:    "execute_pattern",
			Matcher: func(name string) bool { return containsAny(name, "run", "exec", "bash", "shell", "command") },
			Capabilities: []ToolCapability{CapabilityExecute, CapabilityFileSystem},
			Priority:     50,
		},
		{
			Name:    "search_pattern",
			Matcher: func(name string) bool { return containsAny(name, "search", "grep", "find", "query", "ls") },
			Capabilities: []ToolCapability{CapabilitySearch},
			Priority:     50,
		},
		{
			Name:    "network_pattern",
			Matcher: func(name string) bool { return containsAny(name, "http", "fetch", "web", "url", "request") },
			Capabilities: []ToolCapability{CapabilityNetwork},
			Priority:     50,
		},
	}
}

// containsAny checks if s contains any of the substrings
func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if contains(s, sub) {
			return true
		}
	}
	return false
}

// contains is a simple substring check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
