package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
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
	Aliases: []string{"sessions"},
	Short:   "Manage sessions",
	Long:    "Manage Crush sessions. Agents can use --json for machine-readable output.",
}

var (
	sessionListJSON   bool
	sessionShowJSON   bool
	sessionDeleteJSON bool
	sessionRenameJSON bool
)

var sessionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all sessions",
	Long:  "List all sessions. Use --json for machine-readable output.",
	RunE:  runSessionList,
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
	Long:  "Show the most recently modified session. Use --json for machine-readable output.",
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
	sessionLastCmd.Flags().BoolVar(&sessionShowJSON, "json", false, "output in JSON format")
	sessionDeleteCmd.Flags().BoolVar(&sessionDeleteJSON, "json", false, "output in JSON format")
	sessionRenameCmd.Flags().BoolVar(&sessionRenameJSON, "json", false, "output in JSON format")
	sessionCmd.AddCommand(sessionListCmd)
	sessionCmd.AddCommand(sessionShowCmd)
	sessionCmd.AddCommand(sessionLastCmd)
	sessionCmd.AddCommand(sessionDeleteCmd)
	sessionCmd.AddCommand(sessionRenameCmd)
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

	w, cleanup := sessionWriter(len(list))
	defer cleanup()

	hashStyle := lipgloss.NewStyle().Foreground(charmtone.Malibu)
	dateStyle := lipgloss.NewStyle().Foreground(charmtone.Damson)

	for _, s := range list {
		hash := session.HashID(s.ID)[:7]
		date := time.Unix(s.CreatedAt, 0).Format(time.RFC3339)
		title := strings.ReplaceAll(s.Title, "\n", " ")
		title = ansi.Truncate(title, 60, "…")
		fmt.Fprintln(w, hashStyle.Render(hash), dateStyle.Render(date), title)
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
	fmt.Fprintf(&sb, "error: session ID '%s' is ambiguous. Matches:\n", id)
	for _, m := range matches {
		hash := session.HashID(m.ID)
		created := time.Unix(m.CreatedAt, 0).Format("2006-01-02")
		fmt.Fprintf(&sb, "  %s... %q (created %s)\n", hash[:12], m.Title, created)
	}
	sb.WriteString("Use more characters or the full hash.\n")
	return session.Session{}, fmt.Errorf("%s", sb.String())
}

func runSessionShow(cmd *cobra.Command, args []string) error {
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
	messages := message.NewService(queries)

	sess, err := resolveSessionID(ctx, sessions, args[0])
	if err != nil {
		return err
	}

	msgs, err := messages.List(ctx, sess.ID)
	if err != nil {
		return fmt.Errorf("failed to list messages: %w", err)
	}

	msgPtrs := messagePtrs(msgs)
	if sessionShowJSON {
		return outputSessionJSON(cmd.OutOrStdout(), sess, msgPtrs)
	}
	return outputSessionHuman(sess, msgPtrs)
}

