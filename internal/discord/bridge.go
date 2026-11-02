package discord

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/workspace"
)

const (
	// defaultTurnTick paces live status edits (animation + elapsed
	// timer). Discord allows roughly five edits per five seconds per
	// channel; 1.5s stays well inside that.
	defaultTurnTick = 1500 * time.Millisecond

	// defaultEditThrottle is the minimum gap between event-driven edits.
	defaultEditThrottle = 1400 * time.Millisecond

	// typingInterval is how often the typing indicator refreshes while
	// the agent is busy.
	typingInterval = 9 * time.Second

	// defaultCommandTimeout bounds how long a command waits for the
	// TUI to answer. The TUI runs on the same event loop as the
	// session, so a short wait keeps a stalled frontend from blocking
	// the Discord input loop.
	defaultCommandTimeout = 5 * time.Second

	// maxAttachmentBytes caps mirrored Discord attachments.
	maxAttachmentBytes = 8 << 20

	// countedResultLimit bounds the failed tool-result dedupe set.
	countedResultLimit = 256

	// frozenLimit bounds remembered frozen assistant steps.
	frozenLimit = 128

	// completedLimit bounds remembered completed message ids.
	completedLimit = 128
)

// farewellText is posted when the bridge stops.
const farewellText = "👋 **Remote control ended** — the session continues in the terminal."

// discordAPI is the Discord surface the bridge drives. restClient
// implements it; tests inject a fake.
type discordAPI interface {
	createThread(ctx context.Context, channelID, name string) (createdThread, error)
	createMessage(ctx context.Context, channelID, content string, components ...discordComponent) (string, error)
	editMessage(ctx context.Context, channelID, messageID, content string, components ...discordComponent) error
	deleteMessage(ctx context.Context, channelID, messageID string) error
	triggerTyping(ctx context.Context, channelID string) error
	addReaction(ctx context.Context, channelID, messageID, emoji string) error
	editChannelName(ctx context.Context, channelID, name string) error
	gatewayURL(ctx context.Context) (string, error)
	respondInteraction(ctx context.Context, interactionID, token, content string, ephemeral bool) error
	updateInteraction(ctx context.Context, interactionID, token, content string, components []discordComponent) error
	respondModal(ctx context.Context, interactionID, token string, modal discordModal) error
	ensureApplicationCommands(ctx context.Context, guildID string, cmds []applicationCommand) error
}

// Bridge mirrors a live Crush session into a Discord thread and injects
// thread messages back into the session. It owns one control thread and
// one gateway connection, and is safe to Stop more than once.
type Bridge struct {
	env commandEnv
	app *app.App
	// sessionID is the session the bridge mirrors. Guarded by sessMu so a
	// /clear rebind cannot race the event loop; read it through
	// currentSessionID.
	sessionID string
	sessMu    sync.RWMutex
	cfg       Config

	api            discordAPI
	interactionAPI discordAPI
	gateway        *gateway
	http           *http.Client
	gwLoop         func(ctx context.Context)
	events         <-chan pubsub.Event[tea.Msg]
	input          chan inputEvent
	interactions   chan gatewayInteraction
	now            func() time.Time
	editRate       time.Duration
	tick           time.Duration
	typing         time.Duration

	// sendEvent publishes a message to the TUI. Nil when the bridge
	// runs without a TUI (tests).
	sendEvent func(tea.Msg)
	// commandTimeout bounds commands that need a TUI reply.
	commandTimeout time.Duration

	ctx      context.Context
	cancel   context.CancelFunc
	stopOnce sync.Once

	threadID  string
	threadURL string

	modeMu sync.Mutex
	mode   string // "code" or "plan"

	pendingMu       sync.Mutex
	permissions     map[string]permission.PermissionRequest
	permissionOrder []string
	permMessage     map[string]string
	questions       map[string]question.Request
	questionOrder   []string
	// questionMessages holds, per question (in order), the message ids of
	// that question's card. A long card spans several messages so nothing
	// is truncated; the buttons ride on the last one, directly under the
	// question. The submit row rides on the last question's card.
	questionMessages map[string][][]string
	// questionSelections tracks in-progress button answers per batch:
	// question id -> selected choice ids. Cleared when the batch resolves.
	questionSelections map[string]map[string]map[string]bool
	// questionFillIns holds typed "Something else" answers per batch:
	// question id -> text. Cleared when the batch resolves.
	questionFillIns map[string]map[string]string

	subMu       sync.Mutex
	submissions []promptSubmission

	countedMu      sync.Mutex
	countedResults map[string]bool

	frozenMu sync.Mutex
	frozen   map[string]bool

	completedMu sync.Mutex
	completed   map[string]bool

	// compacted tracks summary messages already announced in the thread so
	// streaming updates cannot post the card twice.
	compactedMu sync.Mutex
	compacted   map[string]bool

	renameMu  sync.Mutex
	renamedTo string

	// btwMu guards the last side-question card so bare /btw can repeat it.
	btwMu   sync.Mutex
	lastBTW string

	lastMu        sync.Mutex
	lastAssistant message.Message

	// liveMu guards the messages mirroring the current step's streamed
	// text. The text lives in the transcript, not in the footer, so a
	// long answer is never cut to a tail window.
	liveMu       sync.Mutex
	liveIDs      []string
	liveContents []string
	liveUpdated  time.Time

	footer *footer
}

// promptSubmission records a prompt injected from Discord so the
// resulting user-message echo can be skipped (the author's own message
// is already in the thread).
type promptSubmission struct {
	hash string
	when time.Time
}

// bridgeDeps are the injected collaborators of a bridge; production
// Start fills them with real implementations, tests with fakes.
type bridgeDeps struct {
	api            discordAPI
	interactionAPI discordAPI
	events         <-chan pubsub.Event[tea.Msg]
	gatewayStart   func(ctx context.Context)
	now            func() time.Time
	tick           time.Duration
	editThrottle   time.Duration
	http           *http.Client
	typing         time.Duration
	sendEvent      func(tea.Msg)
	commandTimeout time.Duration
}

// Start creates the Discord control thread for a live session and
// launches the bridge. mode is the TUI's current input mode ("code" or
// "plan"). Only local-mode workspaces are supported.
func Start(ctx context.Context, ws workspace.Workspace, sessionID, mode string) (*Bridge, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	appWS, ok := ws.(*workspace.AppWorkspace)
	if !ok {
		return nil, errors.New("remote control requires local mode (unset CRUSH_CLIENT_SERVER)")
	}
	return start(ctx, appWS, appWS.App(), sessionID, cfg, mode)
}

// start wires and launches a bridge, creating its control thread.
func start(ctx context.Context, env commandEnv, application *app.App, sessionID string, cfg Config, mode string) (*Bridge, error) {
	b := newBridge(env, application, sessionID, cfg, mode, bridgeDeps{
		api:            newRESTClient(cfg.Token),
		interactionAPI: newRESTClient(cfg.Token),
		http:           &http.Client{Timeout: 30 * time.Second},
	})
	sess, err := env.GetSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("read session: %w", err)
	}
	b.setSession(sess)
	thread, err := b.api.createThread(ctx, cfg.ChannelIDs[0], b.threadName(ctx))
	if err != nil {
		return nil, fmt.Errorf("create Discord thread: %w", err)
	}
	b.threadID = thread.ID
	b.threadURL = threadURL(thread)
	if err := b.launch(ctx); err != nil {
		return nil, err
	}
	b.post(b.bannerText(ctx))
	b.flushFooter()
	go func() {
		if err := b.api.ensureApplicationCommands(context.Background(), thread.GuildID, applicationCommands(b.env.Config())); err != nil {
			slog.Warn("Discord bridge: slash command registration failed", "error", err.Error())
		}
	}()
	return b, nil
}

// newBridge builds an unlaunched bridge from injected dependencies.
func newBridge(env commandEnv, application *app.App, sessionID string, cfg Config, mode string, deps bridgeDeps) *Bridge {
	b := &Bridge{
		env:                env,
		app:                application,
		sessionID:          sessionID,
		cfg:                cfg,
		api:                deps.api,
		http:               deps.http,
		events:             deps.events,
		gwLoop:             deps.gatewayStart,
		now:                deps.now,
		tick:               deps.tick,
		editRate:           deps.editThrottle,
		typing:             deps.typing,
		sendEvent:          deps.sendEvent,
		commandTimeout:     deps.commandTimeout,
		mode:               mode,
		permissions:        map[string]permission.PermissionRequest{},
		permMessage:        map[string]string{},
		questions:          map[string]question.Request{},
		questionMessages:   map[string][][]string{},
		questionSelections: map[string]map[string]map[string]bool{},
		questionFillIns:    map[string]map[string]string{},
		countedResults:     map[string]bool{},
		frozen:             map[string]bool{},
		completed:          map[string]bool{},
		footer: &footer{
			state:        turnIdle,
			toolCounts:   map[string]int{},
			countedCalls: map[string]bool{},
			updates:      make(chan struct{}, 1),
		},
	}
	if b.api == nil {
		b.api = newRESTClient(cfg.Token)
	}
	if deps.interactionAPI != nil {
		b.interactionAPI = deps.interactionAPI
	} else {
		b.interactionAPI = b.api
	}
	if b.http == nil {
		b.http = &http.Client{Timeout: 30 * time.Second}
	}
	if b.now == nil {
		b.now = time.Now
	}
	if b.tick <= 0 {
		b.tick = defaultTurnTick
	}
	if b.editRate <= 0 {
		b.editRate = defaultEditThrottle
	}
	if b.typing <= 0 {
		b.typing = typingInterval
	}
	if b.sendEvent == nil && b.app != nil {
		b.sendEvent = b.app.SendEvent
	}
	if b.commandTimeout <= 0 {
		b.commandTimeout = defaultCommandTimeout
	}
	b.gateway = newGateway(cfg.Token, defaultGatewayIntents)
	return b
}

