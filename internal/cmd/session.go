package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/event"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/charmtone"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
)

var sessionCmd = &cobra.Command{
	Use:     "session",
	Aliases: []string{"sessions", "s"},
	Short:   "Manage sessions",
	Long:    "Manage Crush sessions. Agents can use --json for machine-readable output.",
}

var (
	sessionListJSON    bool
	sessionShowJSON    bool
	sessionShowByModel bool
	sessionLastJSON    bool
	sessionDeleteJSON  bool
	sessionRenameJSON  bool
)

var sessionListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all sessions",
	Long:    "List all sessions. Use --json for machine-readable output.",
	RunE:    runSessionList,
}

var sessionShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show session details",
	Long: "Show session details. Use --json for machine-readable output. ID can be a UUID, full hash, or hash prefix.\n\n" +
		"Use --by-model for a cost breakdown attributed to the model that incurred each charge, " +
		"across this session and every sub-session below it. The session's own `cost` column " +
		"includes cost propagated up from sub-agents, so it alone does not say which model was billed.",
	Args: cobra.ExactArgs(1),
	RunE: runSessionShow,
}

var sessionLastCmd = &cobra.Command{
	Use:   "last",
	Short: "Show most recent session",
	Long:  "Show the last updated session. Use --json for machine-readable output.",
	RunE:  runSessionLast,
}

var sessionDeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"rm"},
	Short:   "Delete a session",
	Long:    "Delete a session by ID. Use --json for machine-readable output. ID can be a UUID, full hash, or hash prefix.",
	Args:    cobra.ExactArgs(1),
	RunE:    runSessionDelete,
}

var sessionRenameCmd = &cobra.Command{
	Use:   "rename <id> <title>",
	Short: "Rename a session",
	Long:  "Rename a session by ID. Use --json for machine-readable output. ID can be a UUID, full hash, or hash prefix.",
	Args:  cobra.MinimumNArgs(2),
	RunE:  runSessionRename,
}

func init() {
	sessionListCmd.Flags().BoolVar(&sessionListJSON, "json", false, "output in JSON format")
	sessionShowCmd.Flags().BoolVar(&sessionShowJSON, "json", false, "output in JSON format")
	sessionShowCmd.Flags().BoolVar(&sessionShowByModel, "by-model", false, "break token usage and cost down by the model that incurred it, across this session and its sub-sessions")
	sessionLastCmd.Flags().BoolVar(&sessionLastJSON, "json", false, "output in JSON format")
	sessionDeleteCmd.Flags().BoolVar(&sessionDeleteJSON, "json", false, "output in JSON format")
	sessionRenameCmd.Flags().BoolVar(&sessionRenameJSON, "json", false, "output in JSON format")
	sessionCmd.AddCommand(sessionListCmd)
	sessionCmd.AddCommand(sessionShowCmd)
	sessionCmd.AddCommand(sessionLastCmd)
	sessionCmd.AddCommand(sessionDeleteCmd)
	sessionCmd.AddCommand(sessionRenameCmd)
}

type sessionServices struct {
	sessions session.Service
	messages message.Service
	cfg      *config.ConfigStore
}

func sessionSetup(cmd *cobra.Command) (context.Context, *sessionServices, func(), error) {
	dataDir, _ := cmd.Flags().GetString("data-dir")
	ctx := cmd.Context()

	cfg, err := config.Init("", dataDir, false)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to initialize config: %w", err)
	}
	if dataDir == "" {
		dataDir = cfg.Config().Options.DataDirectory
	}
	if shouldEnableMetrics(cfg.Config()) {
		event.Init()
	}

	conn, err := db.Connect(ctx, dataDir)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	queries := db.New(conn)
	svc := &sessionServices{
		sessions: session.NewService(queries, conn),
		messages: message.NewService(queries),
		cfg:      cfg,
	}
	return ctx, svc, func() { conn.Close() }, nil
}

