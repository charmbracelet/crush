package cmd

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	pluginCmd.AddCommand(pluginInstallCmd)
	pluginCmd.AddCommand(pluginListCmd)
	pluginCmd.AddCommand(pluginUninstallCmd)
	rootCmd.AddCommand(pluginCmd)
}

var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Manage Crush plugins",
	Long:  "Install, list, and manage Crush plugins for extended functionality",
}

var pluginInstallCmd = &cobra.Command{
	Use:   "install <path|url>",
	Short: "Install a plugin",
	Long: `Install a plugin from a local directory or remote URL.

Plugins are installed to .crush/plugins/ directory in your project.
Dependency installation uses pnpm by default to save disk space.
If pnpm is not available, falls back to npm.

Examples:
  crush plugin install ./my-plugin            # Install from local directory
  crush plugin install https://github.com/user/plugin/archive/refs/heads/main.zip  # Install from URL

Tip: Install pnpm for better disk usage: npm install -g pnpm`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		source := args[0]
		workingDir, _ := cmd.Flags().GetString("cwd")
		if workingDir == "" {
			var err error
			workingDir, err = os.Getwd()
			if err != nil {
				return err
			}
		}

		return installPlugin(source, workingDir)
	},
}

var pluginListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed plugins",
	RunE: func(cmd *cobra.Command, args []string) error {
		workingDir, _ := cmd.Flags().GetString("cwd")
		if workingDir == "" {
			var err error
			workingDir, err = os.Getwd()
			if err != nil {
				return err
			}
		}

		return listPlugins(workingDir)
	},
}

var pluginUninstallCmd = &cobra.Command{
	Use:   "uninstall [plugin-name]",
	Short: "Uninstall a plugin",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pluginName := args[0]
		workingDir, _ := cmd.Flags().GetString("cwd")
		if workingDir == "" {
			var err error
			workingDir, err = os.Getwd()
			if err != nil {
				return err
			}
		}

		return uninstallPlugin(pluginName, workingDir)
	},
}

func installPlugin(source, workingDir string) error {
	// Create plugins directory if it doesn't exist.
	pluginsDir := filepath.Join(workingDir, ".crush", "plugins")
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create plugins directory: %w", err)
	}

	// Check if source is a local directory.
	if info, err := os.Stat(source); err == nil && info.IsDir() {
		return installFromLocalDir(source, pluginsDir)
	}

	// Check if source is a URL.
	if isURL(source) {
		return installFromURL(source, pluginsDir)
	}

	return fmt.Errorf("source must be a local directory path or a URL (got: %s)", source)
}

func installFromLocalDir(source, pluginsDir string) error {
	// Read plugin metadata from source directory.
	metadataPath := filepath.Join(source, "plugin.json")
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		return fmt.Errorf("plugin metadata not found: %s", metadataPath)
	}

	var metadata struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return fmt.Errorf("failed to read plugin metadata: %w", err)
	}
	if err := json.Unmarshal(data, &metadata); err != nil {
		return fmt.Errorf("invalid plugin metadata: %w", err)
	}

	// Validate plugin name to prevent path traversal.
	if err := validatePluginName(metadata.Name); err != nil {
		return fmt.Errorf("invalid plugin name: %w", err)
	}

	// Copy plugin to plugins directory (excluding node_modules).
	pluginDir := filepath.Join(pluginsDir, metadata.Name)

	// Additional safety check: ensure resolved path stays within plugins directory.
	cleanPluginDir := filepath.Clean(pluginDir) + string(filepath.Separator)
	cleanPluginsDir := filepath.Clean(pluginsDir) + string(filepath.Separator)
	if !strings.HasPrefix(cleanPluginDir, cleanPluginsDir) {
		return fmt.Errorf("plugin name resolves outside plugins directory: %s", metadata.Name)
	}

	if err := copyPluginDir(source, pluginDir); err != nil {
		return fmt.Errorf("failed to copy plugin: %w", err)
	}

	// Install dependencies if package.json exists.
	packageJSONPath := filepath.Join(pluginDir, "package.json")
	if _, err := os.Stat(packageJSONPath); err == nil {
		if err := installNPMDependencies(pluginDir); err != nil {
			return fmt.Errorf("failed to install dependencies: %w", err)
		}
	}

	fmt.Printf("Plugin %s installed successfully\n", metadata.Name)
	return nil
}

func installFromURL(pluginURL, pluginsDir string) error {
	fmt.Printf("Downloading plugin from %s...\n", pluginURL)

	// Create a temporary directory for download.
	tempDir, err := os.MkdirTemp("", "crush-plugin-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Download the archive.
	archivePath := filepath.Join(tempDir, "plugin.zip")
	if err := downloadFile(pluginURL, archivePath); err != nil {
		return fmt.Errorf("failed to download plugin: %w", err)
	}

	// Extract the archive.
	extractDir := filepath.Join(tempDir, "extracted")
	if err := extractZip(archivePath, extractDir); err != nil {
		return fmt.Errorf("failed to extract plugin: %w", err)
	}

	// Find the plugin directory (might be in a subdirectory like repo-main/).
	pluginSourceDir, err := findPluginDir(extractDir)
	if err != nil {
		return err
	}

	// Install from the extracted directory.
	return installFromLocalDir(pluginSourceDir, pluginsDir)
}

func downloadFile(url, dest string) error {
	resp, err := httpGet(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %s", resp.Status)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func httpGet(url string) (*http.Response, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	return client.Get(url)
}

func extractZip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)

		// Check for Zip Slip vulnerability: ensure the resolved path is within dest.
		cleanFpath := filepath.Clean(fpath)
		cleanDest := filepath.Clean(dest)
		if !strings.HasPrefix(cleanFpath, cleanDest+string(filepath.Separator)) {
			return fmt.Errorf("invalid file path in archive: %s (potential path traversal)", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fpath, f.Mode()); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), 0o755); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		// Limit decompression size to prevent zip bomb attacks (100 MB per file).
		const maxFileSize = 100 << 20 // 100 MB
		_, err = io.Copy(outFile, io.LimitReader(rc, maxFileSize))
		rc.Close()
		outFile.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

