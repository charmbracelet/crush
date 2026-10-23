package model

import (
	"context"
	"errors"
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/ui/util"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

type planHandoffWorkspace struct {
	*testWorkspace
	created     []session.Session
	deleted     []string
	selected    []string
	runSessions []string
	createErr   error
	loadErr     error
	switchErr   error
	modelErr    error
	runErr      error
	beforeRun   func(string)
}

func (w *planHandoffWorkspace) CreateSession(_ context.Context, title string) (session.Session, error) {
	if w.createErr != nil {
		return session.Session{}, w.createErr
	}
	sess := session.Session{ID: fmt.Sprintf("implementation-%d", len(w.created)+1), Title: title}
	w.created = append(w.created, sess)
	return sess, nil
}

func (w *planHandoffWorkspace) GetSession(_ context.Context, id string) (session.Session, error) {
	if w.loadErr != nil {
		return session.Session{}, w.loadErr
	}
	for _, sess := range w.created {
		if sess.ID == id {
			return sess, nil
		}
	}
	return session.Session{}, errors.New("unknown session")
}

func (w *planHandoffWorkspace) DeleteSession(_ context.Context, id string) error {
	w.deleted = append(w.deleted, id)
	return nil
}

func (w *planHandoffWorkspace) SetCurrentSession(_ context.Context, id string) error {
	w.selected = append(w.selected, id)
	return nil
}

func (w *planHandoffWorkspace) ListMessages(context.Context, string) ([]message.Message, error) {
	return nil, nil
}

func (w *planHandoffWorkspace) ListUserMessages(context.Context, string) ([]message.Message, error) {
	return nil, nil
}

func (w *planHandoffWorkspace) AgentModel() workspace.AgentModel {
	return workspace.AgentModel{}
}

func (w *planHandoffWorkspace) AgentQueuedPromptsList(string) []string { return nil }

func (w *planHandoffWorkspace) AgentSetMain(id string) error {
	if w.switchErr != nil {
		return w.switchErr
	}
	return w.testWorkspace.AgentSetMain(id)
}

func (w *planHandoffWorkspace) UpdateAgentModel(ctx context.Context) error {
	if w.modelErr != nil && w.setMainCalledWith == config.AgentCoder {
		return w.modelErr
	}
	return w.testWorkspace.UpdateAgentModel(ctx)
}

func (w *planHandoffWorkspace) AgentRun(ctx context.Context, id, prompt string, attachments ...message.Attachment) error {
	if w.beforeRun != nil {
		w.beforeRun(id)
	}
	w.runSessions = append(w.runSessions, id)
	w.testWorkspace.AgentRun(ctx, id, prompt, attachments...)
	return w.runErr
}

func newPlanHandoffUI(t *testing.T) (*UI, *planHandoffWorkspace) {
	t.Helper()
	u, ws := newPlanUI(t, "source")
	w := &planHandoffWorkspace{testWorkspace: ws}
	w.agentReady = true
	u.com.Workspace = w
	u.status = NewStatus(u.com, nil)
	u.width, u.height = 80, 30
	u.state, u.focus = uiChat, uiFocusEditor
	u.handlePlanHandoff(notify.RunComplete{
		SessionID: "source", Text: common.PlanStartMarker + "\n# Plan\n\nDo the work.\n" + common.PlanReadyMarker,
	})
	return u, w
}

// executePlanCommands runs finite command batches without polling or timers.
func executePlanCommands(cmds []tea.Cmd) []tea.Msg {
	var messages []tea.Msg
	for _, cmd := range cmds {
		if cmd == nil {
			continue
		}
		switch msg := cmd().(type) {
		case tea.BatchMsg:
			messages = append(messages, executePlanCommands(msg)...)
		case tea.Cmd:
			messages = append(messages, executePlanCommands([]tea.Cmd{msg})...)
		default:
			messages = append(messages, msg)
		}
	}
	return messages
}

func TestPlanHandoffImplementationDestinations(t *testing.T) {
	t.Parallel()
	for _, newSession := range []bool{false, true} {
		for _, yolo := range []bool{false, true} {
			t.Run(fmt.Sprintf("new=%v/yolo=%v", newSession, yolo), func(t *testing.T) {
				t.Parallel()
				u, ws := newPlanHandoffUI(t)
				ws.yolo = !yolo
				u.chat.SetMessages(chat.NewAssistantMessageItem(u.com.Styles, &message.Message{
					ID: "source-plan", Role: message.Assistant,
					Parts: []message.ContentPart{message.TextContent{Text: "Source discussion"}},
				}))
				options := dialog.PlanHandoffOptions{NewSession: newSession, YOLO: yolo}
				cmd := u.activeInline.(*dialog.PlanHandoffInline).OnConfirm(options)
				u.activeInline = nil
				var finalize func() []tea.Cmd
				if newSession {
					require.Empty(t, ws.created, "creation belongs in the command")
					prepared := cmd().(planSessionPreparedMsg)
					require.NoError(t, prepared.err)
					require.Empty(t, ws.selected, "preparation must not change presence")
					require.Equal(t, "source", u.session.ID)
					cmds := u.applyPlanSessionPrepared(prepared)
					require.Len(t, cmds, 1)
					switched := cmds[0]().(planCoderReadyMsg)
					require.NoError(t, switched.err)
					finalize = func() []tea.Cmd { return u.startPlanImplementation(switched) }
					ws.beforeRun = func(id string) {
						require.Equal(t, id, u.session.ID)
						require.Nil(t, u.chat.MessageItem("source-plan"))
						require.Equal(t, []string{id}, ws.selected)
					}
				} else {
					switched := cmd().(modeSwitchedMsg)
					require.NoError(t, switched.err)
					finalize = func() []tea.Cmd { return u.applyModeSwitch(switched) }
				}
				require.Empty(t, ws.runPrompts, "wait for coder readiness")
				u.modeSwitching = false
				executePlanCommands(finalize())
				require.Equal(t, uiInputModeCode, u.mode)
				require.Equal(t, config.AgentCoder, ws.setMainCalledWith)
				require.Equal(t, yolo, ws.yolo)
				require.Empty(t, u.planReadyText)
				require.Empty(t, u.planReadySessionID)
				require.Equal(t, []bool{true}, ws.runHidden)
				if newSession {
					require.Equal(t, []string{"implementation-1"}, ws.runSessions)
					require.Equal(t, []string{"Implement the following plan:\n\n# Plan\n\nDo the work."}, ws.runPrompts)
					require.Empty(t, ws.created[0].ParentSessionID)
					require.Equal(t, "New Session", ws.created[0].Title)
				} else {
					require.Empty(t, ws.created)
					require.Equal(t, []string{"source"}, ws.runSessions)
					require.Equal(t, []string{"Implement the plan."}, ws.runPrompts)
				}
			})
		}
	}
}

func TestPlanHandoffPreparationFailures(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"create", "load", "switch", "model"} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()
			u, ws := newPlanHandoffUI(t)
			failure := errors.New(stage + " failed")
			switch stage {
			case "create":
				ws.createErr = failure
			case "load":
				ws.loadErr = failure
			case "switch":
				ws.switchErr = failure
			case "model":
				ws.modelErr = failure
			}
			cmd := u.activeInline.(*dialog.PlanHandoffInline).OnConfirm(dialog.PlanHandoffOptions{NewSession: true, YOLO: true})
			u.activeInline = nil
			prepared := cmd().(planSessionPreparedMsg)
			cmds := u.applyPlanSessionPrepared(prepared)
			if prepared.err == nil {
				switched := cmds[0]().(planCoderReadyMsg)
				require.ErrorIs(t, switched.err, failure)
				u.modeSwitching = false
				cmds = u.startPlanImplementation(switched)
			}
			executePlanCommands(cmds)
			require.Equal(t, "source", u.session.ID)
			require.Equal(t, uiInputModePlan, u.mode)
			require.False(t, u.modeSwitching)
			require.Nil(t, u.planImplementation)
			require.NotEmpty(t, u.planReadyText)
			require.True(t, u.activeInline.(*dialog.PlanHandoffInline).NewSession)
			require.Empty(t, ws.runPrompts)
			require.Empty(t, ws.selected)
			require.False(t, ws.yolo, "restore permissions after a failed switch")
			if stage != "create" {
				require.Equal(t, []string{"implementation-1"}, ws.deleted)
			}
			if stage == "model" {
				require.Equal(t, config.AgentPlan, ws.setMainCalledWith)
			}
		})
	}
}