func runSessionList(cmd *cobra.Command, _ []string) error {
	event.SetNonInteractive(true)

	ctx, svc, cleanup, err := sessionSetup(cmd)
	if err != nil {
		return err
	}
	defer cleanup()

	event.SessionListed(sessionListJSON)

	list, err := svc.sessions.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	if sessionListJSON {
		out := cmd.OutOrStdout()
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
		enc := json.NewEncoder(out)
		enc.SetEscapeHTML(false)
		return enc.Encode(output)
	}

	w, cleanup, usingPager := sessionWriter(ctx, len(list))
	defer cleanup()

	hashStyle := lipgloss.NewStyle().Foreground(charmtone.Malibu)
	dateStyle := lipgloss.NewStyle().Foreground(charmtone.Damson)

	width := sessionOutputWidth
	if tw, _, err := term.GetSize(os.Stdout.Fd()); err == nil && tw > 0 {
		width = tw
	}
	// 7 (hash) + 1 (space) + 25 (RFC3339 date) + 1 (space) = 34 chars prefix.
	titleWidth := max(width-34, 10)

	var writeErr error
	for _, s := range list {
		hash := session.HashID(s.ID)[:7]
		date := time.Unix(s.CreatedAt, 0).Format(time.RFC3339)
		title := strings.ReplaceAll(s.Title, "\n", " ")
		title = ansi.Truncate(title, titleWidth, "…")
		_, writeErr = fmt.Fprintln(w, hashStyle.Render(hash), dateStyle.Render(date), title)
		if writeErr != nil {
			break
		}
	}
	if writeErr != nil && usingPager && isBrokenPipe(writeErr) {
		return nil
	}
	return writeErr
}

type sessionJSON struct {
	ID       string `json:"id"`
	UUID     string `json:"uuid"`
	Title    string `json:"title"`
	Created  string `json:"created"`
	Modified string `json:"modified"`
}

type sessionMutationResult struct {
	ID      string `json:"id"`
	UUID    string `json:"uuid"`
	Title   string `json:"title"`
	Deleted bool   `json:"deleted,omitempty"`
	Renamed bool   `json:"renamed,omitempty"`
}

// resolveSessionID resolves a session ID that can be a UUID, full hash, or hash prefix.
// Returns an error if the prefix is ambiguous (matches multiple sessions).
func resolveSessionID(ctx context.Context, svc session.Service, id string) (session.Session, error) {
	// Try direct UUID lookup first
	if s, err := svc.Get(ctx, id); err == nil {
		return s, nil
	}

	// List all sessions and check for hash matches
	sessions, err := svc.List(ctx)
	if err != nil {
		return session.Session{}, err
	}

	var matches []session.Session
	for _, s := range sessions {
		hash := session.HashID(s.ID)
		if hash == id || strings.HasPrefix(hash, id) {
			matches = append(matches, s)
		}
	}

	if len(matches) == 0 {
		return session.Session{}, fmt.Errorf("session not found: %s", id)
	}

	if len(matches) == 1 {
		return matches[0], nil
	}

	// Ambiguous - show matches like Git does
	var sb strings.Builder
	fmt.Fprintf(&sb, "session ID '%s' is ambiguous. Matches:\n\n", id)
	for _, m := range matches {
		hash := session.HashID(m.ID)
		created := time.Unix(m.CreatedAt, 0).Format("2006-01-02")
		// Keep title on one line by replacing newlines with spaces, and truncate.
		title := strings.ReplaceAll(m.Title, "\n", " ")
		title = ansi.Truncate(title, 50, "…")
		fmt.Fprintf(&sb, "  %s... %q (created %s)\n", hash[:12], title, created)
	}
	sb.WriteString("\nUse more characters or the full hash")
	return session.Session{}, errors.New(sb.String())
}

func runSessionShow(cmd *cobra.Command, args []string) error {
	event.SetNonInteractive(true)

	ctx, svc, cleanup, err := sessionSetup(cmd)
	if err != nil {
		return err
	}
	defer cleanup()

	event.SessionShown(sessionShowJSON)

	sess, err := resolveSessionID(ctx, svc.sessions, args[0])
	if err != nil {
		return err
	}

	if sessionShowByModel {
		report, reportErr := sessionByModelReport(ctx, svc.sessions, svc.messages, sess)
		if reportErr != nil {
			return reportErr
		}
		if sessionShowJSON {
			return outputSessionByModelJSON(cmd.OutOrStdout(), sess, report)
		}
		return outputSessionByModelHuman(cmd.OutOrStdout(), sess, report)
	}

	msgs, err := svc.messages.List(ctx, sess.ID)
	if err != nil {
		return fmt.Errorf("failed to list messages: %w", err)
	}

	msgPtrs := messagePtrs(msgs)
	if sessionShowJSON {
		return outputSessionJSON(cmd.OutOrStdout(), sess, msgPtrs)
	}
	return outputSessionHuman(ctx, svc.cfg, sess, msgPtrs)
}

func runSessionDelete(cmd *cobra.Command, args []string) error {
	event.SetNonInteractive(true)

	ctx, svc, cleanup, err := sessionSetup(cmd)
	if err != nil {
		return err
	}
	defer cleanup()

	event.SessionDeletedCommand(sessionDeleteJSON)

	sess, err := resolveSessionID(ctx, svc.sessions, args[0])
	if err != nil {
		return err
	}

	if err := svc.sessions.Delete(ctx, sess.ID); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	out := cmd.OutOrStdout()
	if sessionDeleteJSON {
		enc := json.NewEncoder(out)
		enc.SetEscapeHTML(false)
		return enc.Encode(sessionMutationResult{
			ID:      session.HashID(sess.ID),
			UUID:    sess.ID,
			Title:   sess.Title,
			Deleted: true,
		})
	}

	fmt.Fprintf(out, "Deleted session %s\n", session.HashID(sess.ID)[:12])
	return nil
}

