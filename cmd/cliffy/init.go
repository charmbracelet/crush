package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bwl/cliffy/internal/config"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize cliffy configuration interactively",
	Long: `Initialize cliffy with an interactive setup guide.

This command helps you:
  - Set up your API key for OpenRouter (or other providers)
  - Create a sample cliffy.json configuration
  - Verify your configuration works

The configuration will be saved to ~/.config/cliffy/cliffy.json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInit()
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit() error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("🎾 Cliffy Initialization")
	fmt.Println("=======================")
	fmt.Println()

	// Check if already initialized
	globalConfigPath := config.GlobalConfig()
	if _, err := os.Stat(globalConfigPath); err == nil {
		fmt.Printf("Configuration already exists at: %s\n", globalConfigPath)
		fmt.Print("Overwrite? (y/N): ")
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// Guide user through provider selection
	fmt.Println("Let's set up your AI provider.")
	fmt.Println()
	fmt.Println("Recommended: OpenRouter (free tier available)")
	fmt.Println("  - Get API key: https://openrouter.ai/settings/keys")
	fmt.Println("  - Free models: x-ai/grok-4-fast:free")
	fmt.Println()
	fmt.Println("Other options: OpenAI, Anthropic, Google Gemini")
	fmt.Println()

	// Ask for provider choice
	fmt.Print("Choose provider (openrouter/openai/anthropic/gemini) [openrouter]: ")
	providerInput, _ := reader.ReadString('\n')
	providerInput = strings.TrimSpace(strings.ToLower(providerInput))
	if providerInput == "" {
		providerInput = "openrouter"
	}

	var providerID, apiKeyEnv, baseURL, modelLarge, modelSmall string
	switch providerInput {
	case "openrouter":
		providerID = "openrouter"
		apiKeyEnv = "CLIFFY_OPENROUTER_API_KEY"
		baseURL = "https://openrouter.ai/api/v1"
		modelLarge = "x-ai/grok-4-fast:free"
		modelSmall = "x-ai/grok-4-fast:free"
	case "openai":
		providerID = "openai"
		apiKeyEnv = "OPENAI_API_KEY"
		baseURL = "https://api.openai.com/v1"
		modelLarge = "gpt-4o"
		modelSmall = "gpt-4o-mini"
	case "anthropic":
		providerID = "anthropic"
		apiKeyEnv = "ANTHROPIC_API_KEY"
		baseURL = "https://api.anthropic.com/v1"
		modelLarge = "claude-3-5-sonnet-20241022"
		modelSmall = "claude-3-5-haiku-20241022"
	case "gemini":
		providerID = "gemini"
		apiKeyEnv = "GEMINI_API_KEY"
		baseURL = "https://generativelanguage.googleapis.com/v1beta"
		modelLarge = "gemini-2.0-flash-exp"
		modelSmall = "gemini-2.0-flash-exp"
	default:
		return fmt.Errorf("unsupported provider: %s", providerInput)
	}

	// Check for existing API key in environment
	existingKey := os.Getenv(apiKeyEnv)
	var apiKey string

	if existingKey != "" {
		fmt.Printf("✓ Found %s in environment\n", apiKeyEnv)
		apiKey = fmt.Sprintf("${%s}", apiKeyEnv)
	} else {
		fmt.Printf("\nNo %s found in environment.\n", apiKeyEnv)
		fmt.Print("Enter API key (or leave empty to use env var later): ")
		keyInput, _ := reader.ReadString('\n')
		keyInput = strings.TrimSpace(keyInput)

		if keyInput == "" {
			apiKey = fmt.Sprintf("${%s}", apiKeyEnv)
			fmt.Printf("\n⚠️  Remember to set %s before using cliffy:\n", apiKeyEnv)
			fmt.Printf("   export %s=\"your-api-key\"\n", apiKeyEnv)
		} else {
			apiKey = keyInput
		}
	}

	// Create config structure
	cfg := createSampleConfig(providerID, apiKey, baseURL, modelLarge, modelSmall)

	// Ensure config directory exists
	configDir := filepath.Dir(globalConfigPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write config file
	configJSON, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(globalConfigPath, configJSON, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	fmt.Println()
	fmt.Printf("✓ Configuration saved to: %s\n", globalConfigPath)
	fmt.Println()

	// Mark as initialized in data directory
	dataDir := filepath.Join(configDir, ".cliffy")
	if err := os.MkdirAll(dataDir, 0755); err == nil {
		initFlagPath := filepath.Join(dataDir, config.InitFlagFilename)
		os.WriteFile(initFlagPath, []byte(""), 0644)
	}

	// Test configuration
	fmt.Println("Testing configuration...")
	if existingKey != "" || strings.Contains(apiKey, "${") {
		// Skip test if using env var and it's set
		if os.Getenv(apiKeyEnv) != "" {
			fmt.Println("✓ Configuration looks good!")
			fmt.Println()
			fmt.Println("Try it out:")
			fmt.Println("  cliffy \"what is 2+2?\"")
		} else {
			fmt.Printf("⚠️  Set your API key first:\n")
			fmt.Printf("   export %s=\"your-api-key\"\n", apiKeyEnv)
			fmt.Println()
			fmt.Println("Then try:")
			fmt.Println("  cliffy \"what is 2+2?\"")
		}
	} else {
		fmt.Println("✓ Configuration complete!")
		fmt.Println()
		fmt.Println("Try it out:")
		fmt.Println("  cliffy \"what is 2+2?\"")
	}

	fmt.Println()
	fmt.Println("For troubleshooting, run: cliffy doctor")

	return nil
}

func createSampleConfig(providerID, apiKey, baseURL, modelLarge, modelSmall string) map[string]interface{} {
	return map[string]interface{}{
		"$schema": "https://cliffy.ettio.com/schema.json",
		"models": map[string]interface{}{
			"large": map[string]interface{}{
				"provider": providerID,
				"model":    modelLarge,
			},
			"small": map[string]interface{}{
				"provider": providerID,
				"model":    modelSmall,
			},
		},
		"providers": map[string]interface{}{
			providerID: map[string]interface{}{
				"api_key":  apiKey,
				"base_url": baseURL,
			},
		},
		"options": map[string]interface{}{
			"context_paths": []string{
				".cursorrules",
				"CLAUDE.md",
				".github/copilot-instructions.md",
			},
		},
	}
}