// launch starts the bridge's goroutines. The caller must have set
// threadID.
func (b *Bridge) launch(ctx context.Context) error {
	b.ctx, b.cancel = context.WithCancel(ctx)
	if b.events == nil && b.app != nil {
		b.events = b.app.Events(b.ctx)
	}
	if b.events == nil {
		b.events = make(chan pubsub.Event[tea.Msg])
	}
	b.input = make(chan inputEvent, 32)
	b.interactions = make(chan gatewayInteraction, 32)
	go b.eventLoop()
	go b.inputLoop()
	go b.interactionLoop()
	go b.typingLoop()
	go b.footerLoop()
	if b.gwLoop != nil {
		go b.gwLoop(b.ctx)
	} else {
		go b.runGateway(b.ctx)
	}
	return nil
}

// Stop shuts the bridge down, posting a farewell first so the thread is
// not left dangling. Safe to call multiple times and from any goroutine;
// the farewell is bounded so quitting never hangs. reason names what ended
// the bridge and is logged, so an unexpected stop in the field can be
// attributed without guessing.
func (b *Bridge) Stop(reason string) {
	b.stopOnce.Do(func() {
		slog.Info("Discord bridge stopped", "reason", reason, "thread", b.threadURL)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		f := b.footer
		f.mu.Lock()
		f.stopped = true
		footerID := f.messageID
		f.messageID = ""
		f.mu.Unlock()
		if b.threadID != "" {
			if footerID != "" {
				if err := b.api.deleteMessage(ctx, b.threadID, footerID); err != nil {
					slog.Debug("Discord bridge: failed to delete status footer", "error", err.Error())
				}
			}
			_, _ = b.api.createMessage(ctx, b.threadID, farewellText)
		}
		if b.cancel != nil {
			b.cancel()
		}
		if b.app != nil {
			// Let the TUI clear its reference when the stop was initiated
			// from Discord (e.g. "/rc off"); self-initiated stops have
			// already cleared it and ignore this.
			b.app.SendEvent(StoppedMsg{Bridge: b})
		}
	})
}

// ThreadURL returns the Discord link of the control thread.
func (b *Bridge) ThreadURL() string { return b.threadURL }

// currentSessionID returns the session the bridge mirrors. Rebinds happen
// under the same lock, so callers always observe one consistent session.
func (b *Bridge) currentSessionID() string {
	b.sessMu.RLock()
	defer b.sessMu.RUnlock()
	return b.sessionID
}

// Mode reports the bridge's view of the input mode.
func (b *Bridge) modeName() string {
	b.modeMu.Lock()
	defer b.modeMu.Unlock()
	return b.mode
}

// SetMode records a local mode change so /status stays accurate.
func (b *Bridge) SetMode(plan bool) {
	b.modeMu.Lock()
	defer b.modeMu.Unlock()
	if plan {
		b.mode = "plan"
	} else {
		b.mode = "code"
	}
}

// setMode switches the session's agent from Discord and notifies the
// TUI so its badge and caches track the change.
func (b *Bridge) setMode(ctx context.Context, plan bool) error {
	agentID := config.AgentCoder
	if plan {
		agentID = config.AgentPlan
	}
	if err := b.env.AgentSetMain(agentID); err != nil {
		return err
	}
	if err := b.env.UpdateAgentModel(ctx); err != nil {
		return err
	}
	b.SetMode(plan)
	if b.app != nil {
		b.app.SendEvent(ModeChangedMsg{Plan: plan})
	}
	return nil
}

// setYolo toggles permission auto-approval and notifies the TUI.
func (b *Bridge) setYolo(on bool) {
	b.env.PermissionSetSkipRequests(on)
	if b.app != nil {
		b.app.SendEvent(YoloChangedMsg{On: on})
	}
}

// changeModel switches the preferred large model and rebuilds the agent
// models, then notifies the TUI.
func (b *Bridge) changeModel(ctx context.Context, sel config.SelectedModel) error {
	if err := b.env.UpdatePreferredModel(config.ScopeGlobal, config.SelectedModelTypeLarge, sel); err != nil {
		return err
	}
	if err := b.env.UpdateAgentModel(ctx); err != nil {
		return err
	}
	if b.app != nil {
		b.app.SendEvent(ModelChangedMsg{Provider: sel.Provider, Model: sel.Model})
	}
	return nil
}

// post sends one or more chunked messages to the thread, above the
// status footer. Failures are logged only: the session must never break
// because Discord is down.
func (b *Bridge) post(content string) {
	b.postAbove(content)
}

// postedMessages is the result of posting chunked content: every chunk's
// message id in order, plus the id of the chunk carrying the components
// (the last chunk, since Discord renders controls after the content).
type postedMessages struct {
	ids         []string
	componentID string
}

// firstID returns the first chunk's id, or "" when nothing was posted.
func (p postedMessages) firstID() string {
	if len(p.ids) == 0 {
		return ""
	}
	return p.ids[0]
}

// postAbove posts content above the footer and returns the id of the
// first chunk ("" when nothing was posted). When a footer message
// exists it is edited into the first chunk and re-created after the
// content, so the footer stays the thread's last message.
func (b *Bridge) postAbove(content string) string {
	return b.postAboveWithComponents(content, nil).firstID()
}

// postAboveWithComponents posts content with interactive components above
// the footer, mirroring postAbove's footer recycling.
func (b *Bridge) postAboveWithComponents(content string, components []discordComponent) postedMessages {
	var posted postedMessages
	chunks := splitMessage(content)
	if len(chunks) == 0 {
		return posted
	}
	f := b.footer
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.stopped {
		return posted
	}

	// Discord renders components after the message content, so when a long
	// card is split into several messages the controls must ride on the
	// LAST chunk. Attaching them to the first chunk strands the buttons in
	// the middle of the form, above the rest of its text.
	lastChunk := len(chunks) - 1
	restStart := 0
	if f.messageID != "" {
		var comps []discordComponent
		if lastChunk == 0 {
			comps = components
		}
		if err := b.api.editMessage(b.ctx, b.threadID, f.messageID, chunks[0], comps...); err == nil {
			posted.ids = append(posted.ids, f.messageID)
			if len(comps) > 0 {
				posted.componentID = f.messageID
			}
			restStart = 1
		} else {
			slog.Debug("Discord bridge: failed to reuse footer for message", "error", err.Error())
		}
		f.messageID = ""
		f.lastBody = ""
	}
	for i := restStart; i < len(chunks); i++ {
		chunk := chunks[i]
		var comps []discordComponent
		if i == lastChunk {
			comps = components
		}
		id, err := b.api.createMessage(b.ctx, b.threadID, chunk, comps...)
		if err != nil {
			slog.Debug("Discord bridge: failed to post message", "error", err.Error())
			break
		}
		posted.ids = append(posted.ids, id)
		if len(comps) > 0 {
			posted.componentID = id
		}
	}
	b.attachFooterLocked()
	return posted
}

// note surfaces a bridge notice in the TUI (when hosted in-process) and
// the log.
func (b *Bridge) note(text string, isErr bool) {
	if isErr {
		slog.Warn("Discord bridge: " + text)
	} else {
		slog.Info("Discord bridge: " + text)
	}
	if b.app != nil {
		b.app.SendEvent(StatusNoteMsg{Text: text, IsErr: isErr})
	}
}

// threadName names the control thread after the session or the working
// directory.
func (b *Bridge) threadName(ctx context.Context) string {
	if sess, err := b.env.GetSession(ctx, b.currentSessionID()); err == nil {
		if title := strings.TrimSpace(sess.Title); title != "" {
			return truncateRunes(title, 100)
		}
	}
	dir := filepath.Base(b.env.WorkingDir())
	if dir == "" || dir == "." || dir == string(filepath.Separator) {
		dir = "session"
	}
	return truncateRunes("crush — "+dir, 100)
}

// bannerText announces the bridge in the fresh thread.
func (b *Bridge) bannerText(ctx context.Context) string {
	title := b.threadName(ctx)
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s **Crush remote control enabled**\n", iconBot)
	fmt.Fprintf(&sb, "-# %s · mode `%s` · yolo `%s`\n", title, b.modeName(), onOff(b.env.PermissionSkipRequests()))
	sb.WriteString("-# type a prompt, or `/help` for commands · `/rc off` to end")
	return sb.String()
}

// threadURL builds the clickable link for a created thread.
func threadURL(t createdThread) string {
	if t.GuildID == "" {
		return "https://discord.com/channels/@me/" + t.ID
	}
	return "https://discord.com/channels/" + t.GuildID + "/" + t.ID
}

// ── Event mirroring ──────────────────────────────────────────────────────