func runSessionRename(cmd *cobra.Command, args []string) error {
	event.SetNonInteractive(true)

	ctx, svc, cleanup, err := sessionSetup(cmd)
	if err != nil {
		return err
	}
	defer cleanup()

	event.SessionRenamed(sessionRenameJSON)

	sess, err := resolveSessionID(ctx, svc.sessions, args[0])
	if err != nil {
		return err
	}

	newTitle := strings.Join(args[1:], " ")
	if err := svc.sessions.Rename(ctx, sess.ID, newTitle); err != nil {
		return fmt.Errorf("failed to rename session: %w", err)
	}

	out := cmd.OutOrStdout()
	if sessionRenameJSON {
		enc := json.NewEncoder(out)
		enc.SetEscapeHTML(false)
		return enc.Encode(sessionMutationResult{
			ID:      session.HashID(sess.ID),
			UUID:    sess.ID,
			Title:   newTitle,
			Renamed: true,
		})
	}

	fmt.Fprintf(out, "Renamed session %s to %q\n", session.HashID(sess.ID)[:12], newTitle)
	return nil
}

func runSessionLast(cmd *cobra.Command, _ []string) error {
	event.SetNonInteractive(true)

	ctx, svc, cleanup, err := sessionSetup(cmd)
	if err != nil {
		return err
	}
	defer cleanup()

	event.SessionLastShown(sessionLastJSON)

	list, err := svc.sessions.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	if len(list) == 0 {
		return fmt.Errorf("no sessions found")
	}

	sess := list[0]

	msgs, err := svc.messages.List(ctx, sess.ID)
	if err != nil {
		return fmt.Errorf("failed to list messages: %w", err)
	}

	msgPtrs := messagePtrs(msgs)
	if sessionLastJSON {
		return outputSessionJSON(cmd.OutOrStdout(), sess, msgPtrs)
	}
	return outputSessionHuman(ctx, svc.cfg, sess, msgPtrs)
}

const (
	sessionOutputWidth     = 80
	sessionMaxContentWidth = 120
)

func messagePtrs(msgs []message.Message) []*message.Message {
	ptrs := make([]*message.Message, len(msgs))
	for i := range msgs {
		ptrs[i] = &msgs[i]
	}
	return ptrs
}

// maxCostTreeSessions bounds the descendant walk. A subtree larger than this
// is reported truncated rather than walked forever: sub-agents can spawn
// sub-agents, and an unbounded walk over a corrupted parent chain would hang
// the command.
const maxCostTreeSessions = 4096

// sessionCostStore is the slice of [session.Service] the cost walk needs.
type sessionCostStore interface {
	ListChildren(ctx context.Context, parentSessionID string) ([]session.Session, error)
}

// sessionMessageStore is the slice of [message.Service] the cost walk needs.
type sessionMessageStore interface {
	List(ctx context.Context, sessionID string) ([]message.Message, error)
}

// sessionSubtreeNode is one session in the shown session's subtree, paired
// with its own assistant messages. Depth 0 is the shown session itself.
type sessionSubtreeNode struct {
	sess  session.Session
	depth int
	msgs  []*message.Message
}

// collectSessionSubtree returns the shown session followed by every descendant
// session, breadth-first, each with its own messages loaded. Sub-agent cost is
// propagated UP into the parent's `cost` column (see
// coordinator.updateParentSessionCost), so a breakdown that only reads the
// shown session's own messages can never account for the parent's figure — the
// walk is what makes the two reconcile.
//
// It is cycle-safe (a session already visited is skipped) and bounded by
// maxCostTreeSessions; the second return value reports truncation.
func collectSessionSubtree(
	ctx context.Context,
	sessions sessionCostStore,
	messages sessionMessageStore,
	root session.Session,
) ([]sessionSubtreeNode, bool, error) {
	visited := map[string]bool{root.ID: true}
	nodes := []sessionSubtreeNode{{sess: root, depth: 0}}
	truncated := false

	for i := 0; i < len(nodes); i++ {
		msgs, err := messages.List(ctx, nodes[i].sess.ID)
		if err != nil {
			return nil, false, fmt.Errorf("failed to list messages for session %s: %w", nodes[i].sess.ID, err)
		}
		nodes[i].msgs = messagePtrs(msgs)

		children, err := sessions.ListChildren(ctx, nodes[i].sess.ID)
		if err != nil {
			return nil, false, fmt.Errorf("failed to list child sessions of %s: %w", nodes[i].sess.ID, err)
		}
		for _, child := range children {
			if visited[child.ID] {
				continue
			}
			if len(nodes) >= maxCostTreeSessions {
				truncated = true
				break
			}
			visited[child.ID] = true
			nodes = append(nodes, sessionSubtreeNode{sess: child, depth: nodes[i].depth + 1})
		}
		if truncated {
			break
		}
	}
	return nodes, truncated, nil
}

