package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/orchestra"
	"github.com/charmbracelet/crush/internal/spec"
	"github.com/spf13/cobra"
)

func init() {
	// Spec commands
	specCmd.AddCommand(specCreateCmd)
	specCmd.AddCommand(specValidateCmd)
	specCmd.AddCommand(specShowCmd)
	specCmd.AddCommand(specEditCmd)
	specCmd.AddCommand(specListCmd)
	specCmd.AddCommand(specImpactCmd)

	// Blueprint commands
	blueprintCmd.AddCommand(blueprintGenerateCmd)
	blueprintCmd.AddCommand(blueprintShowCmd)

	// Construction commands
	constructionCmd.AddCommand(constructionGenerateCmd)
	constructionCmd.AddCommand(constructionValidateCmd)

	// Phase commands
	phaseCmd.AddCommand(phaseGenerateCmd)
	phaseCmd.AddCommand(phaseShowCmd)
	phaseCmd.AddCommand(phaseStartCmd)

	// Task commands
	taskCmd.AddCommand(taskGenerateCmd)
	taskCmd.AddCommand(taskShowCmd)
	taskCmd.AddCommand(taskStartCmd)
	taskCmd.AddCommand(taskWatchCmd)
	taskCmd.AddCommand(taskCommentCmd)

	rootCmd.AddCommand(
		specCmd,
		blueprintCmd,
		constructionCmd,
		phaseCmd,
		taskCmd,
		statusCmd,
	)
}

// Spec commands
var specCmd = &cobra.Command{
	Use:   "spec",
	Short: "Manage spec sheets",
	Long:  "Create, validate, and manage spec sheets that drive the orchestration system.",
}

var specCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new spec",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := ResolveCwd(cmd)
		if err != nil {
			return err
		}

		name := args[0]
		specsDir := filepath.Join(cwd, "specs")
		manager := spec.NewManager(specsDir)

		s, err := manager.Create(name, "TODO: Add description")
		if err != nil {
			return fmt.Errorf("failed to create spec: %w", err)
		}

		fmt.Printf("Created spec: %s\n", s.ID)
		fmt.Printf("Location: %s/%s.yaml\n", specsDir, s.ID)
		fmt.Println("\nEdit the spec to add your requirements, entities, and API endpoints.")
		return nil
	},
}

var specValidateCmd = &cobra.Command{
	Use:   "validate <spec>",
	Short: "Validate a spec file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := ResolveCwd(cmd)
		if err != nil {
			return err
		}

		specFile := args[0]
		if !strings.HasSuffix(specFile, ".yaml") {
			specFile += ".yaml"
		}

		path := filepath.Join(cwd, "specs", specFile)
		parser := spec.NewParser()
		s, err := parser.ParseFile(path)
		if err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}

		fmt.Printf("✓ Spec %s is valid\n", s.ID)
		fmt.Printf("  Name: %s\n", s.Name)
		fmt.Printf("  Version: %s\n", s.Version)
		fmt.Printf("  Status: %s\n", s.Status)
		fmt.Printf("  Entities: %d\n", len(s.Entities))
		fmt.Printf("  API Endpoints: %d\n", len(s.APIEndpoints))
		fmt.Printf("  Requirements: %d functional, %d non-functional\n",
			len(s.Requirements.Functional), len(s.Requirements.NonFunctional))
		return nil
	},
}

var specShowCmd = &cobra.Command{
	Use:   "show <spec>",
	Short: "Show spec details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := ResolveCwd(cmd)
		if err != nil {
			return err
		}

		specName := args[0]
		specsDir := filepath.Join(cwd, "specs")
		manager := spec.NewManager(specsDir)

		s, err := manager.Load(specName)
		if err != nil {
			return fmt.Errorf("failed to load spec: %w", err)
		}

		fmt.Printf("Spec: %s\n", s.ID)
		fmt.Printf("Name: %s\n", s.Name)
		fmt.Printf("Version: %s\n", s.Version)
		fmt.Printf("Status: %s\n", s.Status)
		fmt.Printf("Owner: %s\n", s.Owner)
		fmt.Printf("\nDescription:\n%s\n", s.Description)

		if len(s.Goals) > 0 {
			fmt.Println("\nGoals:")
			for _, g := range s.Goals {
				fmt.Printf("  - %s\n", g)
			}
		}

		if len(s.Entities) > 0 {
			fmt.Println("\nEntities:")
			for _, e := range s.Entities {
				fmt.Printf("  - %s (%d fields)\n", e.Name, len(e.Fields))
			}
		}

		if len(s.APIEndpoints) > 0 {
			fmt.Println("\nAPI Endpoints:")
			for _, e := range s.APIEndpoints {
				fmt.Printf("  - %s %s\n", e.Method, e.Path)
			}
		}

		return nil
	},
}

