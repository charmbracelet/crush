package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/client"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
)

// Options configures the Telegram bridge.
type Options struct {
	Client    *client.Client
	Workspace proto.Workspace
	Token     string
	ChatID    int64
	// BaseURL overrides the Telegram API base for tests.
	// Default "https://api.telegram.org".
	BaseURL string
}

type pendingPerm struct {
	req   proto.PermissionRequest
	msgID int64
}

type bridge struct {
	mu         sync.Mutex
	opts       Options
	tg         *api
	sessionID  string
	titles     map[string]string
	runs       map[string]string // runID -> session ID
	pendPerms  map[string]pendingPerm
	sessions   []proto.Session
	msgSession map[int64]string
	msgOrder   []int64
	// reconnectNotify tracks whether we already told the chat about a
	// lost connection so we only notify once per outage.
	reconnectNotify bool
}

// Run starts the Telegram bridge and blocks until ctx is cancelled.
func Run(ctx context.Context, opts Options) error {
	if opts.Client == nil {
		return fmt.Errorf("telegram bridge: client is required")
	}
	if opts.Token == "" {
		return fmt.Errorf("telegram bridge: token is required")
	}
	if opts.ChatID == 0 {
		return fmt.Errorf("telegram bridge: chat id is required")
	}

	tg := newAPI(opts.Token, opts.BaseURL)
	me, err := tg.getMe(ctx)
	if err != nil {
		return fmt.Errorf("telegram bot token invalid: %w", err)
	}
	slog.Info("Telegram bot authenticated", "username", me.Username, "id", me.ID)

	b := &bridge{
		opts:       opts,
		tg:         tg,
		titles:     make(map[string]string),
		runs:       make(map[string]string),
		pendPerms:  make(map[string]pendingPerm),
		msgSession: make(map[int64]string),
	}

	if err := b.waitForAgent(ctx); err != nil {
		return err
	}

	if err := b.pickInitialSession(ctx); err != nil {
		return err
	}

	title := b.sessionTitle(b.sessionID)
	startup := fmt.Sprintf("🟢 crush bridge online\nproject: %s\nsession: %s", opts.Workspace.Path, title)
	if _, err := b.sendPlain(ctx, b.sessionID, startup); err != nil {
		slog.Warn("Failed to send startup message", "error", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		b.eventPump(ctx)
	}()
	go func() {
		defer wg.Done()
		b.typingLoop(ctx)
	}()

	// Update pump runs in this goroutine.
	b.updatePump(ctx)

	// Best-effort shutdown notice.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _ = b.sendPlain(shutdownCtx, b.sessionID, "🔴 bridge shutting down")

	// Wait briefly for background pumps to notice ctx cancel.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
	return nil
}

func (b *bridge) waitForAgent(ctx context.Context) error {
	wsID := b.opts.Workspace.ID
	if err := b.opts.Client.InitiateAgentProcessing(ctx, wsID); err != nil {
		slog.Warn("Failed to initiate agent processing", "error", err)
	}
	timeout := time.After(30 * time.Second)
	for {
		info, err := b.opts.Client.GetAgentInfo(ctx, wsID)
		if err == nil && info.IsReady {
			if err := b.opts.Client.UpdateAgent(ctx, wsID); err != nil {
				slog.Warn("Failed to update agent", "error", err)
			}
			return nil
		}
		select {
		case <-timeout:
			if err != nil {
				return fmt.Errorf("timeout waiting for agent: %w", err)
			}
			return fmt.Errorf("timeout waiting for agent readiness")
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func (b *bridge) pickInitialSession(ctx context.Context) error {
	wsID := b.opts.Workspace.ID
	sessions, err := b.opts.Client.ListSessions(ctx, wsID)
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}
	b.refreshTitles(sessions)

	var best *proto.Session
	for i := range sessions {
		s := &sessions[i]
		if !isTopLevelSession(*s) {
			continue
		}
		if best == nil || s.UpdatedAt > best.UpdatedAt {
			best = s
		}
	}
	if best != nil {
		b.sessionID = best.ID
		return nil
	}
	sess, err := b.opts.Client.CreateSession(ctx, wsID, "telegram")
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	b.sessionID = sess.ID
	b.titles[sess.ID] = sess.Title
	return nil
}

func isTopLevelSession(s proto.Session) bool {
	if s.ParentSessionID != "" {
		return false
	}
	if strings.HasPrefix(s.ID, "title-") {
		return false
	}
	if strings.Contains(s.ID, "$$") {
		return false
	}
	return true
}

func (b *bridge) refreshTitles(sessions []proto.Session) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, s := range sessions {
		if !isTopLevelSession(s) {
			continue
		}
		b.titles[s.ID] = s.Title
	}
	b.sessions = sessions
}

func (b *bridge) knownSession(id string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	_, ok := b.titles[id]
	return ok
}

func (b *bridge) sessionTitle(id string) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if t, ok := b.titles[id]; ok && t != "" {
		return t
	}
	if id == "" {
		return "(none)"
	}
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func (b *bridge) activeSession() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.sessionID
}

func (b *bridge) setActiveSession(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sessionID = id
}

// tagPrefix returns a session tag for outbound messages when the
// session is not the active one.
func (b *bridge) tagPrefix(sessionID string) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if sessionID == "" || sessionID == b.sessionID {
		return ""
	}
	title := b.titles[sessionID]
	if title == "" {
		title = sessionID
		if len(title) > 8 {
			title = title[:8]
		}
	}
	return "📁 " + title + "\n"
}