// aggregateSessionCostByModel groups assistant-message Finish usage by
// model/provider across the whole subtree. Every figure it returns is what the
// named model itself consumed, taken from the message that recorded it — a row
// never carries cost that merely passed through the session owning the
// subtree. Messages without Finish usage still count toward MessageCount so
// older sessions remain useful.
func aggregateSessionCostByModel(nodes []sessionSubtreeNode) []sessionModelCost {
	type key struct{ model, provider string }
	order := make([]key, 0)
	by := make(map[key]*sessionModelCost)
	for _, node := range nodes {
		for _, msg := range node.msgs {
			if msg == nil || msg.Role != message.Assistant {
				continue
			}
			k := key{model: msg.Model, provider: msg.Provider}
			if k.model == "" {
				k.model = "unknown"
			}
			if k.provider == "" {
				k.provider = "unknown"
			}
			agg, ok := by[k]
			if !ok {
				agg = &sessionModelCost{Model: k.model, Provider: k.provider}
				by[k] = agg
				order = append(order, k)
			}
			agg.MessageCount++
			fin := msg.FinishPart()
			if fin == nil {
				continue
			}
			agg.PromptTokens += fin.PromptTokens
			agg.CompletionTokens += fin.CompletionTokens
			if node.depth == 0 {
				agg.IncurredCostHere += fin.Cost
			} else {
				agg.IncurredCostSubSessions += fin.Cost
			}
		}
	}
	out := make([]sessionModelCost, 0, len(order))
	for _, k := range order {
		agg := by[k]
		agg.TotalTokens = agg.PromptTokens + agg.CompletionTokens
		agg.IncurredCostTotal = agg.IncurredCostHere + agg.IncurredCostSubSessions
		out = append(out, *agg)
	}
	return out
}

// sessionOwnCost sums the cost recorded on a session's own assistant messages.
// This is the session's OWN spend: unlike session.Cost it excludes anything
// propagated up from a sub-session.
func sessionOwnCost(msgs []*message.Message) float64 {
	var total float64
	for _, msg := range msgs {
		if msg == nil || msg.Role != message.Assistant {
			continue
		}
		if fin := msg.FinishPart(); fin != nil {
			total += fin.Cost
		}
	}
	return total
}

// buildSessionCostReport turns a walked subtree into the reported breakdown.
func buildSessionCostReport(nodes []sessionSubtreeNode, truncated bool) sessionCostReport {
	report := sessionCostReport{
		Truncated: truncated,
		ByModel:   aggregateSessionCostByModel(nodes),
		BySession: make([]sessionCostTreeNode, 0, len(nodes)),
	}
	for _, node := range nodes {
		own := sessionOwnCost(node.msgs)
		if node.depth == 0 {
			report.RecordedCost = node.sess.Cost
			report.IncurredCostHere = own
		} else {
			report.IncurredCostSubSessions += own
			report.SubSessionCount++
		}
		report.BySession = append(report.BySession, sessionCostTreeNode{
			ID:           session.HashID(node.sess.ID),
			UUID:         node.sess.ID,
			Title:        node.sess.Title,
			Depth:        node.depth,
			RecordedCost: node.sess.Cost,
			IncurredCost: own,
		})
	}
	report.IncurredCostTotal = report.IncurredCostHere + report.IncurredCostSubSessions
	report.UnattributedCost = report.RecordedCost - report.IncurredCostTotal
	return report
}

func sessionByModelReport(
	ctx context.Context,
	sessions sessionCostStore,
	messages sessionMessageStore,
	sess session.Session,
) (sessionCostReport, error) {
	nodes, truncated, err := collectSessionSubtree(ctx, sessions, messages, sess)
	if err != nil {
		return sessionCostReport{}, err
	}
	return buildSessionCostReport(nodes, truncated), nil
}