var specEditCmd = &cobra.Command{
	Use:   "edit <spec>",
	Short: "Edit a spec file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := ResolveCwd(cmd)
		if err != nil {
			return err
		}

		specName := args[0]
		specPath := filepath.Join(cwd, "specs", specName+".yaml")

		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vim"
		}

		// This would use exec to open the editor
		fmt.Printf("Opening %s with %s\n", specPath, editor)
		fmt.Println("(Editor integration not implemented in this demo)")
		return nil
	},
}

var specListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all specs",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := ResolveCwd(cmd)
		if err != nil {
			return err
		}

		specsDir := filepath.Join(cwd, "specs")
		manager := spec.NewManager(specsDir)

		specs, err := manager.LoadAll()
		if err != nil {
			return fmt.Errorf("failed to load specs: %w", err)
		}

		if len(specs) == 0 {
			fmt.Println("No specs found. Create one with: crush spec create <name>")
			return nil
		}

		fmt.Println("Specs:")
		for _, s := range specs {
			fmt.Printf("  %s - %s (%s)\n", s.ID, s.Name, s.Status)
		}

		return nil
	},
}

var specImpactCmd = &cobra.Command{
	Use:   "impact <spec>",
	Short: "Analyze impact of spec changes",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Impact analysis requires comparing spec versions.")
		fmt.Println("Use: crush spec diff <spec> <version1> <version2>")
		return nil
	},
}

// Blueprint commands
var blueprintCmd = &cobra.Command{
	Use:   "blueprint",
	Short: "Manage blueprints",
	Long:  "Generate and manage blueprints from spec sheets.",
}

var blueprintGenerateCmd = &cobra.Command{
	Use:   "generate <spec>",
	Short: "Generate blueprints from a spec",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := ResolveCwd(cmd)
		if err != nil {
			return err
		}

		specName := args[0]
		specsDir := filepath.Join(cwd, "specs")
		blueprintsDir := filepath.Join(cwd, "blueprints")

		manager := spec.NewManager(specsDir)
		s, err := manager.Load(specName)
		if err != nil {
			return fmt.Errorf("failed to load spec: %w", err)
		}

		generator := orchestra.NewBlueprintGenerator(blueprintsDir)
		if err := generator.Generate(*s); err != nil {
			return fmt.Errorf("failed to generate blueprints: %w", err)
		}

		fmt.Printf("Generated blueprints for %s:\n", s.ID)
		fmt.Printf("  - %s/README.md\n", s.ID)
		fmt.Printf("  - %s/architecture.md\n", s.ID)
		fmt.Printf("  - %s/domain-model.md\n", s.ID)
		fmt.Printf("  - %s/api-contracts.md\n", s.ID)
		fmt.Printf("  - %s/user-stories.md\n", s.ID)

		return nil
	},
}

var blueprintShowCmd = &cobra.Command{
	Use:   "show <spec> <type>",
	Short: "Show a specific blueprint",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		specName := args[0]
		blueprintType := args[1]

		cwd, err := ResolveCwd(cmd)
		if err != nil {
			return err
		}

		blueprintPath := filepath.Join(cwd, "blueprints", specName, blueprintType+".md")
		data, err := os.ReadFile(blueprintPath)
		if err != nil {
			return fmt.Errorf("failed to read blueprint: %w", err)
		}

		fmt.Println(string(data))
		return nil
	},
}

// Construction commands
var constructionCmd = &cobra.Command{
	Use:   "construction",
	Short: "Manage construction documents",
	Long:  "Generate and manage construction documents (Prisma, OpenAPI, TypeScript) from specs.",
}

var constructionGenerateCmd = &cobra.Command{
	Use:   "generate <spec>",
	Short: "Generate construction documents from a spec",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := ResolveCwd(cmd)
		if err != nil {
			return err
		}

		specName := args[0]
		specsDir := filepath.Join(cwd, "specs")
		constructionDir := filepath.Join(cwd, "construction")

		manager := spec.NewManager(specsDir)
		s, err := manager.Load(specName)
		if err != nil {
			return fmt.Errorf("failed to load spec: %w", err)
		}

		generator := orchestra.NewConstructionGenerator(constructionDir)
		if err := generator.Generate(*s); err != nil {
			return fmt.Errorf("failed to generate construction docs: %w", err)
		}

		fmt.Printf("Generated construction documents for %s:\n", s.ID)
		fmt.Printf("  - prisma/schema.prisma\n")
		fmt.Printf("  - api/openapi.yaml\n")
		fmt.Printf("  - contracts/types.ts\n")

		return nil
	},
}

