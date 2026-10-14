package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/charmbracelet/crush/internal/client"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/server"
	"github.com/spf13/cobra"
)

const (
	// psSessionsPerWorkspace caps the sessions listed per project.
	psSessionsPerWorkspace = 9
	// psServerTimeout bounds the whole query of one server so a hung server
	// cannot hang ps.
	psServerTimeout = 3 * time.Second
	psNoServerMsg   = "No agent-cli server running (open projects with agent-cli-tmux)"
)

// psSource is the part of [client.Client] that ps needs.
type psSource interface {
	ListWorkspaces(ctx context.Context) ([]proto.Workspace, error)
	ListSessions(ctx context.Context, id string) ([]proto.Session, error)
	GetAgentSessionInfo(ctx context.Context, id, sessionID string) (*proto.AgentSession, error)
}

var _ psSource = (*client.Client)(nil)

type psRow struct {
	Path      string `json:"path"`
	SessionID string `json:"session_id"`
	Title     string `json:"title"`
	Busy      bool   `json:"busy"`
	// Waiting is true when the session awaits a permission or question
	// answer; it takes precedence over Busy in the STATUS column.
	Waiting bool `json:"waiting"`
}

func (r psRow) status() string {
	switch {
	case r.Waiting:
		return "WAITING"
	case r.Busy:
		return "BUSY"
	default:
		return "IDLE"
	}
}

var psCmd = &cobra.Command{
	Use:   "ps",
	Short: "List open projects and their sessions with BUSY/WAITING/IDLE status",
	Long: `List every project and session on the running agent-cli servers.
Without --host, scans all per-project server sockets. Never starts a server.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		asJSON, _ := cmd.Flags().GetBool("json")
		hosts := []string{clientHost}
		if !cmd.Flags().Changed("host") {
			hosts = discoverPSHosts()
		}
		perWorkspace := psSessionsPerWorkspace
		if all, _ := cmd.Flags().GetBool("all"); all {
			perWorkspace = math.MaxInt
		}
		return runPS(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), hosts, asJSON, perWorkspace)
	},
}

func init() {
	psCmd.Flags().Bool("all", false, "List every session instead of the newest 9 per project")
	psCmd.Flags().Bool("json", false, "Output as JSON (prints [] when no server is running)")
	rootCmd.AddCommand(psCmd)
}

// discoverPSHosts finds per-project server sockets in the same directories
// the agent-cli launcher chooses from.
func discoverPSHosts() []string {
	var dirs []string
	rt := os.Getenv("AGENT_CLI_RUNTIME_DIR")
	if rt == "" {
		rt = fmt.Sprintf("/run/user/%d", os.Getuid())
	}
	dirs = append(dirs, rt, os.Getenv("XDG_RUNTIME_DIR"))
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".cache", "agent-cli"))
	}

	var hosts []string
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		socks, _ := filepath.Glob(filepath.Join(dir, "agent-cli-*.sock"))
		for _, s := range socks {
			if h := "unix://" + s; !slices.Contains(hosts, h) {
				hosts = append(hosts, h)
			}
		}
	}
	return hosts
}

// psHostResult is the outcome of querying one server.
type psHostResult struct {
	rows  []psRow
	alive bool // Health succeeded.
	err   error
}

// runPS queries all hosts concurrently and prints the combined result in
// input order. A server counts as running once Health succeeds; failures
// after that only drop the affected rows and are noted on errw. No server is
// ever started.
func runPS(ctx context.Context, w, errw io.Writer, hosts []string, asJSON bool, perWorkspace int) error {
	results := make([]psHostResult, len(hosts))
	var wg sync.WaitGroup
	for i, host := range hosts {
		wg.Go(func() {
			rows, alive, err := queryPSHost(ctx, host, psServerTimeout, perWorkspace)
			results[i] = psHostResult{rows, alive, err}
		})
	}
	wg.Wait()

	rows := []psRow{}
	alive, unresponsive := 0, 0
	for i, r := range results {
		if !r.alive {
			// A dead server (nothing listening) is normal; anything else may
			// be a live server holding a busy session, so say so.
			if r.err != nil && !errors.Is(r.err, syscall.ECONNREFUSED) && !errors.Is(r.err, fs.ErrNotExist) {
				unresponsive++
				fmt.Fprintf(errw, "Skipping %s: not responding (%v)\n", hosts[i], r.err)
			}
			continue
		}
		alive++
		rows = append(rows, r.rows...)
		if r.err != nil {
			fmt.Fprintf(errw, "Skipping %s: %v\n", hosts[i], r.err)
		}
	}
	if alive == 0 && unresponsive == 0 && !asJSON {
		_, err := fmt.Fprintln(w, psNoServerMsg)
		return err
	}
	return writePS(w, rows, asJSON)
}

// queryPSHost returns the rows of one server. alive reports whether Health
// succeeded; rows may be partial when err is set after that.
func queryPSHost(ctx context.Context, host string, timeout time.Duration, perWorkspace int) (rows []psRow, alive bool, err error) {
	u, err := server.ParseHostURL(host)
	if err != nil {
		return nil, false, err
	}
	c, err := client.NewClient("", u.Scheme, u.Host)
	if err != nil {
		return nil, false, err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := c.Health(ctx); err != nil {
		return nil, false, err
	}
	rows, err = collectPS(ctx, c, perWorkspace)
	return rows, true, err
}

// collectPS returns up to perWorkspace sessions for every workspace. The
// server lists sessions newest first. A session whose status cannot be read
// (for example deleted meanwhile) and a workspace whose sessions cannot be
// listed are skipped and reported in the returned error, together with the
// rows that were collected. Callers must surface that error: a skipped
// session may be a busy one.
func collectPS(ctx context.Context, src psSource, perWorkspace int) ([]psRow, error) {
	workspaces, err := src.ListWorkspaces(ctx)
	if err != nil {
		return nil, err
	}
	rows := []psRow{}
	var errs, skips []error
scan:
	for _, ws := range workspaces {
		sessions, err := src.ListSessions(ctx, ws.ID)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, s := range sessions[:min(len(sessions), perWorkspace)] {
			info, err := src.GetAgentSessionInfo(ctx, ws.ID, s.ID)
			if err != nil {
				skips = append(skips, fmt.Errorf("session %s: %w", s.ID, err))
				if ctx.Err() != nil {
					break scan
				}
				continue
			}
			rows = append(rows, psRow{
				Path:      ws.Path,
				SessionID: s.ID,
				Title:     s.Title,
				Busy:      info.IsBusy,
				Waiting:   info.PendingPrompt,
			})
		}
	}
	if len(skips) > 0 {
		errs = append(errs, fmt.Errorf("%d sessions skipped (%w)", len(skips), skips[0]))
	}
	return rows, errors.Join(errs...)
}

// psTitleReplacer keeps a title on one table line.
var psTitleReplacer = strings.NewReplacer("\t", " ", "\n", " ", "\r", " ")

func writePS(w io.Writer, rows []psRow, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "PROJECT\tSESSION\tSTATUS\tTITLE")
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", r.Path, r.SessionID, r.status(), psTitleReplacer.Replace(r.Title))
	}
	return tw.Flush()
}
