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

// ShouldShowInitDialog checks if the initialization dialog should be shown for the current directory
func ShouldShowInitDialog() (bool, error) {
	if cfg == nil {
		return false, fmt.Errorf("config not loaded")
	}

	// Create the flag file path
	flagFilePath := filepath.Join(cfg.Data.Directory, InitFlagFilename)

	// Check if the flag file exists
	_, err := os.Stat(flagFilePath)
	if err == nil {
		// File exists, don't show the dialog
		return false, nil
	}

	// If the error is not "file not found", return the error
	if !os.IsNotExist(err) {
		return false, fmt.Errorf("failed to check init flag file: %w", err)
	}

	// Check if any variation of CRUSH.md already exists in working directory
	crushExists, err := crushMdExists(WorkingDirectory())
	if err != nil {
		return false, fmt.Errorf("failed to check for CRUSH.md files: %w", err)
	}
	if crushExists {
		// CRUSH.md already exists, don't show the dialog
		return false, nil
	}

	// File doesn't exist, show the dialog
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
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}
	// Create the flag file path
	flagFilePath := filepath.Join(cfg.Data.Directory, InitFlagFilename)

	// Create an empty file to mark the project as initialized
	file, err := os.Create(flagFilePath)
	if err != nil {
		return fmt.Errorf("failed to create init flag file: %w", err)
	}
	defer file.Close()

	return nil
}
