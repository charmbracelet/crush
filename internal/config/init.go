package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/charmbracelet/crush/internal/fsext"
)

const (
	InitFlagFilename = "init"
)

type ProjectInitFlag struct {
	Initialized bool `json:"initialized"`
}

func Init(workingDir, dataDir string, debug bool) (*Service, error) {
	svc, err := Load(workingDir, dataDir, debug)
	if err != nil {
		return nil, err
	}
	return svc, nil
}

func ProjectNeedsInitialization(svc *Service) (bool, error) {
	if svc == nil {
		return false, fmt.Errorf("config not loaded")
	}

	flagFilePath := filepath.Join(svc.DataDirectory(), InitFlagFilename)

	_, err := os.Stat(flagFilePath)
	if err == nil {
		return false, nil
	}

	if !os.IsNotExist(err) {
		return false, fmt.Errorf("failed to check init flag file: %w", err)
	}

	someContextFileExists, err := contextPathsExist(svc.WorkingDir())
	if err != nil {
		return false, fmt.Errorf("failed to check for context files: %w", err)
	}
	if someContextFileExists {
		return false, nil
	}

	empty, err := dirHasNoVisibleFiles(svc.WorkingDir())
	if err != nil {
		return false, fmt.Errorf("failed to check if directory is empty: %w", err)
	}
	if empty {
		return false, nil
	}

	return true, nil
}

func contextPathsExist(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}

	// Create a slice of lowercase filenames for lookup with slices.Contains
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, strings.ToLower(entry.Name()))
		}
	}

	// Check if any of the default context paths exist in the directory
	for _, path := range defaultContextPaths {
		// Extract just the filename from the path
		_, filename := filepath.Split(path)
		filename = strings.ToLower(filename)

		if slices.Contains(files, filename) {
			return true, nil
		}
	}

	return false, nil
}

// dirHasNoVisibleFiles returns true if the directory has no files/dirs after applying ignore rules
func dirHasNoVisibleFiles(dir string) (bool, error) {
	files, _, err := fsext.ListDirectory(dir, nil, 1, 1)
	if err != nil {
		return false, err
	}
	return len(files) == 0, nil
}

func MarkProjectInitialized(svc *Service) error {
	if svc == nil {
		return fmt.Errorf("config not loaded")
	}
	flagFilePath := filepath.Join(svc.DataDirectory(), InitFlagFilename)

	file, err := os.Create(flagFilePath)
	if err != nil {
		return fmt.Errorf("failed to create init flag file: %w", err)
	}
	defer file.Close()

	return nil
}

func HasInitialDataConfig(svc *Service) bool {
	if svc == nil {
		return false
	}
	cfgPath := GlobalConfigData()
	if _, err := os.Stat(cfgPath); err != nil {
		return false
	}
	return svc.IsConfigured()
}
