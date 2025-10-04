package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/bwl/cliffy/internal/config"
	"github.com/bwl/cliffy/internal/preset"
	"github.com/spf13/cobra"
)

var presetCmd = &cobra.Command{
	Use:   "preset",
	Short: "Manage task presets",
	Long: `Manage curated task presets for common use cases.

Presets provide pre-configured model settings, tool access, and context
for specialized tasks like security reviews, code refactoring, or testing.`,
}

var presetListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available presets",
	Long: `List all available presets with their descriptions.

Presets are curated configurations for common use cases including:
- Quality assurance and code review
- Security auditing
- Performance analysis
- Code refactoring
- Documentation generation
- Test generation`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runPresetList()
	},
}

var presetShowCmd = &cobra.Command{
	Use:   "show <preset-id>",
	Short: "Show details of a specific preset",
	Long:  `Display detailed information about a specific preset including configuration and examples.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runPresetShow(args[0])
	},
}

var presetApplyCmd = &cobra.Command{
	Use:   "apply <preset-id>",
	Short: "Apply a preset to the current configuration",
	Long: `Apply a preset to the current project configuration.

This updates your local cliffy.json to use the preset's settings
as defaults for this project.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runPresetApply(args[0])
	},
}

func init() {
	presetCmd.AddCommand(presetListCmd)
	presetCmd.AddCommand(presetShowCmd)
	presetCmd.AddCommand(presetApplyCmd)
	rootCmd.AddCommand(presetCmd)
}

func runPresetList() error {
	mgr, err := preset.NewManager()
	if err != nil {
		return fmt.Errorf("failed to initialize preset manager: %w", err)
	}

	presetsByCategory := mgr.ListByCategory()
	if len(presetsByCategory) == 0 {
		fmt.Println("No presets available.")
		return nil
	}

	fmt.Println("Available Presets:")
	fmt.Println()

	// Define category order
	categoryOrder := []string{"quality", "security", "performance", "refactoring", "testing", "documentation", "general"}
	categoryNames := map[string]string{
		"quality":       "Quality Assurance",
		"security":      "Security",
		"performance":   "Performance",
		"refactoring":   "Refactoring",
		"testing":       "Testing",
		"documentation": "Documentation",
		"general":       "General",
	}

	for _, category := range categoryOrder {
		presets, ok := presetsByCategory[category]
		if !ok || len(presets) == 0 {
			continue
		}

		categoryName := categoryNames[category]
		if categoryName == "" {
			categoryName = strings.Title(category)
		}

		fmt.Printf("## %s\n\n", categoryName)

		for _, p := range presets {
			fmt.Printf("  %s\n", p.ID)
			fmt.Printf("    Name: %s\n", p.Name)
			fmt.Printf("    %s\n", p.Description)

			// Show key configuration
			features := []string{}
			if p.Model == config.SelectedModelTypeSmall {
				features = append(features, "fast model")
			} else {
				features = append(features, "large model")
			}
			if p.Think {
				features = append(features, "reasoning")
			}
			if p.ReasoningEffort != "" {
				features = append(features, p.ReasoningEffort+" effort")
			}
			if len(features) > 0 {
				fmt.Printf("    Config: %s\n", strings.Join(features, ", "))
			}

			fmt.Println()
		}
	}

	fmt.Println("Usage:")
	fmt.Println("  cliffy --preset <preset-id> \"your task\"")
	fmt.Println("  cliffy preset show <preset-id>")
	fmt.Println("  cliffy preset apply <preset-id>")
	fmt.Println()

	return nil
}

func runPresetShow(presetID string) error {
	mgr, err := preset.NewManager()
	if err != nil {
		return fmt.Errorf("failed to initialize preset manager: %w", err)
	}

	p, err := mgr.Get(presetID)
	if err != nil {
		return fmt.Errorf("preset not found: %s", presetID)
	}

	fmt.Printf("Preset: %s\n", p.Name)
	fmt.Println(strings.Repeat("=", len(p.Name)+8))
	fmt.Println()
	fmt.Printf("ID: %s\n", p.ID)
	fmt.Printf("Category: %s\n", p.Category)
	fmt.Printf("Description: %s\n", p.Description)
	fmt.Println()

	fmt.Println("Configuration:")
	fmt.Printf("  Model: %s\n", p.Model)
	if p.MaxTokens > 0 {
		fmt.Printf("  Max Tokens: %d\n", p.MaxTokens)
	}
	if p.ReasoningEffort != "" {
		fmt.Printf("  Reasoning Effort: %s\n", p.ReasoningEffort)
	}
	if p.Think {
		fmt.Printf("  Thinking Mode: enabled\n")
	}
	if p.MaxConcurrent > 0 {
		fmt.Printf("  Max Concurrent: %d\n", p.MaxConcurrent)
	}
	if p.FailFast {
		fmt.Printf("  Fail Fast: enabled\n")
	}
	fmt.Println()

	if len(p.AllowedTools) > 0 {
		fmt.Println("Allowed Tools:")
		for _, tool := range p.AllowedTools {
			fmt.Printf("  - %s\n", tool)
		}
		fmt.Println()
	}

	if len(p.DisabledTools) > 0 {
		fmt.Println("Disabled Tools:")
		for _, tool := range p.DisabledTools {
			fmt.Printf("  - %s\n", tool)
		}
		fmt.Println()
	}

	if len(p.ContextPaths) > 0 {
		fmt.Println("Context Paths:")
		for _, path := range p.ContextPaths {
			fmt.Printf("  - %s\n", path)
		}
		fmt.Println()
	}

	if p.SystemPromptPrefix != "" {
		fmt.Println("System Prompt Prefix:")
		// Indent the prefix
		lines := strings.Split(p.SystemPromptPrefix, "\n")
		for _, line := range lines {
			if line != "" {
				fmt.Printf("  %s\n", line)
			}
		}
		fmt.Println()
	}

	if len(p.Examples) > 0 {
		fmt.Println("Examples:")
		for _, example := range p.Examples {
			fmt.Printf("  %s\n", example)
		}
		fmt.Println()
	}

	// Show JSON representation
	fmt.Println("JSON Configuration:")
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal preset: %w", err)
	}
	fmt.Println(string(data))

	return nil
}

