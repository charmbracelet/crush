package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/bwl/cliffy/internal/config"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check cliffy configuration health",
	Long: `Run diagnostics on your cliffy configuration.

This command checks:
  - Configuration file validity
  - Provider API keys and connectivity
  - Model availability
  - Common configuration issues

Use this to troubleshoot when cliffy isn't working as expected.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDoctor()
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor() error {
	fmt.Println("🩺 Cliffy Health Check")
	fmt.Println("=====================")
	fmt.Println()

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Check for config files
	fmt.Println("📁 Configuration Files")
	fmt.Println("----------------------")

	globalConfig := config.GlobalConfig()
	globalConfigData := config.GlobalConfigData()

	hasGlobalConfig := fileExists(globalConfig)
	hasGlobalData := fileExists(globalConfigData)

	if hasGlobalConfig {
		fmt.Printf("✓ Global config: %s\n", globalConfig)
	} else {
		fmt.Printf("✗ No global config found at: %s\n", globalConfig)
	}

	if hasGlobalData {
		fmt.Printf("✓ Global data config: %s\n", globalConfigData)
	} else {
		fmt.Printf("  (Optional) No data config at: %s\n", globalConfigData)
	}

	// Check for local configs
	localConfigs := []string{
		".cliffy.json",
		"cliffy.json",
	}
	hasLocalConfig := false
	for _, name := range localConfigs {
		path := cwd + "/" + name
		if fileExists(path) {
			fmt.Printf("✓ Local config: %s\n", path)
			hasLocalConfig = true
			break
		}
	}
	if !hasLocalConfig {
		fmt.Println("  (Optional) No local config in current directory")
	}

	if !hasGlobalConfig && !hasGlobalData && !hasLocalConfig {
		fmt.Println()
		fmt.Println("❌ No configuration found!")
		fmt.Println()
		fmt.Println("To fix this, run: cliffy init")
		return nil
	}

	fmt.Println()

	// Try to load config
	fmt.Println("⚙️  Configuration Loading")
	fmt.Println("------------------------")

	cfg, err := config.Init(cwd, ".cliffy", false)
	if err != nil {
		fmt.Printf("✗ Failed to load config: %v\n", err)
		fmt.Println()
		fmt.Println("Common issues:")
		fmt.Println("  - Invalid JSON syntax")
		fmt.Println("  - Missing required fields")
		fmt.Println()
		fmt.Println("Try: cliffy init (to create a fresh config)")
		return nil
	}

	fmt.Println("✓ Configuration loaded successfully")
	fmt.Println()

	// Check providers
	fmt.Println("🔌 Providers")
	fmt.Println("------------")

	if !cfg.IsConfigured() {
		fmt.Println("✗ No providers configured")
		fmt.Println()
		fmt.Println("To fix this, run: cliffy init")
		return nil
	}

	enabledProviders := cfg.EnabledProviders()
	if len(enabledProviders) == 0 {
		fmt.Println("✗ No enabled providers found")
		fmt.Println()
		fmt.Println("Check your config and ensure at least one provider has:")
		fmt.Println("  - Valid API key")
		fmt.Println("  - Valid base URL")
		fmt.Println("  - disable: false (or omit the field)")
		return nil
	}

	fmt.Printf("Found %d enabled provider(s):\n", len(enabledProviders))

	for _, provider := range enabledProviders {
		fmt.Printf("\n  Provider: %s\n", provider.Name)
		fmt.Printf("    ID: %s\n", provider.ID)
		fmt.Printf("    Type: %s\n", provider.Type)
		fmt.Printf("    Base URL: %s\n", provider.BaseURL)

		// Check API key
		apiKey := provider.APIKey
		if strings.HasPrefix(apiKey, "${") && strings.HasSuffix(apiKey, "}") {
			// Environment variable reference
			envVar := strings.TrimSuffix(strings.TrimPrefix(apiKey, "${"), "}")
			resolvedKey, err := cfg.Resolve(apiKey)
			if err != nil || resolvedKey == "" {
				fmt.Printf("    ✗ API Key: %s not set in environment\n", envVar)
				fmt.Printf("      Set it with: export %s=\"your-key\"\n", envVar)
			} else {
				fmt.Printf("    ✓ API Key: %s (from %s)\n", maskKey(resolvedKey), envVar)
			}
		} else if apiKey != "" {
			fmt.Printf("    ✓ API Key: %s (hardcoded in config)\n", maskKey(apiKey))
		} else {
			fmt.Println("    ✗ API Key: not set")
		}

		// Test connection if we have a valid key
		resolvedKey, err := cfg.Resolve(provider.APIKey)
		if err == nil && resolvedKey != "" {
			fmt.Print("    Testing connection... ")
			if err := provider.TestConnection(cfg.Resolver()); err != nil {
				fmt.Printf("✗ Failed\n")
				fmt.Printf("      Error: %v\n", err)
			} else {
				fmt.Println("✓ Connected")
			}
		}

		// Check models
		if len(provider.Models) > 0 {
			fmt.Printf("    Models: %d available\n", len(provider.Models))
		} else {
			fmt.Println("    ✗ No models configured")
		}
	}

	fmt.Println()

	// Check model selection
	fmt.Println("🤖 Model Selection")
	fmt.Println("------------------")

	largeModel := cfg.LargeModel()
	smallModel := cfg.SmallModel()

	if largeModel != nil {
		selectedLarge := cfg.Models[config.SelectedModelTypeLarge]
		fmt.Printf("✓ Large model: %s (%s)\n", largeModel.Name, selectedLarge.Provider)
	} else {
		fmt.Println("✗ No large model selected")
	}

	if smallModel != nil {
		selectedSmall := cfg.Models[config.SelectedModelTypeSmall]
		fmt.Printf("✓ Small model: %s (%s)\n", smallModel.Name, selectedSmall.Provider)
	} else {
		fmt.Println("✗ No small model selected")
	}

	fmt.Println()

	// Check context paths
	fmt.Println("📝 Context Files")
	fmt.Println("----------------")

	if cfg.Options != nil && len(cfg.Options.ContextPaths) > 0 {
		foundAny := false
		for _, path := range cfg.Options.ContextPaths {
			fullPath := cwd + "/" + path
			if fileExists(fullPath) {
				fmt.Printf("✓ %s\n", path)
				foundAny = true
			}
		}
		if !foundAny {
			fmt.Println("  No context files found (this is optional)")
			fmt.Println("  Create .cursorrules or CLAUDE.md to provide AI context")
		}
	} else {
		fmt.Println("  No context paths configured")
	}

	fmt.Println()

	// Summary
	fmt.Println("✅ Summary")
	fmt.Println("----------")

	if largeModel != nil || smallModel != nil {
		fmt.Println("Your configuration looks good! Try:")
		fmt.Println("  cliffy \"what is 2+2?\"")
	} else {
		fmt.Println("Configuration has issues. Run: cliffy init")
	}

	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func maskKey(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	return key[:4] + "..." + key[len(key)-4:]
}