func (b *bridge) recordOutbound(msgID int64, sessionID string) {
	if msgID == 0 || sessionID == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, exists := b.msgSession[msgID]; !exists {
		b.msgOrder = append(b.msgOrder, msgID)
	}
	b.msgSession[msgID] = sessionID
	const maxTracked = 500
	for len(b.msgOrder) > maxTracked {
		old := b.msgOrder[0]
		b.msgOrder = b.msgOrder[1:]
		delete(b.msgSession, old)
	}
}

func (b *bridge) sessionFromReply(replyMsgID int64) (string, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	s, ok := b.msgSession[replyMsgID]
	return s, ok
}

func (b *bridge) sendPlain(ctx context.Context, sessionID, text string) (Message, error) {
	prefix := b.tagPrefix(sessionID)
	chunks := formatChunks(Chunk(prefix+text, 3900))
	var last Message
	for _, c := range chunks {
		msg, err := b.tg.sendMessage(ctx, b.opts.ChatID, c, nil)
		if err != nil {
			return Message{}, err
		}
		last = msg
		b.recordOutbound(msg.MessageID, sessionID)
	}
	return last, nil
}

func (b *bridge) sendHTML(ctx context.Context, sessionID, text string, kb *InlineKeyboardMarkup) (Message, error) {
	prefix := b.tagPrefix(sessionID)
	// HTML messages are assumed pre-sized by callers for permission
	// summaries; still guard with chunking in plain mode if oversized.
	full := prefix + text
	if utf16Len(full) > 3900 {
		// Fall back to plain chunked delivery without keyboard.
		return b.sendPlain(ctx, sessionID, stripRoughHTML(full))
	}
	msg, err := b.tg.sendMessage(ctx, b.opts.ChatID, full, &sendOpts{HTML: true, Keyboard: kb})
	if err != nil {
		return Message{}, err
	}
	b.recordOutbound(msg.MessageID, sessionID)
	return msg, nil
}

func stripRoughHTML(s string) string {
	// Best-effort strip of a few tags we emit; used only as a fallback
	// when an HTML message exceeds the size cap.
	replacer := strings.NewReplacer(
		"<b>", "", "</b>", "",
		"<pre>", "", "</pre>", "",
		"<code>", "", "</code>", "",
		"&lt;", "<", "&gt;", ">", "&amp;", "&",
	)
	return replacer.Replace(s)
}

// --- update pump (Telegram → crush) ---

func (b *bridge) updatePump(ctx context.Context) {
	var offset int64
	for {
		if ctx.Err() != nil {
			return
		}
		updates, err := b.tg.getUpdates(ctx, offset, 50)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Warn("Telegram getUpdates failed", "error", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(3 * time.Second):
			}
			continue
		}
		for _, u := range updates {
			offset = u.UpdateID + 1
			b.handleUpdate(ctx, u)
		}
	}
}

func (b *bridge) handleUpdate(ctx context.Context, u Update) {
	if u.CallbackQuery != nil {
		b.handleCallback(ctx, u.CallbackQuery)
		return
	}
	if u.Message == nil {
		return
	}
	msg := u.Message
	if msg.Chat.ID != b.opts.ChatID {
		slog.Info("Rejected message from unauthorized chat", "chat_id", msg.Chat.ID)
		return
	}
	if len(msg.Photo) > 0 || msg.Document != nil {
		_, _ = b.sendPlain(ctx, b.activeSession(), "📎 attachments not supported yet")
		return
	}
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return
	}
	if strings.HasPrefix(text, "/") {
		b.handleCommand(ctx, text)
		return
	}
	b.handlePrompt(ctx, msg, text)
}

