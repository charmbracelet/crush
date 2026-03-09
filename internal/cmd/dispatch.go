package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/dispatch"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
)

func init() {
	// Agent subcommands
	agentsCmd.AddCommand(agentsListCmd)
	agentsCmd.AddCommand(agentsShowCmd)
	agentsCmd.AddCommand(agentsCreateCmd)
	agentsCmd.AddCommand(agentsUpdateCmd)
	agentsCmd.AddCommand(agentsDeleteCmd)
	agentsCmd.AddCommand(agentsEnableCmd)
	agentsCmd.AddCommand(agentsDisableCmd)

	// Dispatch subcommands
	dispatchCmd.AddCommand(dispatchListCmd)
	dispatchCmd.AddCommand(dispatchShowCmd)
	dispatchCmd.AddCommand(dispatchSendCmd)

	rootCmd.AddCommand(agentsCmd, dispatchCmd)
}

var agentsCmd = &cobra.Command{
	Use:   "agents",
	Short: "Manage registered agents",
	Long:  `Manage the agent registry for multi-agent orchestration.`,
}

var agentsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all registered agents",
	RunE: func(cmd *cobra.Command, args []string) error {
		dispatchSvc, err := setupDispatch(cmd)
		if err != nil {
			return err
		}

		all, _ := cmd.Flags().GetBool("all")
		agents, err := dispatchSvc.ListAgents(cmd.Context(), all)
		if err != nil {
			return fmt.Errorf("failed to list agents: %w", err)
		}

		if len(agents) == 0 {
			fmt.Println("No agents registered.")
			return nil
		}

		if term.IsTerminal(os.Stdout.Fd()) {
			fmt.Println("Registered Agents:")
			fmt.Println("==================")
			for _, a := range agents {
				status := "enabled"
				if !a.Enabled {
					status = "disabled"
				}
				fmt.Printf("\n%s [%s]\n", a.Name, status)
				if a.Description != "" {
					fmt.Printf("  Description: %s\n", a.Description)
				}
				if len(a.Capabilities) > 0 {
					fmt.Printf("  Capabilities: %v\n", a.Capabilities)
				}
				if a.CLICommand != "" {
					fmt.Printf("  CLI Command: %s\n", a.CLICommand)
				}
			}
		} else {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.Encode(agents)
		}
		return nil
	},
}

var agentsShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show details of an agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dispatchSvc, err := setupDispatch(cmd)
		if err != nil {
			return err
		}

		agent, err := dispatchSvc.GetAgent(cmd.Context(), args[0])
		if err != nil {
			return fmt.Errorf("failed to get agent: %w", err)
		}

		if term.IsTerminal(os.Stdout.Fd()) {
			status := "enabled"
			if !agent.Enabled {
				status = "disabled"
			}
			fmt.Printf("Name: %s\n", agent.Name)
			fmt.Printf("Status: %s\n", status)
			if agent.Description != "" {
				fmt.Printf("Description: %s\n", agent.Description)
			}
			if len(agent.Capabilities) > 0 {
				fmt.Printf("Capabilities: %v\n", agent.Capabilities)
			}
			if agent.SystemPrompt != "" {
				fmt.Printf("System Prompt: %s\n", agent.SystemPrompt)
			}
			if agent.CLICommand != "" {
				fmt.Printf("CLI Command: %s\n", agent.CLICommand)
			}
			if len(agent.ModelRequirements) > 0 {
				fmt.Printf("Model Requirements: %v\n", agent.ModelRequirements)
			}
			fmt.Printf("Created: %s\n", time.Unix(agent.CreatedAt, 0).Format(time.RFC3339))
			fmt.Printf("Updated: %s\n", time.Unix(agent.UpdatedAt, 0).Format(time.RFC3339))
		} else {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.Encode(agent)
		}
		return nil
	},
}

var agentsCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Register a new agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dispatchSvc, err := setupDispatch(cmd)
		if err != nil {
			return err
		}

		description, _ := cmd.Flags().GetString("description")
		capabilities, _ := cmd.Flags().GetStringSlice("capabilities")
		cliCommand, _ := cmd.Flags().GetString("cli-command")

		agent, err := dispatchSvc.CreateAgent(cmd.Context(), dispatch.CreateAgentParams{
			Name:         args[0],
			Description:  description,
			Capabilities: capabilities,
			CLICommand:   cliCommand,
		})
		if err != nil {
			return fmt.Errorf("failed to create agent: %w", err)
		}

		fmt.Printf("Agent '%s' created successfully.\n", agent.Name)
		return nil
	},
}