func runPresetApply(presetID string) error {
	mgr, err := preset.NewManager()
	if err != nil {
		return fmt.Errorf("failed to initialize preset manager: %w", err)
	}

	p, err := mgr.Get(presetID)
	if err != nil {
		return fmt.Errorf("preset not found: %s", presetID)
	}

	// Load current config
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	cfg, err := config.Init(cwd, ".cliffy", false)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Printf("Applying preset '%s' to project configuration...\n", p.Name)
	fmt.Println()

	// Apply preset to agent configuration
	agentCfg, ok := cfg.Agents["coder"]
	if !ok {
		return fmt.Errorf("coder agent not found in config")
	}

	// Save original values for comparison
	originalModel := agentCfg.Model
	_ = originalModel // Suppress unused variable warning

	// Apply preset
	p.ApplyToAgent(&agentCfg)
	cfg.Agents["coder"] = agentCfg

	// Apply to options
	if cfg.Options == nil {
		cfg.Options = &config.Options{}
	}
	p.ApplyToOptions(cfg.Options)

	// Update the model configuration if needed
	if model, ok := cfg.Models[p.Model]; ok {
		p.ApplyToSelectedModel(&model)
		cfg.Models[p.Model] = model
	}

	// Save changes to config file
	if err := savePresetToConfig(cfg, p); err != nil {
		return fmt.Errorf("failed to save preset to config: %w", err)
	}

	// Report changes
	fmt.Println("Changes applied:")
	if originalModel != agentCfg.Model {
		fmt.Printf("  Model: %s → %s\n", originalModel, agentCfg.Model)
	}
	if len(p.AllowedTools) > 0 {
		fmt.Printf("  Allowed tools: %v\n", p.AllowedTools)
	}
	if len(p.DisabledTools) > 0 {
		fmt.Printf("  Disabled tools: %v\n", p.DisabledTools)
	}
	if len(p.ContextPaths) > 0 {
		fmt.Printf("  Context paths: %v\n", p.ContextPaths)
	}

	fmt.Println()
	fmt.Println("Preset applied successfully!")
	fmt.Printf("Configuration saved to .cliffy/config.json\n")
	fmt.Println()
	fmt.Println("You can now run tasks with the preset settings:")
	fmt.Printf("  cliffy \"your task\"\n")
	fmt.Println()
	fmt.Println("Or use --preset flag to override temporarily:")
	fmt.Printf("  cliffy --preset %s \"your task\"\n", p.ID)

	return nil
}

func savePresetToConfig(cfg *config.Config, p *preset.Preset) error {
	// Update agent configuration
	if err := cfg.SetConfigField("agents.coder.model", p.Model); err != nil {
		return err
	}

	if len(p.AllowedTools) > 0 {
		if err := cfg.SetConfigField("agents.coder.allowed_tools", p.AllowedTools); err != nil {
			return err
		}
	}

	if len(p.ContextPaths) > 0 {
		if err := cfg.SetConfigField("options.context_paths", p.ContextPaths); err != nil {
			return err
		}
	}

	if len(p.DisabledTools) > 0 {
		if err := cfg.SetConfigField("options.disabled_tools", p.DisabledTools); err != nil {
			return err
		}
	}

	// Update model configuration
	modelKey := fmt.Sprintf("models.%s", p.Model)
	if p.MaxTokens > 0 {
		if err := cfg.SetConfigField(modelKey+".max_tokens", p.MaxTokens); err != nil {
			return err
		}
	}

	if p.ReasoningEffort != "" {
		if err := cfg.SetConfigField(modelKey+".reasoning_effort", p.ReasoningEffort); err != nil {
			return err
		}
	}

	if p.Think {
		if err := cfg.SetConfigField(modelKey+".think", p.Think); err != nil {
			return err
		}
	}

	return nil
}
