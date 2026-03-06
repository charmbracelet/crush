package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/spf13/cobra"
)

var sessionCmd = &cobra.Command{
	Use:     "session",
	Aliases: []string{"sessions"},
	Short:   "Manage sessions",
	Long:    "Manage Crush sessions including listing, viewing, and deleting sessions.",
}

var sessionListJSON bool

var sessionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all sessions",
	Long:  "List all sessions with their hash, title, and creation date.",
	RunE:  runSessionList,
}

func init() {
	sessionListCmd.Flags().BoolVar(&sessionListJSON, "json", false, "Output in JSON format")
	sessionCmd.AddCommand(sessionListCmd)
}

func runSessionList(cmd *cobra.Command, _ []string) error {
	dataDir, _ := cmd.Flags().GetString("data-dir")
	ctx := cmd.Context()

	if dataDir == "" {
		cfg, err := config.Init("", "", false)
		if err != nil {
			return fmt.Errorf("failed to initialize config: %w", err)
		}
		dataDir = cfg.Options.DataDirectory
	}

	conn, err := db.Connect(ctx, dataDir)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer conn.Close()

	queries := db.New(conn)
	sessions := session.NewService(queries, conn)

	list, err := sessions.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	if sessionListJSON {
		output := make([]sessionJSON, len(list))
		for i, s := range list {
			output[i] = sessionJSON{
				ID:       session.HashID(s.ID),
				UUID:     s.ID,
				Title:    s.Title,
				Created:  time.Unix(s.CreatedAt, 0).Format(time.RFC3339),
				Modified: time.Unix(s.UpdatedAt, 0).Format(time.RFC3339),
			}
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetEscapeHTML(false)
		return enc.Encode(output)
	}

	for _, s := range list {
		hash := session.HashID(s.ID)
		createdAt := time.Unix(s.CreatedAt, 0)
		relative := relativeTime(createdAt)
		fmt.Printf("%s %q %s (%s)\n", hash[:7], s.Title, createdAt.Format(time.RFC3339), relative)
	}

	return nil
}

type sessionJSON struct {
	ID       string `json:"id"`
	UUID     string `json:"uuid"`
	Title    string `json:"title"`
	Created  string `json:"created"`
	Modified string `json:"modified"`
}

func relativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case d < 24*time.Hour:
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case d < 48*time.Hour:
		return "yesterday"
	case d < 7*24*time.Hour:
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%d days ago", days)
	case d < 30*24*time.Hour:
		weeks := int(d.Hours() / 24 / 7)
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	case d < 365*24*time.Hour:
		months := int(d.Hours() / 24 / 30)
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	default:
		years := int(d.Hours() / 24 / 365)
		if years == 1 {
			return "1 year ago"
		}
		return fmt.Sprintf("%d years ago", years)
	}
}