func TestPlanHandoffIgnoresStalePreparation(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"prepare", "queued switch", "switch"} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()
			u, ws := newPlanHandoffUI(t)
			cmd := u.activeInline.(*dialog.PlanHandoffInline).OnConfirm(dialog.PlanHandoffOptions{NewSession: true})
			u.activeInline = nil
			prepared := cmd().(planSessionPreparedMsg)
			var switched planCoderReadyMsg
			var switchCmd tea.Cmd
			if stage != "prepare" {
				cmds := u.applyPlanSessionPrepared(prepared)
				switchCmd = cmds[0]
				if stage == "switch" {
					switched = switchCmd().(planCoderReadyMsg)
				}
			}
			u.session = &session.Session{ID: "other"}
			u.setPlanReadyPending("")
			var cmds []tea.Cmd
			if stage == "prepare" {
				cmds = u.applyPlanSessionPrepared(prepared)
			} else {
				if stage == "queued switch" {
					switched = switchCmd().(planCoderReadyMsg)
					require.ErrorIs(t, switched.err, context.Canceled)
					require.Empty(t, ws.setMainCalledWith)
				}
				cmds = u.startPlanImplementation(switched)
			}
			executePlanCommands(cmds)
			require.Equal(t, "other", u.session.ID)
			require.Empty(t, ws.runPrompts)
			require.Empty(t, ws.selected)
			require.Equal(t, []string{"implementation-1"}, ws.deleted)
		})
	}
}