func outputSessionByModelJSON(w io.Writer, sess session.Session, report sessionCostReport) error {
	output := sessionByModelOutput{
		Meta: sessionByModelMeta{
			ID:                      session.HashID(sess.ID),
			UUID:                    sess.ID,
			Title:                   sess.Title,
			Created:                 time.Unix(sess.CreatedAt, 0).Format(time.RFC3339),
			Modified:                time.Unix(sess.UpdatedAt, 0).Format(time.RFC3339),
			ContextPromptTokens:     sess.PromptTokens,
			ContextCompletionTokens: sess.CompletionTokens,
			CostAttribution:         report,
		},
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(output)
}

func outputSessionByModelHuman(w io.Writer, sess session.Session, report sessionCostReport) error {
	fmt.Fprintf(w, "Session: %s\n\n", sess.Title)

	fmt.Fprintln(w, "Cost attribution")
	line := func(label string, amount float64, note string) {
		fmt.Fprintf(w, "  %-30s $%.6f  (%s)\n", label, amount, note)
	}
	line("recorded on this session row", report.RecordedCost,
		"own spend plus cost propagated up from sub-sessions")
	line("incurred by this session", report.IncurredCostHere,
		"this session's own assistant steps only")
	line(fmt.Sprintf("incurred by %d sub-sessions", report.SubSessionCount), report.IncurredCostSubSessions,
		"each sub-session's own assistant steps only")
	line("attributed total", report.IncurredCostTotal,
		"own + sub-sessions; reconciles with the recorded row")
	line("unattributed", report.UnattributedCost,
		"recorded minus attributed; non-zero means steps with no recorded usage")
	if report.Truncated {
		fmt.Fprintf(w, "  WARNING: subtree larger than %d sessions; figures above are incomplete\n", maxCostTreeSessions)
	}
	fmt.Fprintf(w, "\nContext on last step: %d in / %d out (session row counters; these track context\n"+
		"occupancy of the most recent step, NOT a per-turn total, so they do not sum to the\n"+
		"PROMPT/COMPLETION columns below)\n\n",
		sess.PromptTokens, sess.CompletionTokens)

	if len(report.ByModel) == 0 {
		fmt.Fprintln(w, "No assistant messages with model metadata.")
		return nil
	}

	fmt.Fprintln(w, "Every figure below is what the named MODEL itself consumed, summed over this")
	fmt.Fprintln(w, "session and its sub-sessions. No row includes cost propagated up from a")
	fmt.Fprintln(w, "sub-session. $ HERE and $ SUB split the same spend by where it happened.")
	fmt.Fprintf(w, "%-24s %-16s %6s %12s %12s %12s %12s %12s\n",
		"MODEL", "PROVIDER", "MSGS", "PROMPT", "COMPLETION", "$ HERE", "$ SUB", "$ TOTAL")
	for _, row := range report.ByModel {
		fmt.Fprintf(w, "%-24s %-16s %6d %12d %12d %12.6f %12.6f %12.6f\n",
			row.Model, row.Provider, row.MessageCount,
			row.PromptTokens, row.CompletionTokens,
			row.IncurredCostHere, row.IncurredCostSubSessions, row.IncurredCostTotal)
	}

	if len(report.BySession) > 1 {
		fmt.Fprintln(w, "\nSessions in this subtree (RECORDED is the session row including propagated")
		fmt.Fprintln(w, "cost; INCURRED is that session's own steps only, and INCURRED sums to the")
		fmt.Fprintln(w, "attributed total above)")
		fmt.Fprintf(w, "%-6s %-16s %12s %12s %s\n", "DEPTH", "ID", "RECORDED", "INCURRED", "TITLE")
		for _, node := range report.BySession {
			title := strings.ReplaceAll(node.Title, "\n", " ")
			fmt.Fprintf(w, "%-6d %-16s %12.6f %12.6f %s\n",
				node.Depth, node.ID[:min(len(node.ID), 12)],
				node.RecordedCost, node.IncurredCost,
				ansi.Truncate(title, 40, "…"))
		}
	}
	return nil
}

func outputSessionJSON(w io.Writer, sess session.Session, msgs []*message.Message) error {
	skills := extractSkillsFromMessages(msgs)
	output := sessionShowOutput{
		Meta: sessionShowMeta{
			ID:               session.HashID(sess.ID),
			UUID:             sess.ID,
			Title:            sess.Title,
			Created:          time.Unix(sess.CreatedAt, 0).Format(time.RFC3339),
			Modified:         time.Unix(sess.UpdatedAt, 0).Format(time.RFC3339),
			Cost:             sess.Cost,
			PromptTokens:     sess.PromptTokens,
			CompletionTokens: sess.CompletionTokens,
			TotalTokens:      sess.PromptTokens + sess.CompletionTokens,
			Skills:           skills,
		},
		Messages: make([]sessionShowMessage, len(msgs)),
	}

	for i, msg := range msgs {
		output.Messages[i] = sessionShowMessage{
			ID:       msg.ID,
			Role:     string(msg.Role),
			Created:  time.Unix(msg.CreatedAt, 0).Format(time.RFC3339),
			Model:    msg.Model,
			Provider: msg.Provider,
			Parts:    convertParts(msg.Parts),
		}
	}

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(output)
}

func outputSessionHuman(ctx context.Context, cfg *config.ConfigStore, sess session.Session, msgs []*message.Message) error {
	var providerID string
	if cfg != nil {
		providerID = cfg.Config().Models[config.SelectedModelTypeLarge].Provider
	}
	styles := styles.ThemeForProvider(providerID)
	toolResults := chat.BuildToolResultMap(msgs)

	width := sessionOutputWidth
	if w, _, err := term.GetSize(os.Stdout.Fd()); err == nil && w > 0 {
		width = w
	}
	contentWidth := min(width, sessionMaxContentWidth)

	keyStyle := lipgloss.NewStyle().Foreground(charmtone.Damson)
	valStyle := lipgloss.NewStyle().Foreground(charmtone.Malibu)

	hash := session.HashID(sess.ID)[:12]
	created := time.Unix(sess.CreatedAt, 0).Format("Mon Jan 2 15:04:05 2006 -0700")

	skills := extractSkillsFromMessages(msgs)

	// Render to buffer to determine actual height
	var buf strings.Builder

	fmt.Fprintln(&buf, keyStyle.Render("ID:    ")+valStyle.Render(hash))
	fmt.Fprintln(&buf, keyStyle.Render("UUID:  ")+valStyle.Render(sess.ID))
	fmt.Fprintln(&buf, keyStyle.Render("Title: ")+valStyle.Render(sess.Title))
	fmt.Fprintln(&buf, keyStyle.Render("Date:  ")+valStyle.Render(created))
	if len(skills) > 0 {
		skillNames := make([]string, len(skills))
		for i, s := range skills {
			timestamp := time.Unix(sess.CreatedAt, 0).Format("15:04:05 -0700")
			if s.LoadedAt != "" {
				if t, err := time.Parse(time.RFC3339, s.LoadedAt); err == nil {
					timestamp = t.Format("15:04:05 -0700")
				}
			}
			skillNames[i] = fmt.Sprintf("%s (%s)", s.Name, timestamp)
		}
		fmt.Fprintln(&buf, keyStyle.Render("Skills: ")+valStyle.Render(strings.Join(skillNames, ", ")))
	}
	fmt.Fprintln(&buf)

	first := true
	for _, msg := range msgs {
		items := chat.ExtractMessageItems(&styles, msg, toolResults)
		for _, item := range items {
			if !first {
				fmt.Fprintln(&buf)
			}
			first = false
			fmt.Fprintln(&buf, item.Render(contentWidth))
		}
	}
	fmt.Fprintln(&buf)

	contentHeight := strings.Count(buf.String(), "\n")
	w, cleanup, usingPager := sessionWriter(ctx, contentHeight)
	defer cleanup()

	_, err := io.WriteString(w, buf.String())
	// Ignore broken pipe errors when using a pager. This happens when the user
	// exits the pager early (e.g., pressing 'q' in less), which closes the pipe
	// and causes subsequent writes to fail. These errors are expected user behavior.
	if err != nil && usingPager && isBrokenPipe(err) {
		return nil
	}
	return err
}

func isBrokenPipe(err error) bool {
	if err == nil {
		return false
	}
	// Check for syscall.EPIPE (broken pipe)
	if errors.Is(err, syscall.EPIPE) {
		return true
	}
	// Also check for "broken pipe" in the error message
	return strings.Contains(err.Error(), "broken pipe")
}

// sessionWriter returns a writer, cleanup function, and a bool indicating if a pager is used.
// When the content fits within the terminal (or stdout is not a TTY), it returns
// a colorprofile.Writer wrapping stdout. When content exceeds terminal height,
// it starts a pager process (respecting $PAGER, defaulting to "less -R").
func sessionWriter(ctx context.Context, contentHeight int) (io.Writer, func(), bool) {
	// Use NewWriter which automatically detects TTY and strips ANSI when redirected
	if runtime.GOOS == "windows" || !term.IsTerminal(os.Stdout.Fd()) {
		return colorprofile.NewWriter(os.Stdout, os.Environ()), func() {}, false
	}

	_, termHeight, err := term.GetSize(os.Stdout.Fd())
	if err != nil || contentHeight <= termHeight {
		return colorprofile.NewWriter(os.Stdout, os.Environ()), func() {}, false
	}

	// Detect color profile from stderr since stdout is piped to the pager.
	profile := colorprofile.Detect(os.Stderr, os.Environ())

	pager := os.Getenv("PAGER")
	if pager == "" {
		pager = "less -R"
	}

	parts := strings.Fields(pager)
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...) //nolint:gosec
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	pipe, err := cmd.StdinPipe()
	if err != nil {
		return colorprofile.NewWriter(os.Stdout, os.Environ()), func() {}, false
	}

	if err := cmd.Start(); err != nil {
		return colorprofile.NewWriter(os.Stdout, os.Environ()), func() {}, false
	}

	return &colorprofile.Writer{
			Forward: pipe,
			Profile: profile,
		}, func() {
			pipe.Close()
			_ = cmd.Wait()
		}, true
}

