package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

var testConfigDir string

func baseConfigPath() string {
	if testConfigDir != "" {
		return testConfigDir
	}

	xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfigHome != "" {
		return filepath.Join(xdgConfigHome, "crush")
	}

	// return the path to the main config directory
	// for windows, it should be in `%LOCALAPPDATA%/crush/`
	// for linux and macOS, it should be in `$HOME/.config/crush/`
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
		}
		return filepath.Join(localAppData, appName)
	}

	return filepath.Join(os.Getenv("HOME"), ".config", appName)
}

func baseDataPath() string {
	if testConfigDir != "" {
		return testConfigDir
	}

	xdgDataHome := os.Getenv("XDG_DATA_HOME")
	if xdgDataHome != "" {
		return filepath.Join(xdgDataHome, appName)
	}

	// return the path to the main data directory
	// for windows, it should be in `%LOCALAPPDATA%/crush/`
	// for linux and macOS, it should be in `$HOME/.local/share/crush/`
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
		}
		return filepath.Join(localAppData, appName)
	}

	return filepath.Join(os.Getenv("HOME"), ".local", "share", appName)
}

func ConfigPath() string {
	return filepath.Join(baseConfigPath(), fmt.Sprintf("%s.json", appName))
}

func CrushInitialized() bool {
	cfgPath := ConfigPath()
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		// config file does not exist, so Crush is not initialized
		return false
	}
	return true
}
