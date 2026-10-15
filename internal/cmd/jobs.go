package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/charmbracelet/crush/internal/client"
	"github.com/charmbracelet/crush/internal/shell"
	"github.com/spf13/cobra"
)

// jobsClient là phần client mà lệnh jobs cần.
type jobsClient interface {
	ListJobs(ctx context.Context, id string) ([]shell.JobInfo, error)
	JobOutput(ctx context.Context, id, jobID string) (string, error)
	KillJob(ctx context.Context, id, jobID string) error
}

var _ jobsClient = (*client.Client)(nil)

var jobsCmd = &cobra.Command{
	Use:   "jobs",
	Short: "List the bash jobs the agent ran in this project (running and recently finished)",
	Long: `List the bash commands the agent ran in this project's server: running first,
then recently finished, with the owning session, elapsed time and result.
Needs the project server (run through the agent-cli launcher).`,
	Example: `
# Jobs of the project in the current directory
agent-cli jobs

# One JSON array on stdout
agent-cli jobs --json

# Output of one job (head and tail when huge), and stopping one
agent-cli jobs output 01A
agent-cli jobs kill 01A
`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		asJSON, _ := cmd.Flags().GetBool("json")
		return withJobsClient(cmd, func(ctx context.Context, c jobsClient, wsID string) error {
			jobs, err := c.ListJobs(ctx, wsID)
			if err != nil {
				return err
			}
			return printJobs(os.Stdout, jobs, asJSON, time.Now())
		})
	},
}

var jobsOutputCmd = &cobra.Command{
	Use:   "output <job-id>",
	Short: "Print the output of one job",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withJobsClient(cmd, func(ctx context.Context, c jobsClient, wsID string) error {
			out, err := c.JobOutput(ctx, wsID, args[0])
			if err != nil {
				return err
			}
			_, err = io.WriteString(os.Stdout, strings.TrimSuffix(out, "\n")+"\n")
			return err
		})
	},
}

var jobsKillCmd = &cobra.Command{
	Use:   "kill <job-id>",
	Short: "Stop a running job",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return withJobsClient(cmd, func(ctx context.Context, c jobsClient, wsID string) error {
			return c.KillJob(ctx, wsID, args[0])
		})
	},
}

func init() {
	jobsCmd.Flags().Bool("json", false, "Print one JSON array on stdout")
	jobsCmd.AddCommand(jobsOutputCmd, jobsKillCmd)
	rootCmd.AddCommand(jobsCmd)
}

// withJobsClient nối tới server của dự án (như send) rồi chạy fn. Job nằm
// trong tiến trình server; chế độ in-process thì mỗi lệnh là một tiến trình
// riêng, không thấy job của TUI.
func withJobsClient(cmd *cobra.Command, fn func(ctx context.Context, c jobsClient, wsID string) error) error {
	if !useClientServer() {
		return &usageError{errors.New("jobs needs the project server: run it through the agent-cli launcher or set CRUSH_CLIENT_SERVER=1")}
	}
	c, ws, cleanup, err := connectToServer(cmd)
	if err != nil {
		return err
	}
	defer cleanup()
	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()
	return fn(ctx, c, ws.ID)
}

// printJobs in danh sách job: bảng cho người đọc, hoặc một mảng JSON.
func printJobs(w io.Writer, jobs []shell.JobInfo, asJSON bool, now time.Time) error {
	if asJSON {
		if jobs == nil {
			jobs = []shell.JobInfo{}
		}
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		return enc.Encode(jobs)
	}
	if len(jobs) == 0 {
		_, err := fmt.Fprintln(w, "No jobs in this project yet.")
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tSTATUS\tELAPSED\tSESSION\tBG\tCOMMAND")
	for _, j := range jobs {
		status := strings.ToUpper(string(j.Status))
		if j.Status == shell.JobFailed || j.Status == shell.JobKilled {
			status += fmt.Sprintf(" (%d)", j.ExitCode)
		}
		bg := ""
		if j.Background {
			bg = "bg"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			j.ID, status, j.Elapsed(now).Round(time.Second), shortID(j.SessionID), bg, oneLine(cmpOrDesc(j), 70))
	}
	return tw.Flush()
}

// cmpOrDesc trả mô tả của job, không có thì lệnh.
func cmpOrDesc(j shell.JobInfo) string {
	if strings.TrimSpace(j.Description) != "" {
		return j.Description
	}
	return j.Command
}

// oneLine gộp về một dòng và cắt ở max rune.
func oneLine(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > max {
		return string(r[:max-1]) + "…"
	}
	return s
}

// shortID trả 8 ký tự đầu của ID session (rỗng thì "-").
func shortID(id string) string {
	switch {
	case id == "":
		return "-"
	case len(id) > 8:
		return id[:8]
	}
	return id
}
