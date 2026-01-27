// Package subagent provides support for user-defined specialized AI agents.
package subagent

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Subagent represents a user-defined specialized AI agent.
type Subagent struct {
	// Name is the unique identifier for the subagent (kebab-case).
	Name string `yaml:"name"`
	// Description explains when this agent should be used.
	Description string `yaml:"description"`
	// Model specifies which model to use ("inherit" or specific model name).
	Model string `yaml:"model,omitempty"`
	// Tools lists the allowed tools for this agent. Empty means all tools.
	Tools []string `yaml:"tools,omitempty"`
	// AllowedTools lists tools that are pre-approved (no permission prompt).
	AllowedTools []string `yaml:"allowed_tools,omitempty"`
	// YoloMode enables automatic approval of all tool calls.
	YoloMode bool `yaml:"yolo_mode,omitempty"`
	// MaxSteps limits the number of tool invocations.
	MaxSteps int `yaml:"max_steps,omitempty"`
	// Prompt contains the system prompt content (after frontmatter).
	Prompt string `yaml:"-"`
	// Path is the filesystem path where this agent was loaded from.
	Path string `yaml:"-"`
}

// Parse reads a subagent definition from a file with YAML frontmatter.
// The file format is:
//
//	---
//	name: agent-name
//	description: When to use this agent
//	tools:
//	  - View
//	  - Grep
//	---
//	System prompt content here...
func Parse(path string) (*Subagent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading subagent file: %w", err)
	}

	return ParseContent(data, path)
}

// ParseContent parses subagent definition from content bytes.
func ParseContent(data []byte, path string) (*Subagent, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))

	// Look for opening frontmatter delimiter.
	if !scanner.Scan() {
		return nil, fmt.Errorf("empty subagent file")
	}
	firstLine := strings.TrimSpace(scanner.Text())
	if firstLine != "---" {
		return nil, fmt.Errorf("subagent file must start with YAML frontmatter (---)")
	}

	// Read frontmatter content.
	var frontmatter strings.Builder
	foundEnd := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			foundEnd = true
			break
		}
		frontmatter.WriteString(line)
		frontmatter.WriteString("\n")
	}
	if !foundEnd {
		return nil, fmt.Errorf("unclosed YAML frontmatter")
	}

	// Parse YAML frontmatter.
	var agent Subagent
	if err := yaml.Unmarshal([]byte(frontmatter.String()), &agent); err != nil {
		return nil, fmt.Errorf("parsing subagent frontmatter: %w", err)
	}

	// Validate required fields.
	if agent.Name == "" {
		return nil, fmt.Errorf("subagent missing required 'name' field")
	}
	if agent.Description == "" {
		return nil, fmt.Errorf("subagent missing required 'description' field")
	}

	// Read remaining content as the system prompt.
	var prompt strings.Builder
	for scanner.Scan() {
		prompt.WriteString(scanner.Text())
		prompt.WriteString("\n")
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading subagent prompt: %w", err)
	}

	agent.Prompt = strings.TrimSpace(prompt.String())
	agent.Path = path

	// Default model to "inherit" if not specified.
	if agent.Model == "" {
		agent.Model = "inherit"
	}

	return &agent, nil
}

// DefaultDiscoveryPaths returns the default paths to search for subagents.
func DefaultDiscoveryPaths(homeDir, workingDir string) []string {
	paths := []string{}

	// Global user config.
	if homeDir != "" {
		paths = append(paths, filepath.Join(homeDir, ".config", "crush", "agents"))
		paths = append(paths, filepath.Join(homeDir, ".config", "agents"))
	}

	// Project-local paths.
	if workingDir != "" {
		paths = append(paths, filepath.Join(workingDir, ".crush", "agents"))
		paths = append(paths, filepath.Join(workingDir, ".claude", "agents"))
	}

	return paths
}

// Discover finds all subagent definitions in the given paths.
// Files must have .md extension and contain valid YAML frontmatter.
func Discover(paths []string) ([]*Subagent, error) {
	var agents []*Subagent
	seen := make(map[string]bool)

	for _, basePath := range paths {
		entries, err := os.ReadDir(basePath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("reading agents directory %s: %w", basePath, err)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}

			agentPath := filepath.Join(basePath, entry.Name())
			agent, err := Parse(agentPath)
			if err != nil {
				// Skip invalid agent files but log the error.
				continue
			}

			// Skip duplicates (first discovery path wins).
			if seen[agent.Name] {
				continue
			}
			seen[agent.Name] = true
			agents = append(agents, agent)
		}
	}

	return agents, nil
}

// FindByName returns the subagent with the given name, or nil if not found.
func FindByName(agents []*Subagent, name string) *Subagent {
	for _, agent := range agents {
		if agent.Name == name {
			return agent
		}
	}
	return nil
}
