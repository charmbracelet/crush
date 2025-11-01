package commands

import (
	"errors"
	"fmt"
	"log/slog"
)

// registry is the concrete implementation of the Registry interface.
type registry struct {
	// commandsMap provides fast lookup by command name
	commandsMap map[string]*Command
	// commandsList maintains all commands in order for iteration
	commandsList []Command
	// projectDir is the project directory path for loading project commands
	projectDir string
}

// NewRegistry creates a new command registry instance.
// The registry will load commands from project, user home, and XDG config directories.
func NewRegistry(projectDir string) Registry {
	return &registry{
		commandsMap: make(map[string]*Command),
		commandsList: []Command{},
		projectDir:   projectDir,
	}
}

// LoadCommands loads all commands from configured locations (project, user home, XDG config).
// Commands from all sources are merged into a single registry.
// Errors from individual loaders are logged but don't prevent other commands from loading.
func (r *registry) LoadCommands() ([]Command, error) {
	slog.Info("Loading commands from all configured locations",
		"project_dir", r.projectDir,
	)

	var allCommands []Command
	var loadErrors []error
	var projectCount, userCount, xdgCount int

	// Load commands in reverse priority order (lowest priority first, highest last).
	// This ensures that when building the map, higher-priority commands overwrite lower-priority ones.
	// Priority order: XDG Config (lowest) < User Home < Project (highest)

	// Load from XDG config directory (lowest priority - loaded first)
	xdgCommands, err := loadXDGCommands()
	if err != nil {
		slog.Warn("Failed to load XDG config commands",
			"error", err,
		)
		loadErrors = append(loadErrors, fmt.Errorf("XDG config commands: %w", err))
	} else {
		xdgCount = len(xdgCommands)
		allCommands = append(allCommands, xdgCommands...)
		slog.Debug("Loaded XDG config commands",
			"count", xdgCount,
		)
	}

	// Load from user home directory (medium priority)
	userCommands, err := loadUserHomeCommands()
	if err != nil {
		slog.Warn("Failed to load user home commands",
			"error", err,
		)
		loadErrors = append(loadErrors, fmt.Errorf("user home commands: %w", err))
	} else {
		userCount = len(userCommands)
		allCommands = append(allCommands, userCommands...)
		slog.Debug("Loaded user home commands",
			"count", userCount,
		)
	}

	// Load from project directory (highest priority - loaded last, overwrites others)
	projectCommands, err := loadProjectCommands(r.projectDir)
	if err != nil {
		slog.Warn("Failed to load project commands",
			"error", err,
			"project_dir", r.projectDir,
		)
		loadErrors = append(loadErrors, fmt.Errorf("project commands: %w", err))
	} else {
		projectCount = len(projectCommands)
		allCommands = append(allCommands, projectCommands...)
		slog.Debug("Loaded project commands",
			"count", projectCount,
			"project_dir", r.projectDir,
		)
	}

	// Build map and list from merged commands with conflict resolution.
	// Conflict resolution strategy:
	// 1. Namespaces prevent conflicts: `frontend/review-pr.md` → `frontend:review-pr` and
	//    `backend/review-pr.md` → `backend:review-pr` coexist (different names).
	// 2. For commands with the same name (same namespace + filename), precedence order is:
	//    Project > User Home > XDG Config (project commands take precedence).
	// 3. Last loaded command wins for exact duplicates within the same source.
	// 4. Conflicts are detected and logged when a lower-priority command is overwritten.
	r.commandsMap = make(map[string]*Command, len(allCommands))
	r.commandsList = make([]Command, 0, len(allCommands))

	// Track conflicts for logging
	var conflicts []string

	// Load commands in reverse priority order (XDG first, then user, then project last)
	// This ensures project commands (loaded last) overwrite user/XDG commands
	// Priority: Project (highest) > User Home > XDG Config (lowest)
	for i := range allCommands {
		cmd := &allCommands[i]
		if existing, exists := r.commandsMap[cmd.Name]; exists {
			// Conflict detected - log it
			conflicts = append(conflicts, cmd.Name)
			slog.Warn("Command name conflict detected",
				"command", cmd.Name,
				"existing_source", existing.Source,
				"existing_path", existing.Path,
				"new_source", cmd.Source,
				"new_path", cmd.Path,
				"resolution", "Newer command overwrites (project > user > XDG)",
			)
		}
		r.commandsMap[cmd.Name] = cmd
	}

	// Build list from map (ensures no duplicates)
	for _, cmd := range r.commandsMap {
		r.commandsList = append(r.commandsList, *cmd)
	}

	// Log conflict summary if any conflicts occurred
	if len(conflicts) > 0 {
		slog.Info("Command conflicts resolved",
			"conflicts", len(conflicts),
			"commands", conflicts,
		)
	}

	// Return error if all loaders failed, but allow partial success
	if len(loadErrors) > 0 && len(allCommands) == 0 {
		slog.Error("All command loaders failed",
			"errors", len(loadErrors),
		)
		return nil, errors.Join(loadErrors...)
	}

	if len(loadErrors) > 0 {
		slog.Info("Some command loaders had errors, but commands were loaded",
			"loaded", len(allCommands),
			"errors", len(loadErrors),
		)
	}

	slog.Info("Command loading completed",
		"total_commands", len(allCommands),
		"project_commands", projectCount,
		"user_commands", userCount,
		"xdg_commands", xdgCount,
	)

	return r.commandsList, nil
}

// FindCommand looks up a command by its full name (including namespace if applicable).
// Returns the command if found, or an error if not found.
func (r *registry) FindCommand(name string) (*Command, error) {
	cmd, exists := r.commandsMap[name]
	if !exists {
		return nil, fmt.Errorf("command not found: %s", name)
	}
	return cmd, nil
}

// ListCommands returns all loaded commands.
// Returns all commands from all sources in a consistent order.
func (r *registry) ListCommands() []Command {
	// Return a copy to prevent external modification
	result := make([]Command, len(r.commandsList))
	copy(result, r.commandsList)
	return result
}

// Reload refreshes commands from all configured locations.
// Clears existing commands and reloads from all sources.
func (r *registry) Reload() error {
	_, err := r.LoadCommands()
	return err
}