type sessionShowMeta struct {
	ID               string             `json:"id"`
	UUID             string             `json:"uuid"`
	Title            string             `json:"title"`
	Created          string             `json:"created"`
	Modified         string             `json:"modified"`
	Cost             float64            `json:"cost"`
	PromptTokens     int64              `json:"prompt_tokens"`
	CompletionTokens int64              `json:"completion_tokens"`
	TotalTokens      int64              `json:"total_tokens"`
	Skills           []sessionShowSkill `json:"skills,omitempty"`
}

// sessionByModelOutput is the `session show --by-model --json` payload. It is
// deliberately a separate shape from sessionShowOutput: the plain `show` meta
// exposes a single `cost` field that carries sub-session spend propagated up
// from children, and reusing it here would reintroduce under a new name the
// exact ambiguity this breakdown exists to remove.
type sessionByModelOutput struct {
	Meta sessionByModelMeta `json:"meta"`
}

type sessionByModelMeta struct {
	ID       string `json:"id"`
	UUID     string `json:"uuid"`
	Title    string `json:"title"`
	Created  string `json:"created"`
	Modified string `json:"modified"`
	// ContextPromptTokens / ContextCompletionTokens are the session row's
	// token counters. They track the context occupancy of the most recent
	// step (see agent.updateSessionTokenCounters), NOT a per-turn total, so
	// they do not sum to the by_model token columns.
	ContextPromptTokens     int64             `json:"context_prompt_tokens"`
	ContextCompletionTokens int64             `json:"context_completion_tokens"`
	CostAttribution         sessionCostReport `json:"cost_attribution"`
}

