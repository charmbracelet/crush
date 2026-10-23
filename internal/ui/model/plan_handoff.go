package model

import (
	"context"
	"errors"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	agenttools "github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/util"
)

// planImplementation snapshots the approved plan before asynchronous work.
type planImplementation struct {
	sourceSessionID string
	text            string
	options         dialog.PlanHandoffOptions
	loaded          loadSessionMsg
	ctx             context.Context
	cancel          context.CancelFunc
}

type planCoderReadyMsg struct {
	request *planImplementation
	err     error
}

type planSessionPreparedMsg struct {
	request   *planImplementation
	sessionID string
	loaded    loadSessionMsg
	err       error
}

func (m *UI) prepareNewSessionForPlanImpl(options dialog.PlanHandoffOptions) tea.Cmd {
	if m.modeSwitching || m.isAgentBusy() {
		return util.ReportWarn("Agent is busy, please wait before implementing the plan...")
	}
	if !m.hasSession() || m.planReadySessionID != m.session.ID || strings.TrimSpace(m.planReadyText) == "" {
		return util.ReportError(errors.New("no ready plan to implement"))
	}
	ctx, cancel := context.WithCancel(context.Background())
	request := &planImplementation{
		sourceSessionID: m.session.ID,
		text:            m.planReadyText,
		options:         options,
		ctx:             ctx,
		cancel:          cancel,
	}
	m.planImplementation = request
	m.modeSwitching = true
	ws := m.com.Workspace
	return func() tea.Msg {
		result := planSessionPreparedMsg{request: request}
		sess, err := ws.CreateSession(ctx, "New Session")
		result.sessionID = sess.ID
		if err == nil {
			sess, err = ws.GetSession(ctx, sess.ID)
		}
		if err == nil {
			result.loaded.session = &sess
			result.loaded.messages, err = ws.ListMessages(ctx, sess.ID)
		}
		result.err = err
		return result
	}
}

func (m *UI) planImplementationCurrent(request *planImplementation) bool {
	return m.planImplementation == request && m.session != nil &&
		m.session.ID == request.sourceSessionID &&
		m.planReadySessionID == request.sourceSessionID
}

func (m *UI) applyPlanSessionPrepared(msg planSessionPreparedMsg) []tea.Cmd {
	if !m.planImplementationCurrent(msg.request) || msg.err != nil {
		return m.abortPlanImplementation(msg.request, msg.sessionID, msg.err)
	}
	msg.request.loaded = msg.loaded
	ws := m.com.Workspace
	request := msg.request
	return []tea.Cmd{func() tea.Msg {
		if err := request.ctx.Err(); err != nil {
			return planCoderReadyMsg{err: err, request: request}
		}
		previousPermissionsDecision := ws.PermissionSkipRequests()
		ws.PermissionSetSkipRequests(request.options.YOLO)
		err := ws.AgentSetMain(config.AgentCoder)
		switched := err == nil
		if err == nil {
			err = ws.UpdateAgentModel(request.ctx)
		}
		if err != nil && request.ctx.Err() == nil {
			ws.PermissionSetSkipRequests(previousPermissionsDecision)
			// Restore planning if rebuilding the coder model failed after
			// the backend had already selected it. A superseded request must
			// not undo a subsequent session's mode or permission changes.
			if switched {
				rollbackErr := ws.AgentSetMain(config.AgentPlan)
				if rollbackErr == nil {
					rollbackErr = ws.UpdateAgentModel(context.Background())
				}
				err = errors.Join(err, rollbackErr)
			}
		}
		return planCoderReadyMsg{request: request, err: err}
	}}
}

func (m *UI) startPlanImplementation(msg planCoderReadyMsg) []tea.Cmd {
	request := msg.request
	if !m.planImplementationCurrent(request) || msg.err != nil {
		return m.abortPlanImplementation(request, request.loaded.session.ID, msg.err)
	}
	m.planImplementation = nil
	m.modeSwitching = false
	request.cancel()
	m.setPlanReadyPending("")
	m.mode = uiInputModeCode
	m.cycleYolo = false
	m.yoloCache.set(request.options.YOLO)
	m.setEditorPrompt(request.options.YOLO)
	m.sessionFileReads = nil
	m.pillsExpanded = false
	m.pillsAutoExpanded = false
	m.pillsView = ""
	agenttools.ResetCache()
	// A newly created session has no file history, todos, or model to restore.
	m.session = request.loaded.session
	m.sidebarOffset = 0
	m.sessionFiles = nil
	m.promptQueue = 0
	m.promptQueueItems = nil
	m.promptQueueCheckedAt = time.Time{}
	m.invalidateBusyCaches()
	m.invalidatePromptQueue()
	m.historyReset()
	m.activeInline = nil
	m.textarea.Focus()
	m.chat.Blur()
	var cmds []tea.Cmd
	cmds = append(cmds, m.setSessionMessages(request.loaded.messages))
	m.setState(uiChat, uiFocusEditor)
	cmds = append(cmds, m.dispatchBusyRefresh(), m.dispatchPromptQueueRefresh(), m.loadPromptHistory())
	// Install the empty transcript above before submitting the first turn.
	// Loading it concurrently with AgentRun could overwrite streamed replies.
	report := m.reportCurrentSession(m.session.ID)
	run := m.sendMessageInternal("Implement the following plan:\n\n"+request.text, true)
	cmds = append(cmds, func() tea.Msg {
		report()
		return run()
	})
	if request.options.YOLO {
		cmds = append(cmds, util.CmdHandler(util.InfoMsg{Type: util.InfoTypeYolo, Msg: yoloModeBannerMsg}))
	} else {
		cmds = append(cmds, util.ReportInfo("input mode: code"))
	}
	return cmds
}

func (m *UI) abortPlanImplementation(request *planImplementation, sessionID string, err error) []tea.Cmd {
	current := m.planImplementationCurrent(request)
	request.cancel()
	if m.planImplementation == request {
		m.planImplementation = nil
		m.modeSwitching = false
	}
	if current && m.mode == uiInputModePlan {
		m.openPlanHandoff()
		m.activeInline.(*dialog.PlanHandoffInline).NewSession = request.options.NewSession
	}
	var cmds []tea.Cmd
	if err != nil && !errors.Is(err, context.Canceled) {
		cmds = append(cmds, util.ReportError(err))
	}
	if sessionID != "" {
		ws := m.com.Workspace
		cmds = append(cmds, func() tea.Msg {
			if err := ws.DeleteSession(context.Background(), sessionID); err != nil {
				return util.ReportError(err)()
			}
			return nil
		})
	}
	return cmds
}