func (b *bridge) handleCommand(ctx context.Context, text string) {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return
	}
	cmd := fields[0]
	if at := strings.IndexByte(cmd, '@'); at >= 0 {
		cmd = cmd[:at]
	}
	cmd = strings.ToLower(cmd)
	args := fields[1:]

	switch cmd {
	case "/start", "/help":
		_, _ = b.sendPlain(ctx, b.activeSession(), helpText)
	case "/new":
		title := "telegram"
		if len(args) > 0 {
			title = strings.Join(args, " ")
		}
		sess, err := b.opts.Client.CreateSession(ctx, b.opts.Workspace.ID, title)
		if err != nil {
			_, _ = b.sendPlain(ctx, b.activeSession(), "❌ "+err.Error())
			return
		}
		b.mu.Lock()
		b.sessionID = sess.ID
		b.titles[sess.ID] = sess.Title
		b.mu.Unlock()
		_, _ = b.sendPlain(ctx, sess.ID, "🆕 session: "+sess.Title)
	case "/sessions":
		b.cmdSessions(ctx)
	case "/use":
		b.cmdUse(ctx, args)
	case "/status":
		b.cmdStatus(ctx)
	case "/cancel":
		b.cmdCancel(ctx)
	default:
		_, _ = b.sendPlain(ctx, b.activeSession(), "unknown command, try /help")
	}
}

const helpText = `crush telegram bridge

/help — this message
/new [title] — start a new session
/sessions — list sessions
/use <n> — switch to session n from /sessions
/status — agent + session status
/cancel — cancel the active run

Send any other text as a prompt to the active session.
Reply to a bridge message to target that session instead.`

func (b *bridge) cmdSessions(ctx context.Context) {
	sessions, err := b.opts.Client.ListSessions(ctx, b.opts.Workspace.ID)
	if err != nil {
		_, _ = b.sendPlain(ctx, b.activeSession(), "❌ "+err.Error())
		return
	}
	var top []proto.Session
	for _, s := range sessions {
		if isTopLevelSession(s) {
			top = append(top, s)
		}
	}
	sort.Slice(top, func(i, j int) bool {
		return top[i].UpdatedAt > top[j].UpdatedAt
	})
	if len(top) > 10 {
		top = top[:10]
	}
	b.mu.Lock()
	b.sessions = top
	active := b.sessionID
	for _, s := range top {
		b.titles[s.ID] = s.Title
	}
	b.mu.Unlock()

	if len(top) == 0 {
		_, _ = b.sendPlain(ctx, active, "no sessions")
		return
	}
	var lines []string
	for i, s := range top {
		marker := "  "
		if s.ID == active {
			marker = "▶ "
		}
		title := s.Title
		if title == "" {
			title = s.ID
			if len(title) > 8 {
				title = title[:8]
			}
		}
		lines = append(lines, fmt.Sprintf("%s%d. %s — %d msgs, $%.2f",
			marker, i+1, title, s.MessageCount, s.Cost))
	}
	_, _ = b.sendPlain(ctx, active, strings.Join(lines, "\n"))
}

func (b *bridge) cmdUse(ctx context.Context, args []string) {
	if len(args) < 1 {
		_, _ = b.sendPlain(ctx, b.activeSession(), "usage: /use <n>")
		return
	}
	n, err := strconv.Atoi(args[0])
	if err != nil || n < 1 {
		_, _ = b.sendPlain(ctx, b.activeSession(), "usage: /use <n>")
		return
	}
	b.mu.Lock()
	sessions := b.sessions
	b.mu.Unlock()
	if n > len(sessions) {
		_, _ = b.sendPlain(ctx, b.activeSession(), "index out of range; run /sessions first")
		return
	}
	s := sessions[n-1]
	b.setActiveSession(s.ID)
	title := s.Title
	if title == "" {
		title = s.ID
	}
	_, _ = b.sendPlain(ctx, s.ID, "▶ active session: "+title)
}