// eventLoop mirrors session events into the thread until the bridge
// context ends.
func (b *Bridge) eventLoop() {
	for {
		select {
		case <-b.ctx.Done():
			return
		case ev, ok := <-b.events:
			if !ok {
				return
			}
			b.handleEvent(ev)
		}
	}
}

// handleEvent routes one broker event.
func (b *Bridge) handleEvent(ev pubsub.Event[tea.Msg]) {
	switch p := ev.Payload.(type) {
	case pubsub.Event[message.Message]:
		b.handleMessageEvent(p)
	case pubsub.Event[notify.RunComplete]:
		b.handleRunComplete(p.Payload)
	case pubsub.Event[notify.Notification]:
		b.handleAgentNotification(p.Payload)
	case pubsub.Event[permission.PermissionRequest]:
		b.handlePermissionRequest(p.Payload)
	case pubsub.Event[permission.PermissionNotification]:
		b.handlePermissionNotification(p.Payload)
	case pubsub.Event[question.Request]:
		b.handleQuestionRequest(p.Payload)
	case pubsub.Event[question.Notification]:
		b.handleQuestionNotification(p.Payload)
	case pubsub.Event[session.Session]:
		b.handleSessionEvent(p)
	}
}

func (b *Bridge) handleMessageEvent(ev pubsub.Event[message.Message]) {
	msg := ev.Payload
	if msg.SessionID != b.currentSessionID() {
		return
	}
	if msg.IsSummaryMessage {
		b.handleCompactionMessage(msg)
		return
	}
	switch msg.Role {
	case message.User:
		if ev.Type == pubsub.CreatedEvent {
			b.handleUserMessage(msg)
		}
	case message.Assistant:
		if ev.Type == pubsub.CreatedEvent || ev.Type == pubsub.UpdatedEvent {
			b.handleAssistantMessage(msg)
		}
	case message.Tool:
		if ev.Type == pubsub.CreatedEvent || ev.Type == pubsub.UpdatedEvent {
			b.handleToolResult(msg)
		}
	}
}

// handleCompactionMessage announces a session compaction in the thread.
// The summary text is model working state, not conversation, so only the
// measured outcome is posted, once per compaction: the summary message
// streams through several updates before its stats are attached. Without
// this the thread stayed silent while the context was rewritten, which read
// as "it only summarized".
func (b *Bridge) handleCompactionMessage(msg message.Message) {
	if !msg.IsFinished() {
		return
	}
	stats, ok := msg.CompactionStatsPart()
	if !ok {
		return
	}

	b.compactedMu.Lock()
	if b.compacted == nil {
		b.compacted = map[string]bool{}
	}
	announced := b.compacted[msg.ID]
	b.compacted[msg.ID] = true
	b.compactedMu.Unlock()
	if announced {
		return
	}

	after := stats.SummaryTokens + stats.TailTokens
	b.post(fmt.Sprintf(
		"%s **Context compacted**: %s → %s tokens · %s saved · %d messages kept",
		iconBrain,
		formatTokens(stats.TokensBefore),
		formatTokens(after),
		formatTokens(stats.SavedTokens),
		stats.KeptMessages,
	))
}

// handleUserMessage echoes prompts typed in the terminal so remote
// readers see both sides of the conversation. Prompts injected from
// Discord are skipped: the author's own message is already visible.
func (b *Bridge) handleUserMessage(msg message.Message) {
	text := strings.TrimSpace(messageText(msg))
	if text == "" || strings.HasPrefix(text, "<channel") {
		return
	}
	if b.consumeSubmission(text) {
		return
	}
	b.post(iconLocal + " **(terminal)**\n" + text)
}

// handleAssistantMessage advances the status footer from a message
// snapshot and freezes a completed step's transcript above it.
func (b *Bridge) handleAssistantMessage(msg message.Message) {
	b.completedMu.Lock()
	done := b.completed[msg.ID]
	b.completedMu.Unlock()
	if done {
		return
	}
	if b.isFrozen(msg.ID) {
		return
	}
	b.setLastAssistant(msg)

	waiting := b.hasPendingPermission()
	f := b.footer
	f.mu.Lock()
	b.applyTurnLocked(msg, waiting)
	f.mu.Unlock()

	if hasFinishPart(msg) {
		b.freezeStep(msg)
	} else {
		b.syncLiveTranscriptThrottled(msg)
	}
	b.pokeFooter()
}

// syncLiveTranscriptThrottled mirrors a streaming step's text into the
// thread at the same cadence as footer edits. Both run on the event
// loop, so a freeze can never race a sync.
func (b *Bridge) syncLiveTranscriptThrottled(msg message.Message) {
	b.liveMu.Lock()
	due := b.now().Sub(b.liveUpdated) >= b.editRate
	b.liveMu.Unlock()
	if !due {
		return
	}
	content := renderAssistantTimeline(msg)
	if strings.TrimSpace(content) == "" {
		return
	}
	b.syncLiveTranscript(content)
	b.liveMu.Lock()
	b.liveUpdated = b.now()
	b.liveMu.Unlock()
}

// syncLiveTranscript mirrors a step's text into transcript messages above
// the footer. Existing messages are edited in place and new ones are
// appended when the text grows past Discord's per-message limit, so the
// whole answer is visible instead of a truncated tail.
func (b *Bridge) syncLiveTranscript(content string) {
	content = strings.TrimSpace(content)
	if content == "" {
		return
	}
	chunks := splitMessage(content)
	if len(chunks) == 0 {
		return
	}

	b.liveMu.Lock()
	defer b.liveMu.Unlock()

	grew := false
	for i, chunk := range chunks {
		if i >= len(b.liveIDs) {
			id, err := b.api.createMessage(b.ctx, b.threadID, chunk)
			if err != nil {
				slog.Debug("Discord bridge: failed to post streamed text", "error", err.Error())
				break
			}
			b.liveIDs = append(b.liveIDs, id)
			b.liveContents = append(b.liveContents, chunk)
			grew = true
			continue
		}
		if b.liveContents[i] == chunk {
			continue
		}
		if err := b.api.editMessage(b.ctx, b.threadID, b.liveIDs[i], chunk); err != nil {
			slog.Debug("Discord bridge: failed to edit streamed text", "error", err.Error())
			continue
		}
		b.liveContents[i] = chunk
	}
	if len(b.liveIDs) > len(chunks) {
		for i := len(chunks); i < len(b.liveIDs); i++ {
			if err := b.api.deleteMessage(b.ctx, b.threadID, b.liveIDs[i]); err != nil {
				slog.Debug("Discord bridge: failed to drop stale streamed text", "error", err.Error())
			}
		}
		b.liveIDs = b.liveIDs[:len(chunks)]
		b.liveContents = b.liveContents[:len(chunks)]
	}
	if grew {
		// New chunks landed below the footer; move it back down.
		b.reanchorFooter()
	}
}

// resetLiveTranscript forgets the current step's mirrored messages. A
// later step starts a fresh set of messages.
func (b *Bridge) resetLiveTranscript() {
	b.liveMu.Lock()
	b.liveIDs = nil
	b.liveContents = nil
	b.liveUpdated = time.Time{}
	b.liveMu.Unlock()
}

// handleToolResult tallies failed tool calls for the footer. Results
// are never mirrored: the thread carries only model responses, and
// tool activity appears as counts in the status footer.
func (b *Bridge) handleToolResult(msg message.Message) {
	tr, ok := firstToolResult(msg)
	if !ok || !tr.IsError {
		return
	}
	b.countedMu.Lock()
	if b.countedResults[msg.ID] {
		b.countedMu.Unlock()
		return
	}
	if len(b.countedResults) >= countedResultLimit {
		clear(b.countedResults)
	}
	b.countedResults[msg.ID] = true
	b.countedMu.Unlock()

	f := b.footer
	f.mu.Lock()
	f.toolFailures++
	f.mu.Unlock()
	b.pokeFooter()
}

// handleRunComplete freezes any unfrozen final-step transcript and
// parks the footer in its terminal state.
func (b *Bridge) handleRunComplete(rc notify.RunComplete) {
	if rc.SessionID != b.currentSessionID() {
		return
	}
	if rc.MessageID != "" {
		b.completedMu.Lock()
		if len(b.completed) >= completedLimit {
			clear(b.completed)
		}
		b.completed[rc.MessageID] = true
		b.completedMu.Unlock()
	}

	state := turnDone
	switch {
	case rc.Cancelled:
		state = turnCancelled
	case rc.Error != "":
		state = turnError
	}
	f := b.footer
	f.mu.Lock()
	if f.active {
		f.state = state
		f.finished = b.now()
		f.errText = rc.Error
		f.waiting = false
		f.tail = ""
		f.active = false
	}
	f.mu.Unlock()

	// A step's Finish event normally freezes its transcript; when it
	// never arrived (lossy events), freeze the last snapshot or the
	// authoritative run text here.
	last := b.lastAssistantMessage()
	content := strings.TrimSpace(renderAssistantTimeline(last))
	if content == "" {
		content = strings.TrimSpace(rc.Text)
	}
	b.freezeContent(last.ID, content)
	b.pokeFooter()
}

// handleAgentNotification mirrors re-authentication requests; run
// terminals are covered by RunComplete.
func (b *Bridge) handleAgentNotification(n notify.Notification) {
	if n.SessionID != b.currentSessionID() {
		return
	}
	if n.Type == notify.TypeReAuthenticate {
		msg := n.Message
		if msg == "" {
			msg = "provider authentication expired"
		}
		b.post("🔑 **Re-authentication needed** — " + compactLine(msg, 300))
	}
}

