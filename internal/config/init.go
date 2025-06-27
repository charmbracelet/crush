package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// InitFlagFilename is the name of the file that indicates whether the project has been initialized
	InitFlagFilename = "init"
)

// ProjectInitFlag represents the initialization status for a project directory
type ProjectInitFlag struct {
	Initialized bool `json:"initialized"`
}

// ProjectNeedsInitialization checks if the current project needs initialization
func ProjectNeedsInitialization() (bool, error) {
	if instance == nil {
		return false, fmt.Errorf("config not loaded")
	}

	flagFilePath := filepath.Join(instance.Options.DataDirectory, InitFlagFilename)

	// Check if the flag file exists
	_, err := os.Stat(flagFilePath)
	if err == nil {
		return false, nil
	}

	if !os.IsNotExist(err) {
		return false, fmt.Errorf("failed to check init flag file: %w", err)
	}

	// Check if any variation of CRUSH.md already exists in working directory
	crushExists, err := crushMdExists(WorkingDirectory())
	if err != nil {
		return false, fmt.Errorf("failed to check for CRUSH.md files: %w", err)
	}
	if crushExists {
		return false, nil
	}

	return true, nil
}

// crushMdExists checks if any case variation of crush.md exists in the directory
func crushMdExists(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := strings.ToLower(entry.Name())
		if name == "crush.md" {
			return true, nil
		}
	}

	return false, nil
}

// MarkProjectInitialized marks the current project as initialized
func MarkProjectInitialized() error {
	if instance == nil {
		return fmt.Errorf("config not loaded")
	}
	flagFilePath := filepath.Join(instance.Options.DataDirectory, InitFlagFilename)

	file, err := os.Create(flagFilePath)
	if err != nil {
		return fmt.Errorf("failed to create init flag file: %w", err)
	}
	defer file.Close()

	return nil
}