func (b *bridge) cmdStatus(ctx context.Context) {
	wsID := b.opts.Workspace.ID
	active := b.activeSession()
	info, err := b.opts.Client.GetAgentInfo(ctx, wsID)
	if err != nil {
		_, _ = b.sendPlain(ctx, active, "❌ "+err.Error())
		return
	}
	sessInfo, err := b.opts.Client.GetAgentSessionInfo(ctx, wsID, active)
	if err != nil {
		_, _ = b.sendPlain(ctx, active, "❌ "+err.Error())
		return
	}
	queued, _ := b.opts.Client.GetAgentSessionQueuedPrompts(ctx, wsID, active)
	ready := "not ready"
	if info.IsReady {
		ready = "ready"
	}
	busy := "idle"
	if sessInfo.IsBusy || info.IsBusy {
		busy = "busy"
	}
	model := info.Model.ID
	if model == "" {
		model = "(none)"
	}
	text := fmt.Sprintf(
		"model: %s\nagent: %s / %s\nsession: %s\nqueued: %d\ntokens: %d in / %d out\ncost: $%.4f",
		model, ready, busy, b.sessionTitle(active),
		queued, sessInfo.PromptTokens, sessInfo.CompletionTokens, sessInfo.Cost,
	)
	_, _ = b.sendPlain(ctx, active, text)
}

func (b *bridge) cmdCancel(ctx context.Context) {
	active := b.activeSession()
	if err := b.opts.Client.CancelAgentSession(ctx, b.opts.Workspace.ID, active); err != nil {
		_, _ = b.sendPlain(ctx, active, "❌ "+err.Error())
		return
	}
	_, _ = b.sendPlain(ctx, active, "🛑 cancel requested")
}

func (b *bridge) handlePrompt(ctx context.Context, msg *Message, text string) {
	target := b.activeSession()
	if msg.ReplyToMessage != nil {
		if sid, ok := b.sessionFromReply(msg.ReplyToMessage.MessageID); ok {
			target = sid
		}
	}
	runID := uuid.New().String()
	b.mu.Lock()
	b.runs[runID] = target
	b.mu.Unlock()

	_ = b.tg.sendChatAction(ctx, b.opts.ChatID, "typing")

	if err := b.opts.Client.SendMessage(ctx, b.opts.Workspace.ID, target, runID, text); err != nil {
		b.mu.Lock()
		delete(b.runs, runID)
		b.mu.Unlock()
		_, _ = b.sendPlain(ctx, target, "❌ "+err.Error())
		return
	}

	// Surface queued-behind-busy semantics.
	info, err := b.opts.Client.GetAgentSessionInfo(ctx, b.opts.Workspace.ID, target)
	if err == nil && info.IsBusy {
		// If already busy before our send, the prompt was queued.
		// We can't perfectly distinguish "just started our run" vs
		// "queued", but a positive queued count is a strong signal.
		queued, qerr := b.opts.Client.GetAgentSessionQueuedPrompts(ctx, b.opts.Workspace.ID, target)
		if qerr == nil && queued > 0 {
			_, _ = b.sendPlain(ctx, target, "⏳ queued behind the current run")
		}
	}
}

func (b *bridge) handleCallback(ctx context.Context, cq *CallbackQuery) {
	chatID := int64(0)
	if cq.Message != nil {
		chatID = cq.Message.Chat.ID
	}
	if chatID != b.opts.ChatID {
		slog.Info("Rejected callback from unauthorized chat", "chat_id", chatID)
		_ = b.tg.answerCallbackQuery(ctx, cq.ID, "")
		return
	}

	parts := strings.Split(cq.Data, ":")
	if len(parts) != 3 || parts[0] != "perm" {
		_ = b.tg.answerCallbackQuery(ctx, cq.ID, "unknown action")
		return
	}
	permID := parts[1]
	actionStr := parts[2]

	var action proto.PermissionAction
	var toast string
	var suffix string
	switch actionStr {
	case "allow":
		action = proto.PermissionAllow
		toast = "allowed"
		suffix = "\n\n✅ allowed"
	case "allow_session":
		action = proto.PermissionAllowForSession
		toast = "allowed for session"
		suffix = "\n\n✅ allowed for session"
	case "deny":
		action = proto.PermissionDeny
		toast = "denied"
		suffix = "\n\n🚫 denied"
	default:
		_ = b.tg.answerCallbackQuery(ctx, cq.ID, "unknown action")
		return
	}

	b.mu.Lock()
	pending, ok := b.pendPerms[permID]
	b.mu.Unlock()
	if !ok {
		_ = b.tg.answerCallbackQuery(ctx, cq.ID, "already handled")
		return
	}

	resolved, err := b.opts.Client.GrantPermission(ctx, b.opts.Workspace.ID, proto.PermissionGrant{
		Permission: pending.req,
		Action:     action,
	})
	if err != nil {
		_ = b.tg.answerCallbackQuery(ctx, cq.ID, "error")
		_, _ = b.sendPlain(ctx, pending.req.SessionID, "❌ permission grant failed: "+err.Error())
		return
	}

	b.mu.Lock()
	delete(b.pendPerms, permID)
	b.mu.Unlock()

	if !resolved {
		_ = b.tg.editMessageText(ctx, b.opts.ChatID, pending.msgID, "↩️ already handled elsewhere", false)
		_ = b.tg.answerCallbackQuery(ctx, cq.ID, "already handled")
		return
	}

	// Rebuild the original summary and append the decision. We re-render
	// rather than keeping the original text so we don't need to store it.
	newText := PermissionSummary(pending.req) + suffix
	// Prefix session tag if needed (editMessageText has no auto-tag).
	prefix := b.tagPrefix(pending.req.SessionID)
	_ = b.tg.editMessageText(ctx, b.opts.ChatID, pending.msgID, prefix+newText, true)
	_ = b.tg.answerCallbackQuery(ctx, cq.ID, toast)
}