// handleSessionEvent tracks title generation by renaming the thread and
// keeps the footer's usage metrics current.
func (b *Bridge) handleSessionEvent(ev pubsub.Event[session.Session]) {
	if ev.Type != pubsub.UpdatedEvent || ev.Payload.ID != b.currentSessionID() {
		return
	}
	b.setSession(ev.Payload)

	title := strings.TrimSpace(ev.Payload.Title)
	if title == "" {
		return
	}
	b.renameMu.Lock()
	changed := b.renamedTo != title
	b.renamedTo = title
	b.renameMu.Unlock()
	if changed {
		if err := b.api.editChannelName(b.ctx, b.threadID, truncateRunes(title, 100)); err != nil {
			slog.Debug("Discord bridge: failed to rename thread", "error", err.Error())
		}
	}
}

// ── Permissions and questions ────────────────────────────────────────────

// handlePermissionRequest mirrors a tool permission request as a
// reaction-approvable card.
func (b *Bridge) handlePermissionRequest(req permission.PermissionRequest) {
	if req.SessionID != b.currentSessionID() {
		return
	}
	b.pendingMu.Lock()
	b.permissions[req.ID] = req
	b.permissionOrder = append(b.permissionOrder, req.ID)
	b.pendingMu.Unlock()

	id := b.postAbove(renderPermissionCard(req))
	if id == "" {
		slog.Debug("Discord bridge: failed to post permission card")
		return
	}
	b.pendingMu.Lock()
	b.permMessage[req.ID] = id
	b.pendingMu.Unlock()
	for _, emoji := range []string{reactionAllow, reactionAllowSession, reactionDeny} {
		if err := b.api.addReaction(b.ctx, b.threadID, id, emoji); err != nil {
			slog.Debug("Discord bridge: failed to seed reaction", "error", err.Error())
		}
	}
	b.markWaiting(true)
}

// handlePermissionNotification updates the card once a permission is
// resolved, from either side of the bridge.
func (b *Bridge) handlePermissionNotification(n permission.PermissionNotification) {
	b.pendingMu.Lock()
	var reqID string
	for id, req := range b.permissions {
		if req.ToolCallID == n.ToolCallID {
			reqID = id
			break
		}
	}
	if reqID == "" {
		b.pendingMu.Unlock()
		return
	}
	req := b.permissions[reqID]
	delete(b.permissions, reqID)
	msgID := b.permMessage[reqID]
	delete(b.permMessage, reqID)
	b.pendingMu.Unlock()

	status := "❌ denied"
	if n.Granted {
		status = "✅ approved"
	}
	if msgID != "" {
		card := renderPermissionCard(req) + "\n-# " + status
		if err := b.api.editMessage(b.ctx, b.threadID, msgID, card); err != nil {
			slog.Debug("Discord bridge: failed to update permission card", "error", err.Error())
		}
	}
	b.markWaiting(false)
}

// handleQuestionRequest mirrors an agent question as a thread card.
func (b *Bridge) handleQuestionRequest(req question.Request) {
	if req.SessionID != b.currentSessionID() {
		return
	}
	selections := newQuestionSelections(req)
	fillIns := newQuestionFillIns(req)
	b.pendingMu.Lock()
	b.questions[req.ID] = req
	b.questionOrder = append(b.questionOrder, req.ID)
	b.questionSelections[req.ID] = selections
	b.questionFillIns[req.ID] = fillIns
	b.pendingMu.Unlock()

	ids := make([][]string, 0, len(req.Questions))
	for i := range req.Questions {
		// One card per question keeps that question's controls directly
		// under it. The submit row rides on the final question's card.
		includeSubmit := i == len(req.Questions)-1
		rows := questionRows(req, i, selections, fillIns, includeSubmit)
		posted := b.postAboveWithComponents(renderQuestionContent(req, i, questionHintButtons), rows)
		if len(posted.ids) == 0 {
			slog.Debug("Discord bridge: failed to post question card")
			b.pendingMu.Lock()
			delete(b.questions, req.ID)
			delete(b.questionSelections, req.ID)
			delete(b.questionFillIns, req.ID)
			b.pendingMu.Unlock()
			return
		}
		ids = append(ids, posted.ids)
	}
	b.pendingMu.Lock()
	b.questionMessages[req.ID] = ids
	b.pendingMu.Unlock()
}

// handleQuestionNotification marks the question card resolved.
func (b *Bridge) handleQuestionNotification(n question.Notification) {
	b.pendingMu.Lock()
	req, ok := b.questions[n.BatchID]
	if !ok {
		b.pendingMu.Unlock()
		return
	}
	delete(b.questions, n.BatchID)
	delete(b.questionSelections, n.BatchID)
	delete(b.questionFillIns, n.BatchID)
	msgIDs := b.questionMessages[n.BatchID]
	delete(b.questionMessages, n.BatchID)
	b.pendingMu.Unlock()

	for i, ids := range msgIDs {
		b.editAnsweredCard(ids, req, i)
	}
}

// hasPendingPermission reports whether any permission card is open.
func (b *Bridge) hasPendingPermission() bool {
	b.pendingMu.Lock()
	defer b.pendingMu.Unlock()
	return len(b.permissions) > 0
}

// pendingPermissions returns unresolved requests in arrival order.
func (b *Bridge) pendingPermissions() []permission.PermissionRequest {
	b.pendingMu.Lock()
	defer b.pendingMu.Unlock()
	out := make([]permission.PermissionRequest, 0, len(b.permissions))
	for _, id := range b.permissionOrder {
		if req, ok := b.permissions[id]; ok {
			out = append(out, req)
		}
	}
	return out
}

// pendingQuestion returns the newest unresolved question request.
func (b *Bridge) pendingQuestion() (question.Request, bool) {
	b.pendingMu.Lock()
	defer b.pendingMu.Unlock()
	for i := len(b.questionOrder) - 1; i >= 0; i-- {
		if req, ok := b.questions[b.questionOrder[i]]; ok {
			return req, true
		}
	}
	return question.Request{}, false
}

// renderPermissionCard renders a permission request for the thread.
func renderPermissionCard(req permission.PermissionRequest) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s **Permission needed** · `%s`\n", iconPermission, req.ToolName)
	desc := strings.TrimSpace(req.Description)
	if desc == "" {
		desc = strings.TrimSpace(req.Action + " " + req.Path)
	}
	if desc != "" {
		sb.WriteString(compactLine(desc, 400) + "\n")
	}
	sb.WriteString("-# react ✅ allow · 🔁 allow for session · ❌ deny — or `/allow` · `/deny`")
	return sb.String()
}

// questionHint selects the closing instruction on a question card.
type questionHint int

const (
	// questionHintNone renders a card without its closing instruction,
	// for a card that is no longer open for input.
	questionHintNone questionHint = iota
	// questionHintButtons tells the user the choices are buttons.
	questionHintButtons
)

// renderQuestionContent renders one question of a batch. Each question is
// posted as its own message so its controls sit directly under it; the
// choices carry their descriptions on indented sub-lines, matching the TUI.
func renderQuestionContent(req question.Request, questionIndex int, hint questionHint) string {
	if questionIndex < 0 || questionIndex >= len(req.Questions) {
		return ""
	}
	q := req.Questions[questionIndex]

	var sb strings.Builder
	if len(req.Questions) > 1 {
		fmt.Fprintf(&sb, "%s **Question %d/%d", iconQuestion, questionIndex+1, len(req.Questions))
	} else {
		sb.WriteString(iconQuestion + " **Question")
	}
	if q.Label != "" {
		sb.WriteString(" · " + plainLine(q.Label))
	}
	sb.WriteString("**\n")
	sb.WriteString(strings.TrimSpace(q.Text) + "\n")
	if q.Description != "" {
		sb.WriteString("-# " + plainLine(q.Description) + "\n")
	}
	for j, choice := range questionChoices(q) {
		// The body carries the full label; the button label alone is
		// capped at Discord's hard limit.
		fmt.Fprintf(&sb, "**%d.** %s\n", j+1, plainLine(choice.Label))
		if choice.Description != "" {
			sb.WriteString("-# " + plainLine(choice.Description) + "\n")
		}
	}
	switch hint {
	case questionHintButtons:
		sb.WriteString(questionButtonsHint)
	}
	return sb.String()
}

// questionButtonsHint closes an open question card.
const questionButtonsHint = "-# press a button to answer, or **Something else** to type your own"

// cardChunk returns the chunk of a card that carries its controls. A card
// short enough for one message returns its whole content.
func cardChunk(content string) string {
	chunks := splitMessage(content)
	if len(chunks) == 0 {
		return content
	}
	return chunks[len(chunks)-1]
}

// cardChunks returns the posted chunks of a card, falling back to the
// last chunk when the message count differs from the current content.
func cardChunks(content string, n int) []string {
	chunks := splitMessage(content)
	if len(chunks) == 0 {
		return nil
	}
	out := make([]string, n)
	for i := range n {
		out[i] = chunks[min(i, len(chunks)-1)]
	}
	return out
}

