package autopermission

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

type mockPermissionService struct {
	*pubsub.Broker[permission.PermissionRequest]

	evalResult  permission.EvaluationResult
	promptGrant bool
	promptCalls int
}

func (m *mockPermissionService) GrantPersistent(permission.PermissionRequest) {}
func (m *mockPermissionService) Grant(permission.PermissionRequest)           {}
func (m *mockPermissionService) Deny(permission.PermissionRequest)            {}
func (m *mockPermissionService) EvaluateRequest(context.Context, permission.CreatePermissionRequest) (permission.EvaluationResult, error) {
	return m.evalResult, nil
}
func (m *mockPermissionService) Prompt(context.Context, permission.PermissionRequest) (bool, error) {
	m.promptCalls++
	return m.promptGrant, nil
}
func (m *mockPermissionService) Request(context.Context, permission.CreatePermissionRequest) (bool, error) {
	return false, nil
}
func (m *mockPermissionService) AutoApproveSession(string)          {}
func (m *mockPermissionService) SetSessionAutoApprove(string, bool) {}
func (m *mockPermissionService) IsSessionAutoApprove(string) bool   { return false }
func (m *mockPermissionService) SetSkipRequests(bool)               {}
func (m *mockPermissionService) SkipRequests() bool                 { return false }
func (m *mockPermissionService) SubscribeNotifications(context.Context) <-chan pubsub.Event[permission.PermissionNotification] {
	return make(<-chan pubsub.Event[permission.PermissionNotification])
}

type mockSessionService struct {
	mode session.CollaborationMode
}

func (m *mockSessionService) Subscribe(context.Context) <-chan pubsub.Event[session.Session] {
	return make(<-chan pubsub.Event[session.Session])
}
func (m *mockSessionService) Create(context.Context, string) (session.Session, error) {
	return session.Session{}, nil
}
func (m *mockSessionService) CreateTitleSession(context.Context, string) (session.Session, error) {
	return session.Session{}, nil
}
func (m *mockSessionService) CreateTaskSession(context.Context, string, string, string) (session.Session, error) {
	return session.Session{}, nil
}
func (m *mockSessionService) CreateHandoffSession(context.Context, string, string, string, string, []string) (session.Session, error) {
	return session.Session{}, nil
}
func (m *mockSessionService) Get(context.Context, string) (session.Session, error) {
	return session.Session{CollaborationMode: m.mode}, nil
}
func (m *mockSessionService) GetLast(context.Context) (session.Session, error) {
	return session.Session{}, nil
}
func (m *mockSessionService) List(context.Context) ([]session.Session, error) { return nil, nil }
func (m *mockSessionService) Save(context.Context, session.Session) (session.Session, error) {
	return session.Session{}, nil
}
func (m *mockSessionService) UpdateCollaborationMode(context.Context, string, session.CollaborationMode) (session.Session, error) {
	return session.Session{}, nil
}
func (m *mockSessionService) UpdateTitleAndUsage(context.Context, string, string, int64, int64, float64) error {
	return nil
}
func (m *mockSessionService) Rename(context.Context, string, string) error { return nil }
func (m *mockSessionService) Delete(context.Context, string) error         { return nil }
func (m *mockSessionService) CreateAgentToolSessionID(messageID, toolCallID string) string {
	return ""
}
func (m *mockSessionService) ParseAgentToolSessionID(sessionID string) (messageID string, toolCallID string, ok bool) {
	return "", "", false
}
func (m *mockSessionService) IsAgentToolSession(string) bool { return false }

type mockClassifier struct {
	result permission.AutoClassification
	err    error
	calls  int
}

func (m *mockClassifier) ClassifyPermission(context.Context, permission.PermissionRequest) (permission.AutoClassification, error) {
	m.calls++
	return m.result, m.err
}

func TestAutoPermission_DefaultModeFallsBackToPrompt(t *testing.T) {
	t.Parallel()

	base := &mockPermissionService{
		Broker:      pubsub.NewBroker[permission.PermissionRequest](),
		evalResult:  permission.EvaluationResult{Decision: permission.EvaluationDecisionAsk, Permission: permission.PermissionRequest{SessionID: "s1", ToolName: "edit", Action: "write"}},
		promptGrant: true,
	}
	svc := New(base, &mockSessionService{mode: session.CollaborationModeDefault}, nil, "")

	granted, err := svc.Request(t.Context(), permission.CreatePermissionRequest{})
	require.NoError(t, err)
	require.True(t, granted)
	require.Equal(t, 1, base.promptCalls)
}