// --- event pump (crush → Telegram) ---

func (b *bridge) eventPump(ctx context.Context) {
	backoff := time.Second
	const maxBackoff = 30 * time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		evc, err := b.opts.Client.SubscribeEvents(ctx, b.opts.Workspace.ID)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Warn("SubscribeEvents failed", "error", err)
			b.notifyReconnect(ctx)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff = min(backoff*2, maxBackoff)
			continue
		}
		if b.reconnectNotify {
			_, _ = b.sendPlain(ctx, b.activeSession(), "🟢 reconnected to server")
			b.reconnectNotify = false
		}
		backoff = time.Second
		b.resyncAfterReconnect(ctx)

		for ev := range evc {
			b.handleEvent(ctx, ev)
		}
		if ctx.Err() != nil {
			return
		}
		slog.Warn("Event stream closed; reconnecting")
		b.notifyReconnect(ctx)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, maxBackoff)
	}
}

func (b *bridge) notifyReconnect(ctx context.Context) {
	if b.reconnectNotify {
		return
	}
	b.reconnectNotify = true
	_, _ = b.sendPlain(ctx, b.activeSession(), "⚠️ lost server connection, reconnecting…")
}

func (b *bridge) resyncAfterReconnect(ctx context.Context) {
	// Expire any pending permission keyboards; SSE has no replay so
	// they are almost certainly stale.
	b.mu.Lock()
	pending := make([]pendingPerm, 0, len(b.pendPerms))
	for id, p := range b.pendPerms {
		pending = append(pending, p)
		delete(b.pendPerms, id)
	}
	b.mu.Unlock()
	for _, p := range pending {
		_ = b.tg.editMessageText(ctx, b.opts.ChatID, p.msgID, "⌛ expired", false)
	}
	// Refresh session titles.
	if sessions, err := b.opts.Client.ListSessions(ctx, b.opts.Workspace.ID); err == nil {
		b.refreshTitles(sessions)
	}
}

func (b *bridge) handleEvent(ctx context.Context, ev any) {
	switch e := ev.(type) {
	case pubsub.Event[proto.PermissionRequest]:
		if e.Type != pubsub.CreatedEvent {
			return
		}
		if !b.knownSession(e.Payload.SessionID) {
			return
		}
		b.showPermissionKeyboard(ctx, e.Payload)
	case pubsub.Event[proto.PermissionNotification]:
		b.resolvePermByToolCall(ctx, e.Payload)
	case pubsub.Event[proto.RunComplete]:
		b.handleRunComplete(ctx, e.Payload)
	case pubsub.Event[proto.AgentEvent]:
		if e.Payload.Error != nil {
			b.reportAgentError(ctx, e.Payload)
		}
	case pubsub.Event[proto.Session]:
		b.trackSession(e)
	default:
		// Ignore message/lsp/mcp/etc. in v1.
	}
}

