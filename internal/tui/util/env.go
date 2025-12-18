package util

import (
	"os"
	"runtime"
	"strings"
	"sync"
)

type envInfo struct {
	isWindows         bool
	isWSL             bool
	isWindowsTerminal bool
}

var (
	envOnce sync.Once
	env     envInfo
)

// IsWindows returns true when running on Windows.
func IsWindows() bool {
	initEnv()
	return env.isWindows
}

// IsWSL returns true when running inside Windows Subsystem for Linux.
func IsWSL() bool {
	initEnv()
	return env.isWSL
}

// IsWindowsTerminal returns true when running inside Windows Terminal.
func IsWindowsTerminal() bool {
	initEnv()
	return env.isWindowsTerminal
}

func initEnv() {
	envOnce.Do(func() {
		env.isWindows = runtime.GOOS == "windows"
		env.isWSL = detectWSL()
		env.isWindowsTerminal = detectWindowsTerminal()
	})
}

func detectWSL() bool {
	if runtime.GOOS != "linux" {
		return false
	}

	if os.Getenv("WSL_DISTRO_NAME") != "" {
		return true
	}

	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}

	version := strings.ToLower(string(data))
	return strings.Contains(version, "microsoft") || strings.Contains(version, "wsl")
}

func detectWindowsTerminal() bool {
	return os.Getenv("WT_SESSION") != ""
}