var agentsUpdateCmd = &cobra.Command{
	Use:   "update <name>",
	Short: "Update an existing agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dispatchSvc, err := setupDispatch(cmd)
		if err != nil {
			return err
		}

		description, _ := cmd.Flags().GetString("description")
		capabilities, _ := cmd.Flags().GetStringSlice("capabilities")
		cliCommand, _ := cmd.Flags().GetString("cli-command")

		agent, err := dispatchSvc.UpdateAgent(cmd.Context(), args[0], dispatch.CreateAgentParams{
			Description:  description,
			Capabilities: capabilities,
			CLICommand:   cliCommand,
		})
		if err != nil {
			return fmt.Errorf("failed to update agent: %w", err)
		}

		fmt.Printf("Agent '%s' updated successfully.\n", agent.Name)
		return nil
	},
}

var agentsDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete an agent from the registry",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dispatchSvc, err := setupDispatch(cmd)
		if err != nil {
			return err
		}

		if err := dispatchSvc.DeleteAgent(cmd.Context(), args[0]); err != nil {
			return fmt.Errorf("failed to delete agent: %w", err)
		}

		fmt.Printf("Agent '%s' deleted successfully.\n", args[0])
		return nil
	},
}

var agentsEnableCmd = &cobra.Command{
	Use:   "enable <name>",
	Short: "Enable an agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dispatchSvc, err := setupDispatch(cmd)
		if err != nil {
			return err
		}

		if err := dispatchSvc.SetAgentEnabled(cmd.Context(), args[0], true); err != nil {
			return fmt.Errorf("failed to enable agent: %w", err)
		}

		fmt.Printf("Agent '%s' enabled.\n", args[0])
		return nil
	},
}

var agentsDisableCmd = &cobra.Command{
	Use:   "disable <name>",
	Short: "Disable an agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dispatchSvc, err := setupDispatch(cmd)
		if err != nil {
			return err
		}

		if err := dispatchSvc.SetAgentEnabled(cmd.Context(), args[0], false); err != nil {
			return fmt.Errorf("failed to disable agent: %w", err)
		}

		fmt.Printf("Agent '%s' disabled.\n", args[0])
		return nil
	},
}

var dispatchCmd = &cobra.Command{
	Use:   "dispatch",
	Short: "Manage dispatch messages",
	Long:  `View and manage dispatch messages for inter-agent communication.`,
}

var dispatchListCmd = &cobra.Command{
	Use:   "list",
	Short: "List dispatch messages",
	RunE: func(cmd *cobra.Command, args []string) error {
		dispatchSvc, err := setupDispatch(cmd)
		if err != nil {
			return err
		}

		fromAgent, _ := cmd.Flags().GetString("from")
		toAgent, _ := cmd.Flags().GetString("to")
		status, _ := cmd.Flags().GetString("status")

		messages, err := dispatchSvc.List(cmd.Context(), dispatch.ListMessagesParams{
			FromAgent: fromAgent,
			ToAgent:   toAgent,
			Status:    dispatch.MessageStatus(status),
		})
		if err != nil {
			return fmt.Errorf("failed to list messages: %w", err)
		}

		if len(messages) == 0 {
			fmt.Println("No dispatch messages found.")
			return nil
		}

		if term.IsTerminal(os.Stdout.Fd()) {
			fmt.Println("Dispatch Messages:")
			fmt.Println("==================")
			for _, m := range messages {
				fmt.Printf("\n[%s] %s -> %s\n", m.Status, m.FromAgent, m.ToAgent)
				fmt.Printf("  ID: %s\n", m.ID)
				fmt.Printf("  Task: %s\n", truncate(m.Task, 60))
				fmt.Printf("  Priority: %d\n", m.Priority)
				fmt.Printf("  Created: %s\n", time.Unix(m.CreatedAt, 0).Format(time.RFC3339))
				if m.Result != "" {
					fmt.Printf("  Result: %s\n", truncate(m.Result, 60))
				}
				if m.Error != "" {
					fmt.Printf("  Error: %s\n", m.Error)
				}
			}
		} else {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.Encode(messages)
		}
		return nil
	},
}

var dispatchShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show details of a dispatch message",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dispatchSvc, err := setupDispatch(cmd)
		if err != nil {
			return err
		}

		msg, err := dispatchSvc.Get(cmd.Context(), args[0])
		if err != nil {
			return fmt.Errorf("failed to get message: %w", err)
		}

		if term.IsTerminal(os.Stdout.Fd()) {
			fmt.Printf("ID: %s\n", msg.ID)
			fmt.Printf("Status: %s\n", msg.Status)
			fmt.Printf("From: %s\n", msg.FromAgent)
			fmt.Printf("To: %s\n", msg.ToAgent)
			fmt.Printf("Task: %s\n", msg.Task)
			fmt.Printf("Priority: %d\n", msg.Priority)
			if msg.SessionID != "" {
				fmt.Printf("Session ID: %s\n", msg.SessionID)
			}
			if msg.ParentMessageID != "" {
				fmt.Printf("Parent Message ID: %s\n", msg.ParentMessageID)
			}
			if len(msg.Context) > 0 {
				fmt.Printf("Context: %v\n", msg.Context)
			}
			fmt.Printf("Created: %s\n", time.Unix(msg.CreatedAt, 0).Format(time.RFC3339))
			fmt.Printf("Updated: %s\n", time.Unix(msg.UpdatedAt, 0).Format(time.RFC3339))
			if msg.CompletedAt != nil {
				fmt.Printf("Completed: %s\n", time.Unix(*msg.CompletedAt, 0).Format(time.RFC3339))
			}
			if msg.Result != "" {
				fmt.Printf("Result: %s\n", msg.Result)
			}
			if msg.Error != "" {
				fmt.Printf("Error: %s\n", msg.Error)
			}
		} else {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.Encode(msg)
		}
		return nil
	},
}

var dispatchSendCmd = &cobra.Command{
	Use:   "send <to-agent> <task>",
	Short: "Send a task to an agent",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		dispatchSvc, err := setupDispatch(cmd)
		if err != nil {
			return err
		}

		priority, _ := cmd.Flags().GetInt("priority")
		contextJSON, _ := cmd.Flags().GetString("context")

		var context map[string]any
		if contextJSON != "" {
			if err := json.Unmarshal([]byte(contextJSON), &context); err != nil {
				return fmt.Errorf("invalid context JSON: %w", err)
			}
		}

		msg, err := dispatchSvc.Send(cmd.Context(), dispatch.SendMessageParams{
			FromAgent: "crush-cli",
			ToAgent:   args[0],
			Task:      args[1],
			Context:   context,
			Priority:  priority,
		})
		if err != nil {
			return fmt.Errorf("failed to send message: %w", err)
		}

		fmt.Printf("Task sent to '%s'. Message ID: %s\n", args[0], msg.ID)
		return nil
	},
}

func init() {
	agentsListCmd.Flags().BoolP("all", "a", false, "Show all agents including disabled")
	agentsCreateCmd.Flags().StringP("description", "d", "", "Agent description")
	agentsCreateCmd.Flags().StringSliceP("capabilities", "c", nil, "Agent capabilities")
	agentsCreateCmd.Flags().StringP("cli-command", "x", "", "CLI command to invoke the agent")
	agentsUpdateCmd.Flags().StringP("description", "d", "", "Agent description")
	agentsUpdateCmd.Flags().StringSliceP("capabilities", "c", nil, "Agent capabilities")
	agentsUpdateCmd.Flags().StringP("cli-command", "x", "", "CLI command to invoke the agent")
	dispatchListCmd.Flags().StringP("from", "f", "", "Filter by from agent")
	dispatchListCmd.Flags().StringP("to", "t", "", "Filter by to agent")
	dispatchListCmd.Flags().StringP("status", "s", "", "Filter by status (pending, in_progress, completed, failed)")
	dispatchSendCmd.Flags().IntP("priority", "p", 0, "Task priority")
	dispatchSendCmd.Flags().StringP("context", "c", "", "Context data as JSON")
}

func setupDispatch(cmd *cobra.Command) (dispatch.Service, error) {
	cwd, err := ResolveCwd(cmd)
	if err != nil {
		return nil, err
	}
	dataDir, _ := cmd.Flags().GetString("data-dir")

	cfg, err := config.Load(cwd, dataDir, false)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	conn, err := db.Connect(cmd.Context(), cfg.Options.DataDirectory)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	q := db.New(conn)
	return dispatch.NewService(q, conn), nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
