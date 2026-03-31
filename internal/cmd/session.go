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
	"strings"
	"syscall"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/db"
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
	sessionListJSON   bool
	sessionShowJSON   bool
	sessionLastJSON   bool
	sessionDeleteJSON bool
	sessionRenameJSON bool
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
	Long:  "Show session details. Use --json for machine-readable output. ID can be a UUID, full hash, or hash prefix.",
	Args:  cobra.ExactArgs(1),
	RunE:  runSessionShow,
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
}

func sessionSetup(cmd *cobra.Command) (context.Context, *sessionServices, func(), error) {
	dataDir, _ := cmd.Flags().GetString("data-dir")
	cwd, _ := cmd.Flags().GetString("cwd")
	ctx := cmd.Context()

	if dataDir == "" {
		cfg, err := config.Init(cwd, "", false)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to initialize config: %w", err)
		}
		dataDir = cfg.Config().Options.DataDirectory
	}

	conn, err := db.Connect(ctx, dataDir)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	queries := db.New(conn)
	svc := &sessionServices{
		sessions: session.NewService(queries, conn),
		messages: message.NewService(queries),
	}
	return ctx, svc, func() { conn.Close() }, nil
}

func runSessionList(cmd *cobra.Command, _ []string) error {
	ctx, svc, cleanup, err := sessionSetup(cmd)
	if err != nil {
		return err
	}
	defer cleanup()

	list, err := svc.sessions.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	if sessionListJSON {
		out := cmd.OutOrStdout()
		output := make([]sessionJSON, len(list))
		for i, s := range list {
			output[i] = sessionJSON{
				ID:                     session.HashID(s.ID),
				UUID:                   s.ID,
				Kind:                   string(s.Kind),
				Title:                  s.Title,
				Created:                time.Unix(s.CreatedAt, 0).Format(time.RFC3339),
				Modified:               time.Unix(s.UpdatedAt, 0).Format(time.RFC3339),
				HandoffSourceSessionID: s.HandoffSourceSessionID,
				HandoffGoal:            s.HandoffGoal,
				HandoffRelevantFiles:   s.HandoffRelevantFiles,
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
	titleWidth := width - 34
	if titleWidth < 10 {
		titleWidth = 10
	}

	var writeErr error
	for _, s := range list {
		hash := session.HashID(s.ID)[:7]
		date := time.Unix(s.CreatedAt, 0).Format(time.RFC3339)
		title := strings.ReplaceAll(s.Title, "\n", " ")
		if s.Kind == session.KindHandoff {
			title = "[handoff] " + title
		}
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
	ID                     string   `json:"id"`
	UUID                   string   `json:"uuid"`
	Kind                   string   `json:"kind"`
	Title                  string   `json:"title"`
	Created                string   `json:"created"`
	Modified               string   `json:"modified"`
	HandoffSourceSessionID string   `json:"handoff_source_session_id,omitempty"`
	HandoffGoal            string   `json:"handoff_goal,omitempty"`
	HandoffRelevantFiles   []string `json:"handoff_relevant_files,omitempty"`
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
	// Try direct UUID lookup first.
	if s, err := svc.Get(ctx, id); err == nil {
		return s, nil
	}

	// List all sessions and check for hash matches.
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

	// Ambiguous - show matches like Git does.
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
	ctx, svc, cleanup, err := sessionSetup(cmd)
	if err != nil {
		return err
	}
	defer cleanup()

	sess, err := resolveSessionID(ctx, svc.sessions, args[0])
	if err != nil {
		return err
	}

	msgs, err := svc.messages.List(ctx, sess.ID)
	if err != nil {
		return fmt.Errorf("failed to list messages: %w", err)
	}

	msgPtrs := messagePtrs(msgs)
	if sessionShowJSON {
		return outputSessionJSON(cmd.OutOrStdout(), sess, msgPtrs)
	}
	return outputSessionHuman(ctx, sess, msgPtrs)
}

func runSessionDelete(cmd *cobra.Command, args []string) error {
	ctx, svc, cleanup, err := sessionSetup(cmd)
	if err != nil {
		return err
	}
	defer cleanup()

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
	ctx, svc, cleanup, err := sessionSetup(cmd)
	if err != nil {
		return err
	}
	defer cleanup()

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
	ctx, svc, cleanup, err := sessionSetup(cmd)
	if err != nil {
		return err
	}
	defer cleanup()

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
	return outputSessionHuman(ctx, sess, msgPtrs)
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

func outputSessionJSON(w io.Writer, sess session.Session, msgs []*message.Message) error {
	output := sessionShowOutput{
		Meta: sessionShowMeta{
			ID:                     session.HashID(sess.ID),
			UUID:                   sess.ID,
			Kind:                   string(sess.Kind),
			Title:                  sess.Title,
			Created:                time.Unix(sess.CreatedAt, 0).Format(time.RFC3339),
			Modified:               time.Unix(sess.UpdatedAt, 0).Format(time.RFC3339),
			Cost:                   sess.Cost,
			PromptTokens:           sess.PromptTokens,
			CompletionTokens:       sess.CompletionTokens,
			TotalTokens:            sess.PromptTokens + sess.CompletionTokens,
			HandoffSourceSessionID: sess.HandoffSourceSessionID,
			HandoffGoal:            sess.HandoffGoal,
			HandoffDraftPrompt:     sess.HandoffDraftPrompt,
			HandoffRelevantFiles:   sess.HandoffRelevantFiles,
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

func outputSessionHuman(ctx context.Context, sess session.Session, msgs []*message.Message) error {
	sty := styles.DefaultStyles()
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

	// Render to buffer to determine actual height.
	var buf strings.Builder

	fmt.Fprintln(&buf, keyStyle.Render("ID:    ")+valStyle.Render(hash))
	fmt.Fprintln(&buf, keyStyle.Render("UUID:  ")+valStyle.Render(sess.ID))
	fmt.Fprintln(&buf, keyStyle.Render("Kind:  ")+valStyle.Render(string(sess.Kind)))
	fmt.Fprintln(&buf, keyStyle.Render("Title: ")+valStyle.Render(sess.Title))
	fmt.Fprintln(&buf, keyStyle.Render("Date:  ")+valStyle.Render(created))
	if sess.Kind == session.KindHandoff {
		if sess.HandoffSourceSessionID != "" {
			fmt.Fprintln(&buf, keyStyle.Render("From:  ")+valStyle.Render(sess.HandoffSourceSessionID))
		}
		if sess.HandoffGoal != "" {
			fmt.Fprintln(&buf, keyStyle.Render("Goal:  ")+valStyle.Render(sess.HandoffGoal))
		}
		if sess.HandoffDraftPrompt != "" {
			fmt.Fprintln(&buf, keyStyle.Render("Draft: ")+valStyle.Render(sess.HandoffDraftPrompt))
		}
		if len(sess.HandoffRelevantFiles) > 0 {
			fmt.Fprintln(&buf, keyStyle.Render("Files: ")+valStyle.Render(strings.Join(sess.HandoffRelevantFiles, ", ")))
		}
	}
	fmt.Fprintln(&buf)

	first := true
	for _, msg := range msgs {
		items := chat.ExtractMessageItems(&sty, msg, toolResults)
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
	// Check for syscall.EPIPE (broken pipe).
	if errors.Is(err, syscall.EPIPE) {
		return true
	}
	// Also check for "broken pipe" in the error message.
	return strings.Contains(err.Error(), "broken pipe")
}

// sessionWriter returns a writer, cleanup function, and a bool indicating if a pager is used.
// When the content fits within the terminal (or stdout is not a TTY), it returns
// a colorprofile.Writer wrapping stdout. When content exceeds terminal height,
// it starts a pager process (respecting $PAGER, defaulting to "less -R").
func sessionWriter(ctx context.Context, contentHeight int) (io.Writer, func(), bool) {
	// Use NewWriter which automatically detects TTY and strips ANSI when redirected.
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
	if len(parts) == 0 {
		return colorprofile.NewWriter(os.Stdout, os.Environ()), func() {}, false
	}
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
	ID                     string   `json:"id"`
	UUID                   string   `json:"uuid"`
	Kind                   string   `json:"kind"`
	Title                  string   `json:"title"`
	Created                string   `json:"created"`
	Modified               string   `json:"modified"`
	Cost                   float64  `json:"cost"`
	PromptTokens           int64    `json:"prompt_tokens"`
	CompletionTokens       int64    `json:"completion_tokens"`
	TotalTokens            int64    `json:"total_tokens"`
	HandoffSourceSessionID string   `json:"handoff_source_session_id,omitempty"`
	HandoffGoal            string   `json:"handoff_goal,omitempty"`
	HandoffDraftPrompt     string   `json:"handoff_draft_prompt,omitempty"`
	HandoffRelevantFiles   []string `json:"handoff_relevant_files,omitempty"`
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
	Content       string                    `json:"content,omitempty"`
	IsError       bool                      `json:"is_error,omitempty"`
	MIMEType      string                    `json:"mime_type,omitempty"`
	Metadata      string                    `json:"metadata,omitempty"`
	SubtaskResult *sessionShowSubtaskResult `json:"subtask_result,omitempty"`

	// Binary
	Size int64 `json:"size,omitempty"`

	// Image URL
	URL    string `json:"url,omitempty"`
	Detail string `json:"detail,omitempty"`

	// Finish
	Reason string `json:"reason,omitempty"`
	Time   int64  `json:"time,omitempty"`
}

type sessionShowSubtaskResult struct {
	ChildSessionID   string `json:"child_session_id,omitempty"`
	ParentToolCallID string `json:"parent_tool_call_id,omitempty"`
	Status           string `json:"status,omitempty"`
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
			part := sessionShowPart{
				Type:       "tool_result",
				ToolCallID: p.ToolCallID,
				Name:       p.Name,
				Content:    p.Content,
				IsError:    p.IsError,
				MIMEType:   p.MIMEType,
				Metadata:   p.Metadata,
			}
			if subtask, ok := p.SubtaskResult(); ok {
				part.SubtaskResult = &sessionShowSubtaskResult{
					ChildSessionID:   subtask.ChildSessionID,
					ParentToolCallID: subtask.ParentToolCallID,
					Status:           string(subtask.Status),
				}
			}
			result = append(result, part)
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
				Type:    "finish",
				Reason:  string(p.Reason),
				Time:    p.Time,
				Text:    p.Message,
				Content: p.Details,
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
