package shell

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

var useGoCoreUtils bool

// nativeShellPaths stores detected native shell paths for extending available tools
var nativeShellPaths []string

func init() {
	// If CRUSH_CORE_UTILS is set to either true or false, respect that.
	// By default, enable on Windows only.
	if v, err := strconv.ParseBool(os.Getenv("CRUSH_CORE_UTILS")); err == nil {
		useGoCoreUtils = v
	} else {
		useGoCoreUtils = runtime.GOOS == "windows"
	}

	// Detect native shells (Git Bash, WSL) to extend available Unix tools
	if runtime.GOOS == "windows" {
		nativeShellPaths = detectNativeShellTools()
	}
}

// detectNativeShellTools detects Git Bash and WSL installations to extend
// the available Unix tools in the mvdan/sh interpreter.
func detectNativeShellTools() []string {
	var paths []string

	// Detect Git for Windows bash.exe
	gitBashPath := detectGitBash()
	if gitBashPath != "" {
		// Add both bin and usr/bin to PATH
		gitDir := filepath.Dir(gitBashPath) // C:\Program Files\Git\bin
		usrBinDir := filepath.Join(filepath.Dir(gitDir), "usr", "bin")

		if dirExists(gitDir) {
			paths = append(paths, gitDir)
		}
		if dirExists(usrBinDir) {
			paths = append(paths, usrBinDir)
		}
	}

	// Detect WSL bash (optional, can be enabled later)
	// wslBashPath := detectWSL()
	// if wslBashPath != "" {
	//     // WSL integration could be added here
	// }

	return paths
}

// detectGitBash searches for Git for Windows bash.exe in common installation locations.
func detectGitBash() string {
	candidates := []string{
		// Standard installation paths
		`C:\Program Files\Git\bin\bash.exe`,
		`C:\Program Files (x86)\Git\bin\bash.exe`,

		// Environment variable based paths
		filepath.Join(os.Getenv("ProgramFiles"), "Git", "bin", "bash.exe"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Git", "bin", "bash.exe"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "Git", "bin", "bash.exe"),
	}

	// Check explicit paths first
	for _, path := range candidates {
		if path != "" && fileExists(path) {
			return path
		}
	}

	// Try to find bash.exe in PATH (if Git bin directory is already in PATH)
	if path, err := exec.LookPath("bash.exe"); err == nil {
		return path
	}

	return ""
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// dirExists checks if a directory exists
func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// GetNativeShellPaths returns detected native shell paths for extending tool availability.
// This is used to augment the PATH environment variable with Unix tools from Git Bash/WSL.
func GetNativeShellPaths() []string {
	return nativeShellPaths
}

// HasNativeShellTools returns true if native shell tools (Git Bash, WSL) were detected.
func HasNativeShellTools() bool {
	return len(nativeShellPaths) > 0
}

// ExtendEnvWithNativeTools extends the given environment with paths to native shell tools.
// This allows the mvdan/sh interpreter to find Unix tools like grep, sed, awk, etc.
func ExtendEnvWithNativeTools(env []string) []string {
	if len(nativeShellPaths) == 0 {
		return env
	}

	// Find existing PATH variable (case-insensitive for Windows compatibility)
	pathIndex := -1
	existingPath := ""
	for i, e := range env {
		if len(e) > 5 && strings.EqualFold(e[:5], "PATH=") {
			pathIndex = i
			existingPath = e[5:]
			break
		}
	}

	// Prepend native shell paths to PATH
	newPath := strings.Join(nativeShellPaths, string(filepath.ListSeparator))
	if existingPath != "" {
		newPath = newPath + string(filepath.ListSeparator) + existingPath
	}

	if pathIndex >= 0 {
		// Update existing PATH
		env[pathIndex] = "PATH=" + newPath
	} else {
		// Add new PATH variable
		env = append(env, "PATH="+newPath)
	}

	return env
}
