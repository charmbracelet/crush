package cmd

import (
	"encoding/json"
	"os"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/charmbracelet/crush/internal/projects"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
)

var projectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "List all tracked project directories",
	Long: `List all directories where Crush has been used.
This includes the working directory, data directory path, and last accessed time.`,
	Example: `
# List all projects in a table
crush projects

# Output as JSON
crush projects --json
  `,
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonOutput, _ := cmd.Flags().GetBool("json")

		projectList, err := projects.List()
		if err != nil {
			return err
		}

		if jsonOutput {
			output := struct {
				Projects []projects.Project `json:"projects"`
			}{Projects: projectList}

			data, err := json.Marshal(output)
			if err != nil {
				return err
			}
			cmd.Println(string(data))
			return nil
		}

		if len(projectList) == 0 {
			cmd.Println("No projects tracked yet.")
			return nil
		}

		if term.IsTerminal(os.Stdout.Fd()) {
			// We're in a TTY: make it fancy.
			t := table.New().
				Border(lipgloss.RoundedBorder()).
				StyleFunc(func(row, col int) lipgloss.Style {
					return lipgloss.NewStyle().Padding(0, 2)
				}).
				Headers("Path", "Data Dir", "Last Accessed")

			for _, p := range projectList {
				t.Row(p.Path, p.DataDir, p.LastAccessed.Local().Format("2006-01-02 15:04"))
			}
			lipgloss.Println(t)
			return nil
		}

		// Not a TTY: plain output
		for _, p := range projectList {
			cmd.Printf("%s\t%s\t%s\n", p.Path, p.DataDir, p.LastAccessed.Format("2006-01-02T15:04:05Z07:00"))
		}
		return nil
	},
}

func init() {
	projectsCmd.Flags().Bool("json", false, "Output as JSON")
}
