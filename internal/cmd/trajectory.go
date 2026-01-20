package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/trajectory"
	"github.com/charmbracelet/crush/internal/version"
	"github.com/spf13/cobra"
)

var trajectoryCmd = &cobra.Command{
	Use:   "trajectory",
	Short: "Trajectory export utilities",
	Long:  "Export session trajectories in Harbor ATIF format for analysis and sharing",
}

var trajectoryExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export a session as ATIF trajectory",
	Long:  "Export a Crush session in Harbor ATIF (Agent Trajectory Interchange Format) v1.4",
	Example: `
# Export a session as JSON to stdout
crush trajectory export --session <session-id>

# Export a session to a JSON file
crush trajectory export --session <session-id> --output trajectory.json

# Export as HTML for visualization
crush trajectory export --session <session-id> --format html --output trajectory.html

# Validate with Harbor validator
crush trajectory export --session <session-id> > out.json
python -m harbor.utils.trajectory_validator out.json
  `,
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionID, _ := cmd.Flags().GetString("session")
		outputFile, _ := cmd.Flags().GetString("output")
		format, _ := cmd.Flags().GetString("format")
		dataDir, _ := cmd.Flags().GetString("data-dir")

		if sessionID == "" {
			return fmt.Errorf("--session flag is required")
		}

		ctx := cmd.Context()

		cwd, err := ResolveCwd(cmd)
		if err != nil {
			return err
		}

		// Load config (lightweight, no full app init).
		cfg, err := config.Load(cwd, dataDir, false)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Connect to DB.
		conn, err := db.Connect(ctx, cfg.Options.DataDirectory)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer conn.Close()

		querier := db.New(conn)
		sessionSvc := session.NewService(querier)
		messageSvc := message.NewService(querier)

		// Load session.
		sess, err := sessionSvc.Get(ctx, sessionID)
		if err != nil {
			return fmt.Errorf("failed to get session: %w", err)
		}

		// Load messages.
		messages, err := messageSvc.List(ctx, sessionID)
		if err != nil {
			return fmt.Errorf("failed to list messages: %w", err)
		}

		// Determine model name from first assistant message.
		var modelName string
		for _, msg := range messages {
			if msg.Role == message.Assistant && msg.Model != "" {
				modelName = msg.Model
				break
			}
		}

		// Export to ATIF.
		traj, err := trajectory.ExportSession(sess, messages, "Crush", version.Version, modelName)
		if err != nil {
			return fmt.Errorf("failed to export trajectory: %w", err)
		}

		var data []byte
		switch format {
		case "html":
			data, err = trajectory.RenderHTML(traj)
			if err != nil {
				return fmt.Errorf("failed to render HTML: %w", err)
			}
		case "json":
			data, err = json.MarshalIndent(traj, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal trajectory: %w", err)
			}
		default:
			return fmt.Errorf("unknown format: %s (use 'json' or 'html')", format)
		}

		// Write output.
		if outputFile != "" {
			if err := os.WriteFile(outputFile, data, 0o644); err != nil {
				return fmt.Errorf("failed to write output file: %w", err)
			}
			cmd.Printf("Exported trajectory to %s\n", outputFile)
		} else {
			cmd.Println(string(data))
		}

		return nil
	},
}

func init() {
	trajectoryExportCmd.Flags().StringP("session", "s", "", "Session ID to export (required)")
	trajectoryExportCmd.Flags().StringP("output", "o", "", "Output file path (defaults to stdout)")
	trajectoryExportCmd.Flags().StringP("format", "f", "json", "Output format: json or html")
	_ = trajectoryExportCmd.MarkFlagRequired("session")

	trajectoryCmd.AddCommand(trajectoryExportCmd)
}
