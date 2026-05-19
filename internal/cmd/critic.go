package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/skills/critic"
	"github.com/spf13/cobra"
)

var criticCmd = &cobra.Command{
	Use:   "critic",
	Short: "Inspect critic reviews",
	Long:  "List and inspect self-critic reviews stored in the database.",
}

var (
	criticListSession string
	criticShowMessage string
)

var criticListCmd = &cobra.Command{
	Use:   "list",
	Short: "List critic reviews for a session",
	Long:  "List all critic reviews for the given session ID.",
	RunE:  runCriticList,
}

var criticShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show a single critic review",
	Long:  "Show the full critic review associated with a message ID.",
	RunE:  runCriticShow,
}

var criticStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show critic aggregate statistics",
	Long:  "Show aggregate statistics across all critic reviews.",
	RunE:  runCriticStats,
}

func init() {
	criticListCmd.Flags().StringVar(&criticListSession, "session", "", "session ID to filter by")
	criticShowCmd.Flags().StringVar(&criticShowMessage, "message", "", "message ID to look up")
	_ = criticShowCmd.MarkFlagRequired("message")

	criticCmd.AddCommand(criticListCmd)
	criticCmd.AddCommand(criticShowCmd)
	criticCmd.AddCommand(criticStatsCmd)
}

func criticSetup(cmd *cobra.Command) (context.Context, *critic.Store, func(), error) {
	dataDir, _ := cmd.Flags().GetString("data-dir")
	ctx := cmd.Context()

	if dataDir == "" {
		cfg, err := config.Init("", "", false)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to initialize config: %w", err)
		}
		dataDir = cfg.Config().Options.DataDirectory
	}

	conn, err := db.Connect(ctx, dataDir)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	cleanup := func() { _ = conn.Close() }

	q := db.New(conn)
	store := critic.NewStore(q)
	store.SetDB(conn)
	return ctx, store, cleanup, nil
}

func runCriticList(cmd *cobra.Command, _ []string) error {
	ctx, store, cleanup, err := criticSetup(cmd)
	if err != nil {
		return err
	}
	defer cleanup()

	if criticListSession == "" {
		return fmt.Errorf("--session is required")
	}

	records, err := store.ListBySession(ctx, criticListSession)
	if err != nil {
		return fmt.Errorf("failed to list critic reviews: %w", err)
	}

	if len(records) == 0 {
		fmt.Println("No critic reviews found for this session.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "VERDICT\tCONFIDENCE\tCONCERNS\tSUMMARY")
	for _, r := range records {
		summary := r.Summary
		if len(summary) > 60 {
			summary = summary[:57] + "..."
		}
		fmt.Fprintf(w, "%s\t%.2f\t%d\t%s\n", r.Verdict, r.Confidence, len(r.Concerns), summary)
	}
	_ = w.Flush()
	return nil
}

func runCriticShow(cmd *cobra.Command, _ []string) error {
	ctx, store, cleanup, err := criticSetup(cmd)
	if err != nil {
		return err
	}
	defer cleanup()

	record, err := store.GetByMessageID(ctx, criticShowMessage)
	if err != nil {
		return fmt.Errorf("failed to get critic review: %w", err)
	}

	fmt.Printf("Verdict:    %s\n", record.Verdict)
	fmt.Printf("Confidence: %.2f\n", record.Confidence)
	fmt.Printf("Summary:    %s\n", record.Summary)
	if len(record.Concerns) > 0 {
		fmt.Println("Concerns:")
		for _, c := range record.Concerns {
			fmt.Printf("  - [%s | %s] %s\n", c.Severity, c.Dimension, c.Summary)
			if c.Suggestion != "" {
				fmt.Printf("    Suggestion: %s\n", c.Suggestion)
			}
		}
	}
	return nil
}

func runCriticStats(cmd *cobra.Command, _ []string) error {
	ctx, store, cleanup, err := criticSetup(cmd)
	if err != nil {
		return err
	}
	defer cleanup()

	conn, ok := store.DB()
	if !ok {
		return fmt.Errorf("database connection not available for stats")
	}

	var total int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM critic_reviews").Scan(&total); err != nil {
		return fmt.Errorf("failed to count critic reviews: %w", err)
	}

	var approveCount int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM critic_reviews WHERE verdict = 'approve'").Scan(&approveCount); err != nil {
		return fmt.Errorf("failed to count approved reviews: %w", err)
	}

	var reviseCount int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM critic_reviews WHERE verdict = 'revise'").Scan(&reviseCount); err != nil {
		return fmt.Errorf("failed to count revised reviews: %w", err)
	}

	var haltCount int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM critic_reviews WHERE verdict = 'halt'").Scan(&haltCount); err != nil {
		return fmt.Errorf("failed to count halted reviews: %w", err)
	}

	fmt.Printf("Total reviews:  %d\n", total)
	fmt.Printf("Approved:       %d (%.1f%%)\n", approveCount, pct(approveCount, total))
	fmt.Printf("Revised:        %d (%.1f%%)\n", reviseCount, pct(reviseCount, total))
	fmt.Printf("Halted:         %d (%.1f%%)\n", haltCount, pct(haltCount, total))
	return nil
}

func pct(part, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total) * 100
}
