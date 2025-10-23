package startup

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Manager handles application startup tasks
type Manager struct {
	workspaceRoot string
	configPath    string
}

// NewManager creates a new startup manager
func NewManager(workspaceRoot string) *Manager {
	return &Manager{
		workspaceRoot: workspaceRoot,
		configPath:    filepath.Join(workspaceRoot, ".crush", "crush.json"),
	}
}

// RunStartupTasks performs all startup tasks
func (m *Manager) RunStartupTasks() error {
	// Run silently without output to avoid terminal corruption
	return nil
}


// isOllamaRunning checks if Ollama API is responding
func (m *Manager) isOllamaRunning() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "curl", "-s", "http://localhost:11434/api/tags")
	err := cmd.Run()
	return err == nil
}

// startOllama starts the Ollama service
func (m *Manager) startOllama() error {
	// Check if ollama command exists
	_, err := exec.LookPath("ollama")
	if err != nil {
		return fmt.Errorf("ollama command not found: %w", err)
	}

	// Start ollama in background
	cmd := exec.Command("ollama", "serve")
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ollama: %w", err)
	}

	return nil
}

// waitForOllamaReady waits for Ollama API to be ready
func (m *Manager) waitForOllamaReady() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if m.isOllamaRunning() {
				return nil
			}
		}
	}
}

// ensureNarratorConfig ensures narrator configuration is set up properly
func (m *Manager) ensureNarratorConfig() error {
	fmt.Println("🤖 Setting up narrator configuration...")

	// Check if config file exists
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		fmt.Println("📝 Creating default config file...")
		if err := m.createDefaultConfig(); err != nil {
			return fmt.Errorf("failed to create default config: %w", err)
		}
	}

	// Check for recommended lightweight model (3B parameters max)
	recommendedModel := "llama3.2:3b"
	if err := m.ensureModelAvailable(recommendedModel); err != nil {
		fmt.Printf("⚠️ Could not ensure %s model: %v\n", recommendedModel, err)
		fmt.Println("💡 Using default llama2:3b model")
		recommendedModel = "llama2:3b"
	}

	// Update config with optimal settings
	if err := m.updateNarratorConfig(recommendedModel); err != nil {
		return fmt.Errorf("failed to update narrator config: %w", err)
	}

	fmt.Printf("✅ Narrator configured with model: %s\n", recommendedModel)
	return nil
}

// ensureModelAvailable checks if a model is available and pulls it if needed
func (m *Manager) ensureModelAvailable(modelName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check if model is already available
	cmd := exec.CommandContext(ctx, "ollama", "list")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list models: %w", err)
	}

	if strings.Contains(string(output), modelName) {
		fmt.Printf("✅ Model %s is already available\n", modelName)
		return nil
	}

	// Pull the model
	fmt.Printf("📥 Pulling model %s...\n", modelName)
	cmd = exec.CommandContext(ctx, "ollama", "pull", modelName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to pull model %s: %w", modelName, err)
	}

	fmt.Printf("✅ Successfully pulled model %s\n", modelName)
	return nil
}

// createDefaultConfig creates a default configuration file
func (m *Manager) createDefaultConfig() error {
	configDir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Create minimal working configuration for Ollama
	defaultConfig := map[string]interface{}{
		"schema": "https://github.com/charmbracelet/crush/config/config.json",
		"models": map[string]interface{}{
			"large": map[string]interface{}{
				"model":    "llama3.2:3b",
				"provider": "ollama",
			},
		},
		"options": map[string]interface{}{
			"tui": map[string]interface{}{
				"theme": "charmtone",
			},
			"narrator": map[string]interface{}{
				"enabled":         true,
				"ollama_url":      "http://localhost:11434",
				"model":           "llama3.2:3b",
				"timeout_seconds": 30,
			},
		},
		"context_paths": []string{
			".cursorrules",
			"CRUSH.md",
		},
		"permissions": map[string]interface{}{
			"allowed_tools": []string{
				"view",
				"edit",
				"write",
				"bash",
			},
		},
		"attribution": map[string]interface{}{
			"co_authored_by": true,
			"generated_with": true,
		},
	}

	// Also configure an Ollama provider
	defaultConfig["providers"] = map[string]interface{}{
		"ollama": map[string]interface{}{
			"id":       "ollama",
			"name":     "Ollama",
			"type":     "openai",
			"base_url": "http://localhost:11434/v1",
			"api_key":  "ollama",
			"models": []map[string]interface{}{
				{
					"id":                 "llama3.2:3b",
					"name":               "Llama 3.2 3B",
					"context_window":     131072,
					"default_max_tokens": 4096,
				},
				{
					"id":                 "llama2:3b",
					"name":               "Llama 2 3B",
					"context_window":     4096,
					"default_max_tokens": 1024,
				},
			},
		},
	}

	data, err := json.MarshalIndent(defaultConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal default config: %w", err)
	}

	if err := os.WriteFile(m.configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write default config: %w", err)
	}

	return nil
}

// updateNarratorConfig updates the narrator configuration
func (m *Manager) updateNarratorConfig(modelName string) error {
	// Read existing config
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config JSON: %w", err)
	}

	// Ensure options section exists
	if config["options"] == nil {
		config["options"] = make(map[string]interface{})
	}
	options := config["options"].(map[string]interface{})

	// Update narrator configuration
	options["narrator"] = map[string]interface{}{
		"enabled":         true,
		"ollama_url":      "http://localhost:11434",
		"model":           modelName,
		"timeout_seconds": 30,
	}

	// Write updated config back as JSON
	updatedData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal updated config: %w", err)
	}

	if err := os.WriteFile(m.configPath, updatedData, 0644); err != nil {
		return fmt.Errorf("failed to write updated config: %w", err)
	}

	return nil
}

// GetRecommendedModel returns the recommended lightweight model for the user
func (m *Manager) GetRecommendedModel() string {
	// Priority order for recommended lightweight models (3B parameters max)
	models := []string{
		"llama3.2:3b",  // Latest and most capable lightweight version
		"llama3.1:3b",  // Previous stable lightweight version
		"llama3:3b",    // Original Llama 3 lightweight version
		"llama2:3b",    // Fallback lightweight version
		"qwen2.5:3b",   // Excellent lightweight alternative
		"gemma2:3b",    // Google's lightweight model
		"phi3:3b-mini", // Microsoft's very small model
	}

	for _, model := range models {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "ollama", "list")
		output, err := cmd.Output()
		if err != nil {
			continue
		}

		if strings.Contains(string(output), model) {
			return model
		}
	}

	return "llama2:3b" // Ultimate lightweight fallback
}
