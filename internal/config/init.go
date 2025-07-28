package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	InitFlagFilename = "init"
)

type ProjectInitFlag struct {
	Initialized bool `json:"initialized"`
}

func ProjectNeedsInitialization(cfg *Config) (bool, error) {
	if cfg == nil {
		return false, fmt.Errorf("config not loaded")
	}

	flagFilePath := filepath.Join(cfg.Options.DataDirectory, InitFlagFilename)

	_, err := os.Stat(flagFilePath)
	if err == nil {
		return false, nil
	}

	if !os.IsNotExist(err) {
		return false, fmt.Errorf("failed to check init flag file: %w", err)
	}

	crushExists, err := crushMdExists(cfg.WorkingDir())
	if err != nil {
		return false, fmt.Errorf("failed to check for CRUSH.md files: %w", err)
	}
	if crushExists {
		return false, nil
	}

	return true, nil
}

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

func MarkProjectInitialized(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}
	flagFilePath := filepath.Join(cfg.Options.DataDirectory, InitFlagFilename)

	file, err := os.Create(flagFilePath)
	if err != nil {
		return fmt.Errorf("failed to create init flag file: %w", err)
	}
	defer file.Close()

	return nil
}

func HasInitialDataConfig(cfg *Config) bool {
	if cfg == nil {
		return false
	}
	cfgPath := GlobalConfigData()
	if _, err := os.Stat(cfgPath); err != nil {
		return false
	}
	return cfg.IsConfigured()
}