func TestAutoPermission_AutoModeReadOnlyRequestSkipsClassifier(t *testing.T) {
	t.Parallel()

	base := &mockPermissionService{
		Broker:      pubsub.NewBroker[permission.PermissionRequest](),
		evalResult:  permission.EvaluationResult{Decision: permission.EvaluationDecisionAsk, Permission: permission.PermissionRequest{SessionID: "s1", ToolName: "view", Action: "read"}},
		promptGrant: true,
	}
	classifier := &mockClassifier{}
	svc := New(base, &mockSessionService{mode: session.CollaborationModeAuto}, func() permission.Classifier { return classifier }, "")

	granted, err := svc.Request(t.Context(), permission.CreatePermissionRequest{})
	require.NoError(t, err)
	require.True(t, granted)
	require.Zero(t, base.promptCalls)
	require.Zero(t, classifier.calls)
}

func TestAutoPermission_AutoModeClassifierAllowSkipsPrompt(t *testing.T) {
	t.Parallel()

	base := &mockPermissionService{
		Broker:      pubsub.NewBroker[permission.PermissionRequest](),
		evalResult:  permission.EvaluationResult{Decision: permission.EvaluationDecisionAsk, Permission: permission.PermissionRequest{SessionID: "s1", ToolName: "edit", Action: "write"}},
		promptGrant: true,
	}
	classifier := &mockClassifier{result: permission.AutoClassification{AllowAuto: true}}
	svc := New(base, &mockSessionService{mode: session.CollaborationModeAuto}, func() permission.Classifier { return classifier }, "")

	granted, err := svc.Request(t.Context(), permission.CreatePermissionRequest{})
	require.NoError(t, err)
	require.True(t, granted)
	require.Zero(t, base.promptCalls)
	require.Equal(t, 1, classifier.calls)
}

func TestAutoPermission_AutoModeClassifierBlockFallsBackToPrompt(t *testing.T) {
	t.Parallel()

	base := &mockPermissionService{
		Broker:      pubsub.NewBroker[permission.PermissionRequest](),
		evalResult:  permission.EvaluationResult{Decision: permission.EvaluationDecisionAsk, Permission: permission.PermissionRequest{SessionID: "s1", ToolName: "edit", Action: "write"}},
		promptGrant: true,
	}
	classifier := &mockClassifier{result: permission.AutoClassification{AllowAuto: false}}
	svc := New(base, &mockSessionService{mode: session.CollaborationModeAuto}, func() permission.Classifier { return classifier }, "")

	granted, err := svc.Request(t.Context(), permission.CreatePermissionRequest{})
	require.NoError(t, err)
	require.True(t, granted)
	require.Equal(t, 1, base.promptCalls)
	require.Equal(t, 1, classifier.calls)
}

func TestAutoPermission_AutoModeClassifierErrorFallsBackToPrompt(t *testing.T) {
	t.Parallel()

	base := &mockPermissionService{
		Broker:      pubsub.NewBroker[permission.PermissionRequest](),
		evalResult:  permission.EvaluationResult{Decision: permission.EvaluationDecisionAsk, Permission: permission.PermissionRequest{SessionID: "s1", ToolName: "edit", Action: "write"}},
		promptGrant: true,
	}
	classifier := &mockClassifier{err: context.DeadlineExceeded}
	svc := New(base, &mockSessionService{mode: session.CollaborationModeAuto}, func() permission.Classifier { return classifier }, "")

	granted, err := svc.Request(t.Context(), permission.CreatePermissionRequest{})
	require.NoError(t, err)
	require.True(t, granted)
	require.Equal(t, 1, base.promptCalls)
	require.Equal(t, 1, classifier.calls)
}