var constructionValidateCmd = &cobra.Command{
	Use:   "validate <spec>",
	Short: "Validate construction documents against spec",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Construction validation not yet implemented")
		return nil
	},
}

// Phase commands
var phaseCmd = &cobra.Command{
	Use:   "phase",
	Short: "Manage phases",
	Long:  "Generate and manage development phases from specs.",
}

var phaseGenerateCmd = &cobra.Command{
	Use:   "generate <spec>",
	Short: "Generate phases from a spec",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Phase generation from specs requires AI analysis.")
		fmt.Println("This will be implemented with the full orchestration system.")
		return nil
	},
}

var phaseShowCmd = &cobra.Command{
	Use:   "show <project>",
	Short: "Show phases for a project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := ResolveCwd(cmd)
		if err != nil {
			return err
		}

		project := args[0]
		phasesDir := filepath.Join(cwd, ".orchestra", "phases")
		manager := orchestra.NewPhaseManager(phasesDir, filepath.Join(cwd, ".orchestra", "tasks"))

		phases, err := manager.LoadPhasesForProject(project)
		if err != nil {
			return fmt.Errorf("failed to load phases: %w", err)
		}

		if len(phases) == 0 {
			fmt.Printf("No phases found for project %s\n", project)
			return nil
		}

		fmt.Printf("Phases for %s:\n\n", project)
		for _, p := range phases {
			completed, total, _ := manager.GetPhaseProgress(p.ID)
			progress := 0
			if total > 0 {
				progress = (completed * 100) / total
			}

			fmt.Printf("  %s: %s [%d%% - %d/%d tasks]\n", p.ID, p.Name, progress, completed, total)
			fmt.Printf("    Status: %s\n", p.Status)
			if len(p.Dependencies) > 0 {
				fmt.Printf("    Dependencies: %s\n", strings.Join(p.Dependencies, ", "))
			}
		}

		return nil
	},
}

var phaseStartCmd = &cobra.Command{
	Use:   "start <phase-id>",
	Short: "Start a phase",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := ResolveCwd(cmd)
		if err != nil {
			return err
		}

		phaseID := args[0]
		phasesDir := filepath.Join(cwd, ".orchestra", "phases")
		manager := orchestra.NewPhaseManager(phasesDir, filepath.Join(cwd, ".orchestra", "tasks"))

		phase, err := manager.LoadPhase(phaseID)
		if err != nil {
			return fmt.Errorf("failed to load phase: %w", err)
		}

		phase.Status = orchestra.PhaseStatusInProgress
		if err := manager.SavePhase(phase); err != nil {
			return fmt.Errorf("failed to update phase: %w", err)
		}

		fmt.Printf("Started phase: %s\n", phase.Name)
		fmt.Printf("Deliverables:\n")
		for _, d := range phase.Deliverables {
			fmt.Printf("  - %s\n", d)
		}

		return nil
	},
}

// Task commands
var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Manage tasks",
	Long:  "Generate, start, and manage tasks for agents.",
}

var taskGenerateCmd = &cobra.Command{
	Use:   "generate <phase>",
	Short: "Generate tasks from a phase",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Task generation requires AI analysis of requirements.")
		fmt.Println("This will be implemented with the full orchestration system.")
		return nil
	},
}

var taskShowCmd = &cobra.Command{
	Use:   "show <task-id>",
	Short: "Show task details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := ResolveCwd(cmd)
		if err != nil {
			return err
		}

		taskID := args[0]
		tasksDir := filepath.Join(cwd, ".orchestra", "tasks")
		manager := orchestra.NewPhaseManager("", tasksDir)

		task, err := manager.LoadTask(taskID)
		if err != nil {
			return fmt.Errorf("failed to load task: %w", err)
		}

		fmt.Printf("Task: %s\n", task.ID)
		fmt.Printf("Title: %s\n", task.Title)
		fmt.Printf("Phase: %s\n", task.Phase)
		fmt.Printf("Status: %s\n", task.Status)
		fmt.Printf("Requirement: %s\n", task.Requirement)

		if len(task.AcceptanceCriteria) > 0 {
			fmt.Println("\nAcceptance Criteria:")
			for i, ac := range task.AcceptanceCriteria {
				fmt.Printf("  %d. %s\n", i+1, ac)
			}
		}

		if task.AssignedAgent != "" {
			fmt.Printf("\nAssigned Agent: %s\n", task.AssignedAgent)
			fmt.Printf("Branch: %s\n", task.Branch)
			fmt.Printf("Worktree: %s\n", task.Worktree)
		}

		return nil
	},
}