// editAnsweredCard rewrites every message of a resolved question card,
// dropping its controls and replacing the closing hint with the answered
// marker on the final chunk.
func (b *Bridge) editAnsweredCard(ids []string, req question.Request, questionIndex int) {
	if len(ids) == 0 {
		return
	}
	chunks := cardChunks(renderQuestionContent(req, questionIndex, questionHintButtons), len(ids))
	for i, id := range ids {
		chunk := chunks[i]
		if i == len(ids)-1 {
			chunk = strings.TrimSuffix(chunk, questionButtonsHint) + "-# ✅ answered"
		}
		if err := b.api.editMessage(b.ctx, b.threadID, id, chunk); err != nil {
			slog.Debug("Discord bridge: failed to update question card", "error", err.Error())
		}
	}
}

// ── Discord input ────────────────────────────────────────────────────────

// inputEvent is one unit of Discord input, consumed in arrival order.
type inputEvent struct {
	message  *gatewayMessage
	reaction *gatewayReaction
}

func (b *Bridge) enqueueInput(ev inputEvent) {
	select {
	case b.input <- ev:
	case <-b.ctx.Done():
	}
}

// enqueueInteraction queues a Discord interaction on its own channel.
// Interactions must be acknowledged within three seconds, so they cannot
// share the serial prompt queue: a slow prompt submission (queue lookups,
// footer edits) would otherwise make button presses look like "application
// did not respond".
func (b *Bridge) enqueueInteraction(in gatewayInteraction) {
	select {
	case b.interactions <- in:
	case <-b.ctx.Done():
	default:
		slog.Debug("Discord bridge: dropping interaction, queue full", "type", in.Type)
	}
}

// interactionLoop answers slash commands, button presses, and modal
// submissions independently of prompt traffic.
func (b *Bridge) interactionLoop() {
	for {
		select {
		case <-b.ctx.Done():
			return
		case in := <-b.interactions:
			switch in.Type {
			case gwInteractionApplicationCommand:
				// Slash commands are acknowledged at the top of the
				// handler; run the rest off the loop so a slow command
				// cannot delay the next interaction's acknowledgement.
				go b.handleThreadInteraction(in)
			case gwInteractionMessageComponent:
				b.handleQuestionComponent(in)
			case gwInteractionModalSubmit:
				b.handleQuestionModalSubmit(in)
			}
		}
	}
}

// inputLoop consumes gateway input serially so prompts keep their
// author's order.
func (b *Bridge) inputLoop() {
	for {
		select {
		case <-b.ctx.Done():
			return
		case ev := <-b.input:
			switch {
			case ev.message != nil:
				b.handleThreadMessage(*ev.message)
			case ev.reaction != nil:
				b.handleThreadReaction(*ev.reaction)
			}
		}
	}
}

// handleThreadMessage routes one thread message: slash commands run
// here, anything else becomes a prompt. When the message itself pushed
// nothing under the footer (a plain prompt does not), the footer is
// recycled so it stays the thread's last message.
func (b *Bridge) handleThreadMessage(msg gatewayMessage) {
	if msg.ChannelID != b.threadID {
		return
	}
	before := b.footerMessageID()
	b.routeThreadMessage(msg)
	if b.footerMessageID() == before {
		b.reanchorFooter()
	}
}

// routeThreadMessage dispatches one message from the control thread.
func (b *Bridge) routeThreadMessage(msg gatewayMessage) {
	if !b.cfg.allowedUser(msg.Author.ID) {
		b.post("⛔ `" + msg.Author.Username + "` is not allowed to control this session.")
		return
	}
	text := strings.TrimSpace(msg.Content)
	if text == "" && len(msg.Attachments) == 0 {
		return
	}
	// "\/literal" escapes a prompt that starts with a slash.
	if strings.HasPrefix(text, `\/`) {
		b.submitPrompt(text[1:], msg.Attachments)
		return
	}
	if name, args, ok := parseCommand(text); ok {
		if reply := b.handleCommand(b.ctx, name, args); reply != "" {
			b.post(reply)
		}
		return
	}
	b.submitPrompt(text, msg.Attachments)
}

// handleThreadInteraction answers native Discord slash commands. The
// picker's commands are registered on the application by the
// commandcode bridge, so they appear guild-wide; remote control only
// answers them in its own thread. The command is acknowledged first
// (ephemeral, inside Discord's three-second window) and then runs
// through the same pipeline as a typed command, posting its output in
// the thread.
func (b *Bridge) handleThreadInteraction(in gatewayInteraction) {
	if in.ChannelID != b.threadID {
		return
	}
	if !b.cfg.allowedUser(in.userID()) {
		b.answerInteraction(in, "⛔ Not authorized for this session.")
		return
	}
	name, args := interactionCommand(in)
	usage := "/" + name
	if args != "" {
		usage += " " + args
	}
	b.answerInteraction(in, "Running `"+usage+"`")
	if reply := b.handleCommand(b.ctx, name, args); reply != "" {
		b.post(reply)
	}
}

// handleQuestionComponent answers a question-card button press: a choice
// selects (or toggles) for its question, and a submit press sends the
// batch's answers. The pressed card is redrawn through the interaction
// callback, so a press never needs a separate message edit.
func (b *Bridge) handleQuestionComponent(in gatewayInteraction) {
	if in.ChannelID != b.threadID {
		return
	}
	if !b.cfg.allowedUser(in.userID()) {
		b.answerInteraction(in, "⛔ Not authorized for this session.")
		return
	}
	ref, ok := parseQuestionCustomID(in.Data.CustomID)
	if !ok {
		return
	}

	b.pendingMu.Lock()
	req, pending := b.questions[ref.requestID]
	selections := b.questionSelections[ref.requestID]
	fillIns := b.questionFillIns[ref.requestID]
	msgIDs := append([][]string(nil), b.questionMessages[ref.requestID]...)
	b.pendingMu.Unlock()
	if !pending {
		b.answerInteraction(in, iconError+" That question was already resolved.")
		return
	}

	if ref.submit {
		if !selectionsComplete(req, selections, fillIns) {
			b.answerInteraction(in, iconError+" Answer every question first, or use Something else.")
			return
		}
		b.submitQuestionAnswers(in, req, selections, fillIns, msgIDs, len(req.Questions)-1)
		return
	}

	if ref.questionIndex < 0 || ref.questionIndex >= len(req.Questions) {
		return
	}
	if ref.custom {
		b.openQuestionModal(in, req, ref.questionIndex)
		return
	}
	q := req.Questions[ref.questionIndex]
	choices := questionChoices(q)
	if ref.choiceIndex < 0 || ref.choiceIndex >= len(choices) {
		return
	}
	choiceID := choices[ref.choiceIndex].ID

	b.pendingMu.Lock()
	if selections[q.ID] == nil {
		selections[q.ID] = map[string]bool{}
	}
	if q.Type == question.TypeMultiChoice {
		if selections[q.ID][choiceID] {
			delete(selections[q.ID], choiceID)
		} else {
			selections[q.ID][choiceID] = true
		}
	} else {
		selections[q.ID] = map[string]bool{choiceID: true}
	}
	stillPending := b.questions[req.ID].ID != ""
	b.pendingMu.Unlock()

	// A lone yes/no or single-choice question is answered by the press
	// itself; everything else redraws the question and waits for Submit.
	if !questionNeedsSubmit(req) {
		b.submitQuestionAnswers(in, req, selections, fillIns, msgIDs, ref.questionIndex)
		return
	}
	// The batch may have been resolved from the TUI between the pending
	// check above and this redraw; a redraw would resurrect its buttons.
	if !stillPending {
		b.answerInteraction(in, iconError+" That question was already resolved.")
		return
	}
	b.redrawQuestion(in, req, ref.questionIndex, selections, fillIns, msgIDs)
}

// openQuestionModal answers a "Something else" press with a modal text
// input. The modal response is the interaction acknowledgement, so it is
// sent immediately.
func (b *Bridge) openQuestionModal(in gatewayInteraction, req question.Request, questionIndex int) {
	modal := customAnswerModal(req, questionIndex)
	if err := b.interactionAPI.respondModal(b.ctx, in.ID, in.Token, modal); err != nil {
		slog.Debug("Discord bridge: failed to open question modal", "error", err.Error())
	}
}

// handleQuestionModalSubmit records a custom answer typed into a question
// modal and either submits the batch or redraws the question with the new
// answer highlighted.
func (b *Bridge) handleQuestionModalSubmit(in gatewayInteraction) {
	if in.ChannelID != b.threadID {
		return
	}
	if !b.cfg.allowedUser(in.userID()) {
		b.answerInteraction(in, "⛔ Not authorized for this session.")
		return
	}
	requestID, questionIndex, ok := parseModalCustomID(in.Data.CustomID)
	if !ok {
		return
	}

	b.pendingMu.Lock()
	req, pending := b.questions[requestID]
	selections := b.questionSelections[requestID]
	fillIns := b.questionFillIns[requestID]
	msgIDs := append([][]string(nil), b.questionMessages[requestID]...)
	b.pendingMu.Unlock()
	if !pending {
		b.answerInteraction(in, iconError+" That question was already resolved.")
		return
	}
	if questionIndex < 0 || questionIndex >= len(req.Questions) {
		return
	}
	value := strings.TrimSpace(modalInputValue(in.Data.Components))
	if value == "" {
		b.answerInteraction(in, iconError+" Type an answer first.")
		return
	}

	q := req.Questions[questionIndex]
	b.pendingMu.Lock()
	if fillIns == nil {
		fillIns = map[string]string{}
	}
	fillIns[q.ID] = value
	b.questionFillIns[requestID] = fillIns
	b.pendingMu.Unlock()

	if !questionNeedsSubmit(req) {
		b.submitQuestionAnswers(in, req, selections, fillIns, msgIDs, questionIndex)
		return
	}
	b.redrawQuestion(in, req, questionIndex, selections, fillIns, msgIDs)
}