func TestAutoPermission_SuspendsClassifierAfterRepeatedBlocks(t *testing.T) {
	t.Parallel()

	base := &mockPermissionService{
		Broker:      pubsub.NewBroker[permission.PermissionRequest](),
		evalResult:  permission.EvaluationResult{Decision: permission.EvaluationDecisionAsk, Permission: permission.PermissionRequest{SessionID: "s1", ToolName: "edit", Action: "write"}},
		promptGrant: true,
	}
	classifier := &mockClassifier{result: permission.AutoClassification{AllowAuto: false}}
	svc := New(base, &mockSessionService{mode: session.CollaborationModeAuto}, func() permission.Classifier { return classifier }, "")

	for range defaultMaxConsecutiveClassifierBlocks {
		granted, err := svc.Request(t.Context(), permission.CreatePermissionRequest{})
		require.NoError(t, err)
		require.True(t, granted)
	}

	granted, err := svc.Request(t.Context(), permission.CreatePermissionRequest{})
	require.NoError(t, err)
	require.True(t, granted)
	require.Equal(t, defaultMaxConsecutiveClassifierBlocks, classifier.calls)
	require.Equal(t, defaultMaxConsecutiveClassifierBlocks+1, base.promptCalls)
}

func TestAutoPermission_AutoModeReadOnlyBashSkipsClassifier(t *testing.T) {
	t.Parallel()

	base := &mockPermissionService{
		Broker: pubsub.NewBroker[permission.PermissionRequest](),
		evalResult: permission.EvaluationResult{
			Decision: permission.EvaluationDecisionAsk,
			Permission: permission.PermissionRequest{
				SessionID: "s1",
				ToolName:  tools.BashToolName,
				Action:    "execute",
				Params: tools.BashPermissionsParams{
					Command: "git status --short",
				},
			},
		},
		promptGrant: true,
	}
	classifier := &mockClassifier{}
	svc := New(base, &mockSessionService{mode: session.CollaborationModeAuto}, func() permission.Classifier { return classifier }, "")

	granted, err := svc.Request(t.Context(), permission.CreatePermissionRequest{})
	require.NoError(t, err)
	require.True(t, granted)
	require.Zero(t, base.promptCalls)
	require.Zero(t, classifier.calls)
}

func TestAutoPermission_AutoModeWorkspaceWriteSkipsClassifier(t *testing.T) {
	t.Parallel()

	workingDir := filepath.Join(t.TempDir(), "workspace")
	filePath := filepath.Join(workingDir, "internal", "ui", "model.go")

	base := &mockPermissionService{
		Broker: pubsub.NewBroker[permission.PermissionRequest](),
		evalResult: permission.EvaluationResult{
			Decision: permission.EvaluationDecisionAsk,
			Permission: permission.PermissionRequest{
				SessionID: "s1",
				ToolName:  tools.WriteToolName,
				Action:    "write",
				Path:      workingDir,
				Params: tools.WritePermissionsParams{
					FilePath: filePath,
				},
			},
		},
		promptGrant: true,
	}
	classifier := &mockClassifier{}
	svc := New(base, &mockSessionService{mode: session.CollaborationModeAuto}, func() permission.Classifier { return classifier }, workingDir)

	granted, err := svc.Request(t.Context(), permission.CreatePermissionRequest{})
	require.NoError(t, err)
	require.True(t, granted)
	require.Zero(t, base.promptCalls)
	require.Zero(t, classifier.calls)
}

func TestAutoPermission_AutoModeSensitiveWorkspaceWriteFallsBackToPrompt(t *testing.T) {
	t.Parallel()

	workingDir := filepath.Join(t.TempDir(), "workspace")
	filePath := filepath.Join(workingDir, "AGENTS.md")

	base := &mockPermissionService{
		Broker: pubsub.NewBroker[permission.PermissionRequest](),
		evalResult: permission.EvaluationResult{
			Decision: permission.EvaluationDecisionAsk,
			Permission: permission.PermissionRequest{
				SessionID: "s1",
				ToolName:  tools.WriteToolName,
				Action:    "write",
				Path:      workingDir,
				Params: tools.WritePermissionsParams{
					FilePath: filePath,
				},
			},
		},
		promptGrant: true,
	}
	classifier := &mockClassifier{}
	svc := New(base, &mockSessionService{mode: session.CollaborationModeAuto}, func() permission.Classifier { return classifier }, workingDir)

	granted, err := svc.Request(t.Context(), permission.CreatePermissionRequest{})
	require.NoError(t, err)
	require.True(t, granted)
	require.Equal(t, 1, base.promptCalls)
	require.Zero(t, classifier.calls)
}