func runSessionDelete(cmd *cobra.Command, args []string) error {
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

	sess, err := resolveSessionID(ctx, sessions, args[0])
	if err != nil {
		return err
	}

	if err := sessions.Delete(ctx, sess.ID); err != nil {
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

	sess, err := resolveSessionID(ctx, sessions, args[0])
	if err != nil {
		return err
	}

	newTitle := strings.Join(args[1:], " ")
	if err := sessions.Rename(ctx, sess.ID, newTitle); err != nil {
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
	messages := message.NewService(queries)

	list, err := sessions.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	if len(list) == 0 {
		return fmt.Errorf("no sessions found")
	}

	sess := list[0]

	msgs, err := messages.List(ctx, sess.ID)
	if err != nil {
		return fmt.Errorf("failed to list messages: %w", err)
	}

	msgPtrs := messagePtrs(msgs)
	if sessionShowJSON {
		return outputSessionJSON(cmd.OutOrStdout(), sess, msgPtrs)
	}
	return outputSessionHuman(sess, msgPtrs)
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
			ID:               session.HashID(sess.ID),
			UUID:             sess.ID,
			Title:            sess.Title,
			Created:          time.Unix(sess.CreatedAt, 0).Format(time.RFC3339),
			Modified:         time.Unix(sess.UpdatedAt, 0).Format(time.RFC3339),
			Cost:             sess.Cost,
			PromptTokens:     sess.PromptTokens,
			CompletionTokens: sess.CompletionTokens,
			TotalTokens:      sess.PromptTokens + sess.CompletionTokens,
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

func outputSessionHuman(sess session.Session, msgs []*message.Message) error {
	sty := styles.DefaultStyles()
	toolResults := chat.BuildToolResultMap(msgs)

	width := sessionOutputWidth
	if w, _, err := term.GetSize(os.Stdout.Fd()); err == nil && w > 0 {
		width = w
	}
	contentWidth := min(width, sessionMaxContentWidth)

	// Estimate content height: header (4 lines) + blank lines between messages + message content
	contentHeight := 5 + len(msgs)*3

	w, cleanup := sessionWriter(contentHeight)
	defer cleanup()

	keyStyle := lipgloss.NewStyle().Foreground(charmtone.Damson)
	valStyle := lipgloss.NewStyle().Foreground(charmtone.Malibu)

	hash := session.HashID(sess.ID)
	created := time.Unix(sess.CreatedAt, 0).Format("Mon Jan 2 15:04:05 2006 -0700")

	fmt.Fprintln(w, keyStyle.Render("ID:    ")+valStyle.Render(hash))
	fmt.Fprintln(w, keyStyle.Render("Title: ")+valStyle.Render(sess.Title))
	fmt.Fprintln(w, keyStyle.Render("Date:  ")+valStyle.Render(created))
	fmt.Fprintln(w)

	first := true
	for _, msg := range msgs {
		items := chat.ExtractMessageItems(&sty, msg, toolResults)
		for _, item := range items {
			if !first {
				fmt.Fprintln(w)
			}
			first = false
			fmt.Fprintln(w, item.Render(contentWidth))
		}
	}
	fmt.Fprintln(w)

	return nil
}

// sessionWriter returns a writer and cleanup function based on content height.
// When the content fits within the terminal (or stdout is not a TTY), it returns
// a colorprofile.Writer wrapping stdout. When content exceeds terminal height,
// it starts a pager process (respecting $PAGER, defaulting to "less -R").
func sessionWriter(contentHeight int) (io.Writer, func()) {
	// Use NewWriter which automatically detects TTY and strips ANSI when redirected
	if runtime.GOOS == "windows" || !term.IsTerminal(os.Stdout.Fd()) {
		return colorprofile.NewWriter(os.Stdout, os.Environ()), func() {}
	}

	_, termHeight, err := term.GetSize(os.Stdout.Fd())
	if err != nil || contentHeight <= termHeight {
		return colorprofile.NewWriter(os.Stdout, os.Environ()), func() {}
	}

	profile := colorprofile.Detect(os.Stderr, os.Environ())

	pager := os.Getenv("PAGER")
	if pager == "" {
		pager = "less -R"
	}

	parts := strings.Fields(pager)
	cmd := exec.Command(parts[0], parts[1:]...) //nolint:gosec
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	pipe, err := cmd.StdinPipe()
	if err != nil {
		return colorprofile.NewWriter(os.Stdout, os.Environ()), func() {}
	}

	if err := cmd.Start(); err != nil {
		return colorprofile.NewWriter(os.Stdout, os.Environ()), func() {}
	}

	return &colorprofile.Writer{
		Forward: pipe,
		Profile: profile,
	}, func() {
		pipe.Close()
		_ = cmd.Wait()
	}
}

type sessionShowMeta struct {
	ID               string  `json:"id"`
	UUID             string  `json:"uuid"`
	Title            string  `json:"title"`
	Created          string  `json:"created"`
	Modified         string  `json:"modified"`
	Cost             float64 `json:"cost"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
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
		}
	}
	return result
}

type sessionShowOutput struct {
	Meta     sessionShowMeta      `json:"meta"`
	Messages []sessionShowMessage `json:"messages"`
}