// redrawQuestion updates one question's card in place. Buttons ride on
// the card's last chunk, so only that message is rewritten; when an
// earlier question was pressed, the last question's submit row is
// refreshed over REST.
func (b *Bridge) redrawQuestion(in gatewayInteraction, req question.Request, questionIndex int, selections map[string]map[string]bool, fillIns map[string]string, cards [][]string) {
	includeSubmit := questionIndex == len(req.Questions)-1
	card := cardChunk(renderQuestionContent(req, questionIndex, questionHintButtons))
	rows := questionRows(req, questionIndex, selections, fillIns, includeSubmit)
	cardID := lastCardID(cards, questionIndex)
	if err := b.interactionAPI.updateInteraction(b.ctx, in.ID, in.Token, card, rows); err != nil {
		slog.Debug("Discord bridge: failed to update question message", "error", err.Error())
		if cardID != "" {
			if editErr := b.api.editMessage(b.ctx, b.threadID, cardID, card, rows...); editErr != nil {
				slog.Debug("Discord bridge: failed to redraw question message", "error", editErr.Error())
			}
		}
	}
	if includeSubmit {
		return
	}
	last := len(req.Questions) - 1
	if lastID := lastCardID(cards, last); lastID != "" {
		lastCard := cardChunk(renderQuestionContent(req, last, questionHintButtons))
		lastRows := questionRows(req, last, selections, fillIns, true)
		if err := b.api.editMessage(b.ctx, b.threadID, lastID, lastCard, lastRows...); err != nil {
			slog.Debug("Discord bridge: failed to refresh submit row", "error", err.Error())
		}
	}
}

// lastCardID returns the id of the message that carries a question card's
// controls (its final chunk), or "" when the card was never posted.
func lastCardID(cards [][]string, questionIndex int) string {
	if questionIndex < 0 || questionIndex >= len(cards) {
		return ""
	}
	ids := cards[questionIndex]
	if len(ids) == 0 {
		return ""
	}
	return ids[len(ids)-1]
}

// submitQuestionAnswers sends a batch's answers and marks every question
// card resolved. The question notification that follows is a no-op
// because the batch is already removed here.
func (b *Bridge) submitQuestionAnswers(in gatewayInteraction, req question.Request, selections map[string]map[string]bool, fillIns map[string]string, cards [][]string, originIndex int) {
	if !b.env.QuestionAnswer(answersFromSelections(req, selections, fillIns)) {
		b.answerInteraction(in, iconError+" That question was already resolved.")
		return
	}
	b.pendingMu.Lock()
	delete(b.questions, req.ID)
	delete(b.questionSelections, req.ID)
	delete(b.questionFillIns, req.ID)
	delete(b.questionMessages, req.ID)
	b.pendingMu.Unlock()

	// The interaction callback acknowledges the press and can rewrite the
	// originating message; every other chunk is resolved over REST.
	acked := false
	for i, ids := range cards {
		if len(ids) == 0 {
			continue
		}
		chunks := cardChunks(renderQuestionContent(req, i, questionHintButtons), len(ids))
		for j, id := range ids {
			chunk := chunks[j]
			if j == len(ids)-1 {
				chunk = strings.TrimSuffix(chunk, questionButtonsHint) + "-# ✅ answered"
			}
			if acked || i != originIndex || j != len(ids)-1 {
				if err := b.api.editMessage(b.ctx, b.threadID, id, chunk); err != nil {
					slog.Debug("Discord bridge: failed to resolve question message", "error", err.Error())
				}
				continue
			}
			if err := b.interactionAPI.updateInteraction(b.ctx, in.ID, in.Token, chunk, nil); err != nil {
				slog.Debug("Discord bridge: failed to mark question card answered", "error", err.Error())
				if editErr := b.api.editMessage(b.ctx, b.threadID, id, chunk); editErr != nil {
					slog.Debug("Discord bridge: failed to resolve question card", "error", editErr.Error())
				}
			}
			acked = true
		}
	}
	if !acked && len(cards) > 0 {
		// No message matched the origin index (should not happen); still
		// acknowledge so Discord does not show "application did not respond".
		b.answerInteraction(in, "✅ Answered.")
	}
}

// modalInputValue extracts the first non-empty text input from a modal
// submit payload.
func modalInputValue(rows []gatewayInteractionComponent) string {
	for _, row := range rows {
		for _, component := range row.Components {
			if v := strings.TrimSpace(component.Value); v != "" {
				return v
			}
		}
	}
	return ""
}

// interactionCommand maps an application command onto the bridge's
// command name and argument text. /compact and /summarize are the same
// command; the name is kept as Discord sent it so the interaction
// callback echoes what the author actually ran.
func interactionCommand(in gatewayInteraction) (name, args string) {
	name = strings.ToLower(strings.TrimSpace(in.Data.Name))
	var values []string
	for _, opt := range in.Data.Options {
		if v := strings.TrimSpace(opt.Value); v != "" {
			values = append(values, v)
		}
	}
	return name, strings.Join(values, " ")
}

// answerInteraction posts an ephemeral interaction callback. Failures
// are logged only: Discord shows "The application did not respond" when
// no callback arrives, and there is nothing else the bridge can do.
func (b *Bridge) answerInteraction(in gatewayInteraction, content string) {
	if err := b.interactionAPI.respondInteraction(b.ctx, in.ID, in.Token, content, true); err != nil {
		slog.Debug("Discord bridge: interaction response failed", "error", err.Error())
	}
}

// handleThreadReaction resolves a permission card from its reactions.
func (b *Bridge) handleThreadReaction(r gatewayReaction) {
	if r.ChannelID != b.threadID || !b.cfg.allowedUser(r.UserID) {
		return
	}
	b.pendingMu.Lock()
	var reqID string
	for id, msgID := range b.permMessage {
		if msgID == r.MessageID {
			reqID = id
			break
		}
	}
	req, ok := b.permissions[reqID]
	b.pendingMu.Unlock()
	if !ok {
		return
	}
	switch r.Emoji.Name {
	case reactionAllow:
		b.env.PermissionGrant(req)
	case reactionAllowSession:
		b.env.PermissionGrantPersistent(req)
	case reactionDeny:
		b.env.PermissionDeny(req)
	}
}

// submitPrompt injects a prompt from the thread into the session,
// mirroring any attachments.
func (b *Bridge) submitPrompt(text string, attachments []gatewayAttachment) {
	b.launchRun(text, b.fetchAttachments(attachments)...)
}

// launchRun submits a prompt to the agent in the background. Run itself
// blocks for the whole turn, so it never runs on an input path. The run
// is started with a background context: stopping remote control must not
// cancel work the local session is still running, matching how the TUI
// submits prompts.
func (b *Bridge) launchRun(prompt string, attachments ...message.Attachment) {
	if strings.TrimSpace(prompt) == "" && len(attachments) == 0 {
		return
	}
	b.noteSubmission(prompt)
	sessionID := b.currentSessionID()
	if b.env.AgentIsSessionBusy(sessionID) {
		position := b.env.AgentQueuedPrompts(sessionID) + 1
		b.post(fmt.Sprintf("📥 Queued (position %d) — the current turn is still running.", position))
	}
	go func() {
		if err := b.env.AgentRun(context.Background(), sessionID, prompt, attachments...); err != nil && b.ctx.Err() == nil {
			// AgentRun blocks for the whole turn in local mode, so a turn
			// failure (provider timeout, tool error) arrives here. The
			// footer and agent notifications already report it; calling it
			// a failed prompt is wrong and duplicates the error.
			slog.Debug("Discord bridge: agent run ended with error", "error", err.Error())
		}
	}()
}

// fetchAttachments downloads mirrored attachments; oversized or failed
// downloads are skipped rather than failing the prompt.
func (b *Bridge) fetchAttachments(atts []gatewayAttachment) []message.Attachment {
	var out []message.Attachment
	for _, a := range atts {
		if a.Size > maxAttachmentBytes {
			b.post(fmt.Sprintf("-# ⚠️ skipped attachment `%s` (larger than %d MiB)", a.Filename, maxAttachmentBytes>>20))
			continue
		}
		ctx, cancel := context.WithTimeout(b.ctx, 30*time.Second)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
		if err != nil {
			cancel()
			continue
		}
		resp, err := b.http.Do(req)
		if err != nil {
			cancel()
			slog.Debug("Discord bridge: attachment download failed", "error", err.Error())
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, maxAttachmentBytes+1))
		_ = resp.Body.Close()
		cancel()
		if readErr != nil || len(data) > maxAttachmentBytes {
			continue
		}
		out = append(out, message.Attachment{
			FilePath: a.Filename,
			FileName: a.Filename,
			MimeType: a.ContentType,
			Content:  data,
		})
	}
	return out
}