func findPluginDir(dir string) (string, error) {
	// Check if plugin.json exists in the root.
	if _, err := os.Stat(filepath.Join(dir, "plugin.json")); err == nil {
		return dir, nil
	}

	// Look for plugin.json in subdirectories (common for GitHub archives).
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			subdir := filepath.Join(dir, entry.Name())
			if _, err := os.Stat(filepath.Join(subdir, "plugin.json")); err == nil {
				return subdir, nil
			}
		}
	}

	return "", fmt.Errorf("plugin.json not found in archive")
}

func listPlugins(workingDir string) error {
	pluginsDir := filepath.Join(workingDir, ".crush", "plugins")
	if _, err := os.Stat(pluginsDir); os.IsNotExist(err) {
		fmt.Println("No plugins installed")
		return nil
	}

	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return fmt.Errorf("failed to read plugins directory: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("No plugins installed")
		return nil
	}

	fmt.Println("Installed plugins:")
	for _, entry := range entries {
		if entry.IsDir() {
			pluginJSONPath := filepath.Join(pluginsDir, entry.Name(), "plugin.json")
			if _, err := os.Stat(pluginJSONPath); err == nil {
				data, err := os.ReadFile(pluginJSONPath)
				if err == nil {
					var metadata struct {
						Name        string `json:"name"`
						Version     string `json:"version"`
						Description string `json:"description"`
					}
					if err := json.Unmarshal(data, &metadata); err == nil {
						fmt.Printf("  %s (v%s): %s\n", metadata.Name, metadata.Version, metadata.Description)
						continue
					}
				}
			}
			fmt.Printf("  %s\n", entry.Name())
		}
	}

	return nil
}

func uninstallPlugin(name, workingDir string) error {
	// Validate plugin name to prevent path traversal.
	if err := validatePluginName(name); err != nil {
		return fmt.Errorf("invalid plugin name: %w", err)
	}

	pluginsDir := filepath.Join(workingDir, ".crush", "plugins")
	pluginDir := filepath.Join(pluginsDir, name)

	// Ensure the resolved path is within the plugins directory to prevent path traversal.
	cleanPluginDir := filepath.Clean(pluginDir) + string(filepath.Separator)
	cleanPluginsDir := filepath.Clean(pluginsDir) + string(filepath.Separator)
	if !strings.HasPrefix(cleanPluginDir, cleanPluginsDir) {
		return fmt.Errorf("invalid plugin name: %s", name)
	}

	if _, err := os.Stat(pluginDir); os.IsNotExist(err) {
		return fmt.Errorf("plugin not found: %s", name)
	}

	if err := os.RemoveAll(pluginDir); err != nil {
		return fmt.Errorf("failed to remove plugin: %w", err)
	}

	fmt.Printf("Plugin %s uninstalled successfully\n", name)
	return nil
}

// validatePluginName ensures the plugin name is safe for use in file paths.
// It rejects empty names, names with path separators, and names that could
// resolve to the plugins directory itself (like "." or "..").
func validatePluginName(name string) error {
	if name == "" {
		return fmt.Errorf("plugin name cannot be empty")
	}
	if name == "." || name == ".." {
		return fmt.Errorf("plugin name cannot be '.' or '..'")
	}
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("plugin name cannot contain path separators")
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("plugin name cannot contain '..'")
	}
	return nil
}

func installNPMDependencies(dir string) error {
	// Check if node_modules already exists.
	if _, err := os.Stat(filepath.Join(dir, "node_modules")); err == nil {
		// node_modules already exists.
		return nil
	}

	// Prefer pnpm to avoid duplicate node_modules and save disk space.
	if _, err := exec.LookPath("pnpm"); err == nil {
		fmt.Printf("Installing dependencies with pnpm in %s...\n", dir)
		cmd := exec.Command("pnpm", "install")
		cmd.Dir = dir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("pnpm install failed: %w", err)
		}
		return nil
	}

	// Fallback to npm if pnpm is not available.
	if _, err := exec.LookPath("npm"); err == nil {
		fmt.Printf("pnpm not found, falling back to npm in %s...\n", dir)
		fmt.Println("Tip: Install pnpm for better disk usage: npm install -g pnpm")
		cmd := exec.Command("npm", "install")
		cmd.Dir = dir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("npm install failed: %w", err)
		}
		return nil
	}

	return fmt.Errorf("no package manager found. Please install pnpm (recommended) or npm")
}

// copyPluginDir copies a plugin directory, excluding node_modules to save space
// and avoid copying large dependency trees.
func copyPluginDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		// Skip node_modules directory.
		if entry.IsDir() && entry.Name() == "node_modules" {
			continue
		}

		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyPluginDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}