var taskStartCmd = &cobra.Command{
	Use:   "start <task-id>",
	Short: "Start a task (creates worktree, spawns agent)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := ResolveCwd(cmd)
		if err != nil {
			return err
		}

		taskID := args[0]
		tasksDir := filepath.Join(cwd, ".orchestra", "tasks")
		worktreesDir := filepath.Join(cwd, "worktrees")

		manager := orchestra.NewPhaseManager("", tasksDir)
		task, err := manager.LoadTask(taskID)
		if err != nil {
			return fmt.Errorf("failed to load task: %w", err)
		}

		// Create worktree
		wtManager := orchestra.NewWorktreeManager(cwd, worktreesDir)
		worktree, err := wtManager.Create(taskID, "coder")
		if err != nil {
			return fmt.Errorf("failed to create worktree: %w", err)
		}

		// Update task
		task.Status = orchestra.TaskStatusInProgress
		task.AssignedAgent = "coder"
		task.Branch = worktree.Branch
		task.Worktree = worktree.Path
		now := time.Now()
		task.Started = &now

		if err := manager.SaveTask(task); err != nil {
			return fmt.Errorf("failed to update task: %w", err)
		}

		fmt.Printf("Started task: %s\n", taskID)
		fmt.Printf("  Branch: %s\n", worktree.Branch)
		fmt.Printf("  Worktree: %s\n", worktree.Path)
		fmt.Printf("  Agent: coder\n")

		return nil
	},
}

var taskWatchCmd = &cobra.Command{
	Use:   "watch <task-id>",
	Short: "Watch task progress",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Task watching requires the TUI dashboard.")
		fmt.Println("Use: crush dashboard")
		return nil
	},
}

var taskCommentCmd = &cobra.Command{
	Use:   "comment <task-id> <message>",
	Short: "Add a comment to a task",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := ResolveCwd(cmd)
		if err != nil {
			return err
		}

		taskID := args[0]
		message := args[1]
		tasksDir := filepath.Join(cwd, ".orchestra", "tasks")

		manager := orchestra.NewPhaseManager("", tasksDir)
		if err := manager.AddTaskMessage(taskID, "user", message); err != nil {
			return fmt.Errorf("failed to add comment: %w", err)
		}

		fmt.Printf("Added comment to task %s\n", taskID)
		return nil
	},
}

// Status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show orchestration status",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := ResolveCwd(cmd)
		if err != nil {
			return err
		}

		phasesDir := filepath.Join(cwd, ".orchestra", "phases")
		tasksDir := filepath.Join(cwd, ".orchestra", "tasks")
		worktreesDir := filepath.Join(cwd, "worktrees")

		// Check for orchestra directory
		if _, err := os.Stat(phasesDir); os.IsNotExist(err) {
			fmt.Println("No orchestra found in current directory.")
			fmt.Println("Get started by creating a spec: crush spec create <name>")
			return nil
		}

		phaseManager := orchestra.NewPhaseManager(phasesDir, tasksDir)
		phases, err := phaseManager.LoadPhasesForProject("")
		if err != nil {
			return fmt.Errorf("failed to load phases: %w", err)
		}

		// List worktrees
		wtManager := orchestra.NewWorktreeManager(cwd, worktreesDir)
		worktrees, _ := wtManager.List()

		fmt.Println("Orchestration Status")
		fmt.Println("====================")
		fmt.Println()

		// Show phases
		if len(phases) > 0 {
			fmt.Println("Phases:")
			for _, p := range phases {
				completed, total, _ := phaseManager.GetPhaseProgress(p.ID)
				progress := 0
				if total > 0 {
					progress = (completed * 100) / total
				}

				statusIcon := "○"
				switch p.Status {
				case orchestra.PhaseStatusCompleted:
					statusIcon = "✓"
				case orchestra.PhaseStatusInProgress:
					statusIcon = "⚡"
				case orchestra.PhaseStatusBlocked:
					statusIcon = "✗"
				}

				fmt.Printf("  %s %s: %s [%d%%]\n", statusIcon, p.ID, p.Name, progress)
			}
		}

		// Show active worktrees
		if len(worktrees) > 0 {
			fmt.Println("\nActive Worktrees:")
			for _, wt := range worktrees {
				if wt.Path != cwd {
					fmt.Printf("  - %s (%s)\n", wt.Branch, wt.Path)
				}
			}
		}

		return nil
	},
}