// noteSubmission records a Discord-injected prompt for echo dedupe.
func (b *Bridge) noteSubmission(prompt string) {
	b.subMu.Lock()
	defer b.subMu.Unlock()
	b.submissions = append(b.submissions, promptSubmission{hash: hashPrompt(prompt), when: b.now()})
	if len(b.submissions) > 32 {
		b.submissions = b.submissions[len(b.submissions)-32:]
	}
}

// consumeSubmission reports whether a user-message echo belongs to a
// recently injected Discord prompt.
func (b *Bridge) consumeSubmission(prompt string) bool {
	hash := hashPrompt(prompt)
	b.subMu.Lock()
	defer b.subMu.Unlock()
	for i, s := range b.submissions {
		if s.hash == hash && b.now().Sub(s.when) < 2*time.Minute {
			b.submissions = append(b.submissions[:i], b.submissions[i+1:]...)
			return true
		}
	}
	return false
}

func hashPrompt(prompt string) string {
	sum := sha256.Sum256([]byte(prompt))
	return hex.EncodeToString(sum[:])
}

// ── Gateway and typing ───────────────────────────────────────────────────

// runGateway fetches the gateway URL once and keeps the connection
// alive. Errors are surfaced through notes; the bridge stays usable in
// the meantime (permission cards, etc.) until the gateway returns.
func (b *Bridge) runGateway(ctx context.Context) {
	url, err := b.api.gatewayURL(ctx)
	if err != nil {
		b.note("Discord gateway unavailable: "+err.Error(), true)
		return
	}
	b.gateway.url = url
	b.gateway.onMessage = func(m gatewayMessage) { b.enqueueInput(inputEvent{message: &m}) }
	b.gateway.onReaction = func(r gatewayReaction) { b.enqueueInput(inputEvent{reaction: &r}) }
	b.gateway.onInteraction = func(in gatewayInteraction) { b.enqueueInteraction(in) }
	_ = b.gateway.run(ctx, func(err error) {
		b.note("Discord gateway error: "+err.Error()+" (check the bot token, MESSAGE CONTENT intent, and channel permissions)", true)
	})
}

// typingLoop shows the typing indicator while the agent is busy.
func (b *Bridge) typingLoop() {
	ticker := time.NewTicker(b.typing)
	defer ticker.Stop()
	for {
		select {
		case <-b.ctx.Done():
			return
		case <-ticker.C:
			if !b.env.AgentIsSessionBusy(b.currentSessionID()) {
				continue
			}
			ctx, cancel := context.WithTimeout(b.ctx, 5*time.Second)
			_ = b.api.triggerTyping(ctx, b.threadID)
			cancel()
		}
	}
}

// ── Status footer ────────────────────────────────────────────────────────

// turnState is the live phase of an assistant turn.
type turnState int

const (
	turnIdle turnState = iota
	turnThinking
	turnWriting
	turnTool
	turnWaitingPermission
	turnDone
	turnCancelled
	turnError
)

func (s turnState) terminal() bool {
	return s == turnDone || s == turnCancelled || s == turnError
}

// footer is the bridge's status message: a single message that lives at
// the bottom of the thread and carries the live turn state, per-turn
// tool counts, and session metrics. Assistant text mirrors into its own
// transcript messages above the footer, so nothing is cut to a preview.
// The footer persists between turns so it is always the last message;
// content that must appear above it recycles the footer message (see
// postAbove and reanchorFooter).
type footer struct {
	mu sync.Mutex

	// messageID is the footer's current message. "" means no message is
	// in place (Discord trouble, or a fresh bridge) and the next flush
	// re-creates it.
	messageID string
	stopped   bool

	// Turn state.
	active  bool
	started time.Time
	// finished records when the run reached a terminal state so the
	// "Done in …" duration can freeze at completion; the idle ticker
	// must not keep growing it.
	finished  time.Time
	state     turnState
	toolName  string
	toolParam string
	model     string
	errText   string
	waiting   bool

	// Per-turn aggregates.
	toolCounts   map[string]int
	toolFailures int
	countedCalls map[string]bool

	// tail is the current step's streamed content. It drives the live
	// transcript messages and the writing state; the footer itself no
	// longer renders it.
	tail string

	// Usage metrics from the latest session snapshot.
	sess     session.Session
	haveSess bool

	frame    int
	lastBody string
	lastEdit time.Time

	updates chan struct{}
}

// setSession records the latest session snapshot for footer metrics.
func (b *Bridge) setSession(sess session.Session) {
	f := b.footer
	f.mu.Lock()
	f.sess = sess
	f.haveSess = true
	f.mu.Unlock()
	b.pokeFooter()
}

// rebindSession points the bridge at a new session without touching the
// thread: /clear starts a fresh context and the same thread follows it.
// Every per-session tally is dropped so no card, dedup entry, or footer
// reading from the cleared session leaks into the new one. The session
// lock is held across the whole reset so events for the new session
// cannot slip in between the ID swap and the state wipe; handlers that
// see the old ID either run before the swap or not at all.
func (b *Bridge) rebindSession(ctx context.Context, to session.Session) {
	b.sessMu.Lock()
	from := b.sessionID
	b.sessionID = to.ID

	b.pendingMu.Lock()
	b.permissions = map[string]permission.PermissionRequest{}
	b.permissionOrder = nil
	b.permMessage = map[string]string{}
	b.questions = map[string]question.Request{}
	b.questionOrder = nil
	b.questionMessages = map[string][][]string{}
	b.questionSelections = map[string]map[string]map[string]bool{}
	b.questionFillIns = map[string]map[string]string{}
	b.pendingMu.Unlock()

	b.subMu.Lock()
	b.submissions = nil
	b.subMu.Unlock()

	b.countedMu.Lock()
	clear(b.countedResults)
	b.countedMu.Unlock()

	b.frozenMu.Lock()
	clear(b.frozen)
	b.frozenMu.Unlock()

	b.completedMu.Lock()
	clear(b.completed)
	b.completedMu.Unlock()

	b.renameMu.Lock()
	b.renamedTo = ""
	b.renameMu.Unlock()
	b.sessMu.Unlock()

	b.lastMu.Lock()
	b.lastAssistant = message.Message{}
	b.lastMu.Unlock()

	// Reset the footer to its idle fresh-session shape so the previous
	// turn's tool counts and streaming tail do not linger.
	f := b.footer
	f.mu.Lock()
	f.active = false
	f.started = time.Time{}
	f.state = turnIdle
	f.toolName = ""
	f.toolParam = ""
	f.model = ""
	f.errText = ""
	f.waiting = false
	f.toolCounts = map[string]int{}
	f.toolFailures = 0
	f.countedCalls = map[string]bool{}
	f.tail = ""
	f.mu.Unlock()
	b.resetLiveTranscript()

	b.setSession(to)

	// The old title would mislabel the thread; the next generated title
	// renames it again (renamedTo was reset above).
	if err := b.api.editChannelName(ctx, b.threadID, b.threadName(ctx)); err != nil {
		slog.Debug("Discord bridge: failed to rename thread", "error", err.Error())
	}

	if b.app != nil {
		b.app.SendEvent(SessionClearedMsg{Bridge: b, From: from, To: to.ID})
	}
}

// footerLoop owns the footer's edit cadence: event pokes are throttled
// and the ticker animates the spinner and refreshes the elapsed timer.
func (b *Bridge) footerLoop() {
	ticker := time.NewTicker(b.tick)
	defer ticker.Stop()
	for {
		select {
		case <-b.ctx.Done():
			return
		case <-b.footer.updates:
			f := b.footer
			f.mu.Lock()
			due := b.now().Sub(f.lastEdit) >= b.editRate
			f.mu.Unlock()
			if due {
				b.flushFooter()
			}
		case <-ticker.C:
			b.flushFooter()
		}
	}
}

// pokeFooter wakes the footer loop without blocking the caller.
func (b *Bridge) pokeFooter() {
	select {
	case b.footer.updates <- struct{}{}:
	default:
	}
}

// flushFooter composes and posts or edits the footer when its body
// changed.
func (b *Bridge) flushFooter() {
	f := b.footer
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.stopped {
		return
	}
	f.frame++
	body := b.composeFooterLocked()
	if body == f.lastBody {
		return
	}
	if f.messageID == "" {
		id, err := b.api.createMessage(b.ctx, b.threadID, body)
		if err != nil {
			slog.Debug("Discord bridge: failed to post status footer", "error", err.Error())
			return
		}
		f.messageID = id
	} else if err := b.api.editMessage(b.ctx, b.threadID, f.messageID, body); err != nil {
		slog.Debug("Discord bridge: failed to edit status footer", "error", err.Error())
		// The message may be gone (deleted, or from a previous bot
		// session): drop it so the next flush re-creates the footer.
		f.messageID = ""
		f.lastBody = ""
		return
	}
	f.lastBody = body
	f.lastEdit = b.now()
}

// footerMessageID returns the footer's current message id, if any.
func (b *Bridge) footerMessageID() string {
	f := b.footer
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.messageID
}

// attachFooterLocked posts a fresh footer message when one is not
// already in place, so the footer stays the thread's last message. Must
// be called with f.mu held.
func (b *Bridge) attachFooterLocked() {
	f := b.footer
	if f.stopped || f.messageID != "" {
		return
	}
	body := b.composeFooterLocked()
	id, err := b.api.createMessage(b.ctx, b.threadID, body)
	if err != nil {
		slog.Debug("Discord bridge: failed to post status footer", "error", err.Error())
		return
	}
	f.messageID = id
	f.lastBody = body
	f.lastEdit = b.now()
}