// sessionCostReport reconciles the session row's recorded cost against the
// spend actually attributable to a model.
type sessionCostReport struct {
	// RecordedCost is sessions.cost exactly as stored: this session's own
	// spend PLUS every sub-session's cost, which the coordinator propagates
	// upward. On its own it says nothing about which model was billed.
	RecordedCost float64 `json:"recorded_cost"`
	// IncurredCostHere is the spend recorded on this session's own assistant
	// messages.
	IncurredCostHere float64 `json:"incurred_cost_here"`
	// IncurredCostSubSessions is the sum of the OWN spend of every descendant
	// session (never their recorded_cost, which would double-count).
	IncurredCostSubSessions float64 `json:"incurred_cost_sub_sessions"`
	// IncurredCostTotal is IncurredCostHere + IncurredCostSubSessions, and
	// equals the sum of by_model[].incurred_cost_total.
	IncurredCostTotal float64 `json:"incurred_cost_total"`
	// UnattributedCost is RecordedCost - IncurredCostTotal. Zero means the
	// breakdown reconciles; non-zero means some spend reached the session row
	// without a per-step usage record (e.g. sessions written before this
	// feature existed).
	UnattributedCost float64               `json:"unattributed_cost"`
	SubSessionCount  int                   `json:"sub_session_count"`
	Truncated        bool                  `json:"truncated,omitempty"`
	ByModel          []sessionModelCost    `json:"by_model"`
	BySession        []sessionCostTreeNode `json:"by_session"`
}

// sessionModelCost is the per-model usage breakdown across a session subtree.
// There is deliberately no plain `cost` field: every cost here is attributed
// to the model that incurred it.
type sessionModelCost struct {
	Model            string `json:"model"`
	Provider         string `json:"provider"`
	MessageCount     int64  `json:"message_count"`
	PromptTokens     int64  `json:"prompt_tokens"`
	CompletionTokens int64  `json:"completion_tokens"`
	TotalTokens      int64  `json:"total_tokens"`
	// IncurredCostHere is what this model cost inside the shown session.
	IncurredCostHere float64 `json:"incurred_cost_here"`
	// IncurredCostSubSessions is what this model cost inside descendant
	// sessions of the shown session.
	IncurredCostSubSessions float64 `json:"incurred_cost_sub_sessions"`
	// IncurredCostTotal is the sum of the two above.
	IncurredCostTotal float64 `json:"incurred_cost_total"`
}