func (b *bridge) showPermissionKeyboard(ctx context.Context, req proto.PermissionRequest) {
	text := PermissionSummary(req)
	kb := &InlineKeyboardMarkup{
		InlineKeyboard: [][]InlineKeyboardButton{
			{
				{Text: "✅ Allow", CallbackData: "perm:" + req.ID + ":allow"},
				{Text: "✅ For session", CallbackData: "perm:" + req.ID + ":allow_session"},
				{Text: "🚫 Deny", CallbackData: "perm:" + req.ID + ":deny"},
			},
		},
	}
	msg, err := b.sendHTML(ctx, req.SessionID, text, kb)
	if err != nil {
		slog.Warn("Failed to send permission keyboard", "error", err)
		return
	}
	b.mu.Lock()
	b.pendPerms[req.ID] = pendingPerm{req: req, msgID: msg.MessageID}
	b.mu.Unlock()
}

func (b *bridge) resolvePermByToolCall(ctx context.Context, n proto.PermissionNotification) {
	b.mu.Lock()
	var foundID string
	var found pendingPerm
	for id, p := range b.pendPerms {
		if p.req.ToolCallID == n.ToolCallID {
			foundID = id
			found = p
			break
		}
	}
	if foundID != "" {
		delete(b.pendPerms, foundID)
	}
	b.mu.Unlock()
	if foundID == "" {
		return
	}
	status := "↩️ already handled elsewhere"
	if n.Granted {
		status = "✅ allowed"
	} else if n.Denied {
		status = "🚫 denied"
	}
	newText := PermissionSummary(found.req) + "\n\n" + status
	prefix := b.tagPrefix(found.req.SessionID)
	_ = b.tg.editMessageText(ctx, b.opts.ChatID, found.msgID, prefix+newText, true)
}

func (b *bridge) handleRunComplete(ctx context.Context, rc proto.RunComplete) {
	b.mu.Lock()
	sessID, owned := b.runs[rc.RunID]
	if owned {
		delete(b.runs, rc.RunID)
	} else if rc.RunID == "" && rc.SessionID == b.sessionID {
		// Fallback for untagged completions on the active session.
		sessID = rc.SessionID
		owned = true
	} else if rc.RunID != "" {
		// Not our run.
		b.mu.Unlock()
		return
	} else if b.knownSessionLocked(rc.SessionID) {
		// Untagged completion for a known session we care about.
		sessID = rc.SessionID
		owned = true
	}
	// Expire pending perms for this session.
	var expired []pendingPerm
	if owned {
		for id, p := range b.pendPerms {
			if p.req.SessionID == sessID {
				expired = append(expired, p)
				delete(b.pendPerms, id)
			}
		}
	}
	b.mu.Unlock()

	if !owned {
		return
	}
	for _, p := range expired {
		_ = b.tg.editMessageText(ctx, b.opts.ChatID, p.msgID, "⌛ expired", false)
	}

	switch {
	case rc.Error != "" && !rc.Cancelled:
		_, _ = b.sendPlain(ctx, sessID, "❌ run failed: "+rc.Error)
	case rc.Cancelled:
		_, _ = b.sendPlain(ctx, sessID, "🛑 run cancelled.")
	case rc.Text == "":
		_, _ = b.sendPlain(ctx, sessID, "✅ done (no text output).")
	default:
		_, _ = b.sendPlain(ctx, sessID, rc.Text)
	}
}

func (b *bridge) knownSessionLocked(id string) bool {
	_, ok := b.titles[id]
	return ok
}

func (b *bridge) reportAgentError(ctx context.Context, ae proto.AgentEvent) {
	// Prefer RunID correlation; fall back to session.
	sessID := ae.SessionID
	b.mu.Lock()
	if ae.RunID != "" {
		if s, ok := b.runs[ae.RunID]; ok {
			sessID = s
			delete(b.runs, ae.RunID)
		}
	}
	if sessID == "" {
		sessID = b.sessionID
	}
	b.mu.Unlock()
	_, _ = b.sendPlain(ctx, sessID, "❌ agent error: "+ae.Error.Error())
}

func (b *bridge) trackSession(e pubsub.Event[proto.Session]) {
	s := e.Payload
	if !isTopLevelSession(s) {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	switch e.Type {
	case pubsub.DeletedEvent:
		delete(b.titles, s.ID)
	default:
		b.titles[s.ID] = s.Title
	}
}

func (b *bridge) typingLoop(ctx context.Context) {
	ticker := time.NewTicker(4 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			b.mu.Lock()
			busy := len(b.runs) > 0
			b.mu.Unlock()
			if busy {
				_ = b.tg.sendChatAction(ctx, b.opts.ChatID, "typing")
			}
		}
	}
}