func TestPlanHandoffDuplicateCompletionPreservesChoice(t *testing.T) {
	t.Parallel()
	u, _ := newPlanHandoffUI(t)
	rc := notify.RunComplete{SessionID: "source", MessageID: "first", Text: "Same plan.\n" + common.PlanReadyMarker}
	u.handlePlanHandoff(rc)
	inline := u.activeInline.(*dialog.PlanHandoffInline)
	inline.NewSession = true
	u.handlePlanHandoff(rc)
	require.Same(t, inline, u.activeInline)
	require.True(t, inline.NewSession)
	rc.MessageID = "second"
	u.handlePlanHandoff(rc)
	require.NotSame(t, inline, u.activeInline)
	require.False(t, u.activeInline.(*dialog.PlanHandoffInline).NewSession)
}

func TestPlanHandoffSupersededPlanCannotLaunchAfterReturningToSource(t *testing.T) {
	t.Parallel()
	u, ws := newPlanHandoffUI(t)
	cmd := u.activeInline.(*dialog.PlanHandoffInline).OnConfirm(dialog.PlanHandoffOptions{NewSession: true})
	u.activeInline = nil
	prepared := cmd().(planSessionPreparedMsg)
	u.session = &session.Session{ID: "other"}
	u.setPlanReadyPending("")
	u.session = &session.Session{ID: "source"}
	u.handlePlanHandoff(notify.RunComplete{
		SessionID: "source", Text: common.PlanStartMarker + "\n# Plan\n\nDo the work.\n" + common.PlanReadyMarker,
	})
	executePlanCommands(u.applyPlanSessionPrepared(prepared))
	require.Empty(t, ws.runPrompts)
	require.Empty(t, ws.setMainCalledWith)
	require.Equal(t, []string{"implementation-1"}, ws.deleted)
	require.Equal(t, "source", u.planReadySessionID)
}

func TestPlanHandoffUsesRevisedPlanAndBlocksDuplicateLaunch(t *testing.T) {
	t.Parallel()
	u, ws := newPlanHandoffUI(t)
	u.activeInline.(*dialog.PlanHandoffInline).NewSession = true
	u.handlePlanHandoff(notify.RunComplete{
		SessionID: "source", Text: "Updated plan.\n" + common.PlanReadyMarker,
	})
	inline := u.activeInline.(*dialog.PlanHandoffInline)
	require.False(t, inline.NewSession, "a new ready plan resets the choice")
	cmd := inline.OnConfirm(dialog.PlanHandoffOptions{NewSession: true})
	require.True(t, u.modeSwitching)
	require.NotNil(t, inline.OnConfirm(dialog.PlanHandoffOptions{NewSession: true}))
	u.activeInline = nil
	prepared := cmd().(planSessionPreparedMsg)
	require.Len(t, ws.created, 1)
	cmds := u.applyPlanSessionPrepared(prepared)
	switched := cmds[0]().(planCoderReadyMsg)
	u.modeSwitching = false
	executePlanCommands(u.startPlanImplementation(switched))
	require.Equal(t, []string{"Implement the following plan:\n\nUpdated plan."}, ws.runPrompts)
}

func TestPlanHandoffSubmissionErrorKeepsNewSession(t *testing.T) {
	t.Parallel()
	u, ws := newPlanHandoffUI(t)
	ws.runErr = errors.New("submission failed")
	cmd := u.activeInline.(*dialog.PlanHandoffInline).OnConfirm(dialog.PlanHandoffOptions{NewSession: true})
	u.activeInline = nil
	cmds := u.applyPlanSessionPrepared(cmd().(planSessionPreparedMsg))
	u.modeSwitching = false
	results := executePlanCommands(u.startPlanImplementation(cmds[0]().(planCoderReadyMsg)))
	require.Equal(t, "implementation-1", u.session.ID)
	require.Empty(t, ws.deleted)
	var found bool
	for _, result := range results {
		if msg, ok := result.(util.InfoMsg); ok && msg.Type == util.InfoTypeError {
			require.Contains(t, msg.Msg, "submission failed")
			found = true
		}
	}
	require.True(t, found)
}

func TestHiddenPlanIsAbsentFromLiveAndReloadedChat(t *testing.T) {
	t.Parallel()
	u, _ := newPlanHandoffUI(t)
	hidden := message.Message{
		ID: "hidden-plan", Role: message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "Implement the following plan:\n\nSecret plan", Hidden: true}},
	}
	u.appendSessionMessage(hidden)
	require.Nil(t, u.chat.MessageItem(hidden.ID))
	u.setSessionMessages([]message.Message{hidden})
	require.Nil(t, u.chat.MessageItem(hidden.ID))
}

func TestPlanHandoffEmptyPlanCannotCreateSession(t *testing.T) {
	t.Parallel()
	u, ws := newPlanHandoffUI(t)
	u.handlePlanHandoff(notify.RunComplete{
		SessionID: "source", Text: common.PlanStartMarker + "\n" + common.PlanReadyMarker,
	})
	cmd := u.activeInline.(*dialog.PlanHandoffInline).OnConfirm(dialog.PlanHandoffOptions{NewSession: true})
	msg := cmd().(util.InfoMsg)
	require.Equal(t, util.InfoTypeError, msg.Type)
	require.Empty(t, ws.created)
	require.False(t, u.modeSwitching)
}