// sessionCostTreeNode is one session in the shown session's subtree.
type sessionCostTreeNode struct {
	ID    string `json:"id"`
	UUID  string `json:"uuid"`
	Title string `json:"title"`
	Depth int    `json:"depth"`
	// RecordedCost is the session row value, including propagated child cost.
	RecordedCost float64 `json:"recorded_cost"`
	// IncurredCost is this session's own spend only.
	IncurredCost float64 `json:"incurred_cost"`
}

type sessionShowSkill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	LoadedAt    string `json:"loaded_at"`
}

type sessionShowMessage struct {
	ID       string            `json:"id"`
	Role     string            `json:"role"`
	Created  string            `json:"created"`
	Model    string            `json:"model,omitempty"`
	Provider string            `json:"provider,omitempty"`
	Parts    []sessionShowPart `json:"parts"`
}

type sessionShowPart struct {
	Type string `json:"type"`

	// Text content
	Text string `json:"text,omitempty"`

	// Reasoning
	Thinking   string `json:"thinking,omitempty"`
	StartedAt  int64  `json:"started_at,omitempty"`
	FinishedAt int64  `json:"finished_at,omitempty"`

	// Tool call
	ToolCallID string `json:"tool_call_id,omitempty"`
	Name       string `json:"name,omitempty"`
	Input      string `json:"input,omitempty"`

	// Tool result
	Content  string `json:"content,omitempty"`
	IsError  bool   `json:"is_error,omitempty"`
	MIMEType string `json:"mime_type,omitempty"`

	// Binary
	Size int64 `json:"size,omitempty"`

	// Image URL
	URL    string `json:"url,omitempty"`
	Detail string `json:"detail,omitempty"`

	// Finish
	Reason string `json:"reason,omitempty"`
	Time   int64  `json:"time,omitempty"`
}

func extractSkillsFromMessages(msgs []*message.Message) []sessionShowSkill {
	var skills []sessionShowSkill
	seen := make(map[string]bool)

	for _, msg := range msgs {
		for _, part := range msg.Parts {
			if tr, ok := part.(message.ToolResult); ok && tr.Metadata != "" {
				var meta tools.ViewResponseMetadata
				if err := json.Unmarshal([]byte(tr.Metadata), &meta); err == nil {
					if meta.ResourceType == tools.ViewResourceSkill && meta.ResourceName != "" {
						if !seen[meta.ResourceName] {
							seen[meta.ResourceName] = true
							skills = append(skills, sessionShowSkill{
								Name:        meta.ResourceName,
								Description: meta.ResourceDescription,
								LoadedAt:    time.Unix(msg.CreatedAt, 0).Format(time.RFC3339),
							})
						}
					}
				}
			}
		}
	}

	sort.Slice(skills, func(i, j int) bool {
		if skills[i].LoadedAt == skills[j].LoadedAt {
			return skills[i].Name < skills[j].Name
		}
		return skills[i].LoadedAt < skills[j].LoadedAt
	})

	return skills
}

func convertParts(parts []message.ContentPart) []sessionShowPart {
	result := make([]sessionShowPart, 0, len(parts))
	for _, part := range parts {
		switch p := part.(type) {
		case message.TextContent:
			result = append(result, sessionShowPart{
				Type: "text",
				Text: p.Text,
			})
		case message.ReasoningContent:
			result = append(result, sessionShowPart{
				Type:       "reasoning",
				Thinking:   p.Thinking,
				StartedAt:  p.StartedAt,
				FinishedAt: p.FinishedAt,
			})
		case message.ToolCall:
			result = append(result, sessionShowPart{
				Type:       "tool_call",
				ToolCallID: p.ID,
				Name:       p.Name,
				Input:      p.Input,
			})
		case message.ToolResult:
			result = append(result, sessionShowPart{
				Type:       "tool_result",
				ToolCallID: p.ToolCallID,
				Name:       p.Name,
				Content:    p.Content,
				IsError:    p.IsError,
				MIMEType:   p.MIMEType,
			})
		case message.BinaryContent:
			result = append(result, sessionShowPart{
				Type:     "binary",
				MIMEType: p.MIMEType,
				Size:     int64(len(p.Data)),
			})
		case message.ImageURLContent:
			result = append(result, sessionShowPart{
				Type:   "image_url",
				URL:    p.URL,
				Detail: p.Detail,
			})
		case message.Finish:
			result = append(result, sessionShowPart{
				Type:   "finish",
				Reason: string(p.Reason),
				Time:   p.Time,
			})
		default:
			result = append(result, sessionShowPart{
				Type: "unknown",
			})
		}
	}
	return result
}

type sessionShowOutput struct {
	Meta     sessionShowMeta      `json:"meta"`
	Messages []sessionShowMessage `json:"messages"`
}