// reanchorFooter moves the footer back to the bottom after a message
// the bridge did not post (a user prompt) landed below it, by deleting
// and re-creating the footer message.
func (b *Bridge) reanchorFooter() {
	f := b.footer
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.stopped || f.messageID == "" {
		return
	}
	if err := b.api.deleteMessage(b.ctx, b.threadID, f.messageID); err != nil {
		slog.Debug("Discord bridge: failed to delete status footer", "error", err.Error())
	}
	f.messageID = ""
	f.lastBody = ""
	b.attachFooterLocked()
}

// composeFooterLocked renders the footer body: status header, per-turn
// tool summary, and usage metrics. Assistant text no longer lives here:
// it streams into transcript messages above the footer so nothing is
// truncated to a tail window.
// Must be called with f.mu held.
func (b *Bridge) composeFooterLocked() string {
	f := b.footer
	lines := []string{b.renderFooterHeaderLocked()}
	if summary := renderToolSummary(f.toolCounts, f.toolFailures); summary != "" {
		lines = append(lines, summary)
	}
	if f.haveSess {
		if metrics := renderMetricsLine(f.sess, b.modelContextWindow()); metrics != "" {
			lines = append(lines, metrics)
		}
	}
	return strings.Join(lines, "\n")
}

// renderFooterHeaderLocked renders the footer's status header. Must be
// called with f.mu held.
func (b *Bridge) renderFooterHeaderLocked() string {
	f := b.footer
	if f.state == turnIdle {
		return iconBot + " **Ready** — watching the session"
	}
	elapsed := b.now().Sub(f.started)
	if f.state.terminal() && !f.finished.IsZero() {
		// The run is over: freeze the duration at completion instead of
		// letting the idle ticker keep growing "Done in …".
		elapsed = f.finished.Sub(f.started)
	}
	switch f.state {
	case turnDone:
		line := fmt.Sprintf("%s **Done in %s**", iconDone, humanDuration(elapsed))
		if f.model != "" {
			line += " · `" + f.model + "`"
		}
		return line
	case turnCancelled:
		return fmt.Sprintf("%s **Cancelled after %s**", iconCancelled, humanDuration(elapsed))
	case turnError:
		line := fmt.Sprintf("%s **Failed after %s**", iconError, humanDuration(elapsed))
		if f.errText != "" {
			line += " — " + compactLine(f.errText, 200)
		}
		return line
	}

	icon, label := iconThinking, "Thinking…"
	switch f.state {
	case turnWriting:
		icon, label = iconWriting, "Writing…"
	case turnTool:
		icon, label = iconTool, toolVerb(f.toolName, f.toolParam)+"…"
	case turnWaitingPermission:
		icon, label = iconPermission, "Waiting for permission…"
	}
	return fmt.Sprintf("%s %s **%s** %s", spinnerFrame(f.frame), icon, label, humanDuration(elapsed))
}

// modelContextWindow resolves the context window of the configured
// large model from the configured providers; 0 when it cannot be
// resolved.
func (b *Bridge) modelContextWindow() int64 {
	cfg := b.env.Config()
	if cfg == nil {
		return 0
	}
	sel, ok := cfg.Models[config.SelectedModelTypeLarge]
	if !ok {
		return 0
	}
	for _, p := range configuredProviders(cfg) {
		if string(p.ID) != sel.Provider {
			continue
		}
		for _, m := range p.Models {
			if m.ID == sel.Model {
				return m.ContextWindow
			}
		}
	}
	return 0
}

// applyTurnLocked advances the footer from a message snapshot. Must be
// called with f.mu held.
func (b *Bridge) applyTurnLocked(msg message.Message, waiting bool) {
	f := b.footer
	if !f.active {
		f.active = true
		f.started = b.now()
		f.finished = time.Time{}
		f.errText = ""
		f.tail = ""
		f.toolCounts = map[string]int{}
		f.toolFailures = 0
		f.countedCalls = map[string]bool{}
		f.state = turnThinking
	}
	if msg.Model != "" {
		f.model = msg.Model
		if msg.Provider != "" {
			f.model = msg.Provider + "/" + msg.Model
		}
	}
	for _, tc := range toolCalls(msg) {
		if tc.ID == "" || f.countedCalls[tc.ID] {
			continue
		}
		f.countedCalls[tc.ID] = true
		f.toolCounts[cmpOr(tc.Name, "tool")]++
	}
	f.tail = renderAssistantTimeline(msg)
	state, call := deriveTurnState(msg)
	f.state = state
	f.toolName = call.Name
	f.toolParam = toolMainParam(call.Input)
	if waiting && !f.state.terminal() {
		f.state = turnWaitingPermission
	}
}

// markWaiting flips the waiting-for-permission phase.
func (b *Bridge) markWaiting(waiting bool) {
	f := b.footer
	f.mu.Lock()
	f.waiting = waiting
	if f.active {
		switch {
		case waiting:
			f.state = turnWaitingPermission
		case f.state == turnWaitingPermission:
			f.state = turnThinking
			if strings.TrimSpace(f.tail) != "" {
				f.state = turnWriting
			}
		}
	}
	f.mu.Unlock()
	b.pokeFooter()
}

// freezeStep finalizes a completed step's transcript above the footer
// and marks the step frozen so later snapshots cannot repost it.
func (b *Bridge) freezeStep(msg message.Message) {
	content := renderAssistantTimeline(msg)
	if strings.TrimSpace(content) == "" {
		return
	}
	f := b.footer
	f.mu.Lock()
	f.tail = ""
	f.mu.Unlock()
	b.freezeContent(msg.ID, content)
}

// freezeContent writes a step's final text into its transcript messages
// and forgets the live set so the next step starts fresh. The frozen set
// keeps repeated snapshots of the same step from double-posting.
func (b *Bridge) freezeContent(id, content string) {
	if strings.TrimSpace(content) == "" {
		return
	}
	if id != "" {
		b.frozenMu.Lock()
		if b.frozen[id] {
			b.frozenMu.Unlock()
			return
		}
		if len(b.frozen) >= frozenLimit {
			clear(b.frozen)
		}
		b.frozen[id] = true
		b.frozenMu.Unlock()
	}
	b.syncLiveTranscript(content)
	b.resetLiveTranscript()
}

// isFrozen reports whether a step's transcript was already posted.
func (b *Bridge) isFrozen(id string) bool {
	if id == "" {
		return false
	}
	b.frozenMu.Lock()
	defer b.frozenMu.Unlock()
	return b.frozen[id]
}

// setLastAssistant remembers the newest assistant snapshot for the
// RunComplete fallback freeze.
func (b *Bridge) setLastAssistant(msg message.Message) {
	b.lastMu.Lock()
	b.lastAssistant = msg
	b.lastMu.Unlock()
}

// lastAssistantMessage returns the newest assistant snapshot.
func (b *Bridge) lastAssistantMessage() message.Message {
	b.lastMu.Lock()
	defer b.lastMu.Unlock()
	return b.lastAssistant
}

// hasFinishPart reports whether a message snapshot carries its Finish
// part, i.e. the step stopped streaming.
func hasFinishPart(msg message.Message) bool {
	for _, part := range msg.Parts {
		if _, ok := part.(message.Finish); ok {
			return true
		}
	}
	return false
}

// deriveTurnState infers the live phase from a message snapshot.
func deriveTurnState(msg message.Message) (turnState, message.ToolCall) {
	lastTool, lastText, lastReasoning := -1, -1, -1
	var lastCall message.ToolCall
	for i, part := range msg.Parts {
		switch p := part.(type) {
		case message.ToolCall:
			lastTool = i
			lastCall = p
		case message.TextContent:
			if !p.Hidden && strings.TrimSpace(p.Text) != "" {
				lastText = i
			}
		case message.ReasoningContent:
			lastReasoning = i
		}
	}
	switch {
	case lastText >= 0 && lastText > lastTool && lastText > lastReasoning:
		return turnWriting, message.ToolCall{}
	case lastTool >= 0 && lastTool > lastReasoning:
		return turnTool, lastCall
	default:
		return turnThinking, message.ToolCall{}
	}
}

// ── Message helpers ──────────────────────────────────────────────────────

// messageText returns a message's first visible text part.
func messageText(msg message.Message) string {
	for _, part := range msg.Parts {
		if content, ok := part.(message.TextContent); ok && !content.Hidden && content.Text != "" {
			return content.Text
		}
	}
	return ""
}

// toolCalls extracts the tool-call parts of a message.
func toolCalls(msg message.Message) []message.ToolCall {
	var out []message.ToolCall
	for _, part := range msg.Parts {
		if tc, ok := part.(message.ToolCall); ok {
			out = append(out, tc)
		}
	}
	return out
}

// firstToolResult extracts a message's first tool result.
func firstToolResult(msg message.Message) (message.ToolResult, bool) {
	for _, part := range msg.Parts {
		if tr, ok := part.(message.ToolResult); ok {
			return tr, true
		}
	}
	return message.ToolResult{}, false
}

// cmpOr returns the first non-empty string.
func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
