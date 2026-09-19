package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/plan"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSaveReadyPlanKeepsEachVersion(t *testing.T) {
	t.Parallel()
	workingDir := t.TempDir()
	complete := notify.RunComplete{
		SessionID: uuid.NewString(),
		MessageID: uuid.NewString(),
		Text:      plan.StartMarker + "\n# First plan\n\nA step.\n" + plan.ReadyMarker,
	}

	firstPath, err := saveReadyPlan(workingDir, complete)
	require.NoError(t, err)
	require.Regexp(t, `^\.crush/plans/\d{4}-\d{2}-\d{2}-\d{6}-first-plan\.md$`, firstPath)
	firstContent, err := os.ReadFile(filepath.Join(workingDir, firstPath))
	require.NoError(t, err)
	require.Equal(t, "# First plan\n\nA step.\n", string(firstContent))

	// Re-saving the same plan within the same second reuses its file.
	repeatPath, err := saveReadyPlan(workingDir, complete)
	require.NoError(t, err)
	require.Equal(t, firstPath, repeatPath)

	complete.MessageID = uuid.NewString()
	complete.Text = plan.StartMarker + "\n# Revised plan\n" + plan.ReadyMarker
	secondPath, err := saveReadyPlan(workingDir, complete)
	require.NoError(t, err)
	require.NotEqual(t, firstPath, secondPath)
	secondContent, err := os.ReadFile(filepath.Join(workingDir, secondPath))
	require.NoError(t, err)
	require.Equal(t, "# Revised plan\n", string(secondContent))
	require.Equal(t, "# First plan\n\nA step.\n", string(firstContent))
}

func TestSaveReadyPlanDisambiguatesSameTitleAndSecond(t *testing.T) {
	t.Parallel()
	workingDir := t.TempDir()
	sessionID := uuid.NewString()
	first := notify.RunComplete{
		SessionID: sessionID,
		MessageID: uuid.NewString(),
		Text:      "# Same title\n\nFirst body.\n" + plan.ReadyMarker,
	}
	firstPath, err := saveReadyPlan(workingDir, first)
	require.NoError(t, err)

	// A different plan saved in the same second would collide on the
	// timestamp+title name; it must get a numeric suffix instead of
	// overwriting the first.
	second := notify.RunComplete{
		SessionID: sessionID,
		MessageID: uuid.NewString(),
		Text:      "# Same title\n\nSecond body.\n" + plan.ReadyMarker,
	}
	secondPath, err := saveReadyPlan(workingDir, second)
	require.NoError(t, err)
	require.Equal(t, strings.TrimSuffix(firstPath, ".md")+"-2.md", secondPath)

	firstContent, err := os.ReadFile(filepath.Join(workingDir, firstPath))
	require.NoError(t, err)
	require.Equal(t, "# Same title\n\nFirst body.\n", string(firstContent))
	secondContent, err := os.ReadFile(filepath.Join(workingDir, secondPath))
	require.NoError(t, err)
	require.Equal(t, "# Same title\n\nSecond body.\n", string(secondContent))
}

func TestSaveReadyPlanDoesNotOverwriteDifferentContent(t *testing.T) {
	t.Parallel()
	workingDir := t.TempDir()
	complete := notify.RunComplete{
		SessionID: uuid.NewString(),
		MessageID: uuid.NewString(),
		Text:      "# Original\n" + plan.ReadyMarker,
	}
	path, err := saveReadyPlan(workingDir, complete)
	require.NoError(t, err)

	// Same message, different content: the original file must survive and a
	// new file must be written alongside it.
	complete.Text = "# Changed\n" + plan.ReadyMarker
	newPath, err := saveReadyPlan(workingDir, complete)
	require.NoError(t, err)
	require.NotEqual(t, path, newPath)
	content, err := os.ReadFile(filepath.Join(workingDir, path))
	require.NoError(t, err)
	require.Equal(t, "# Original\n", string(content))
	newContent, err := os.ReadFile(filepath.Join(workingDir, newPath))
	require.NoError(t, err)
	require.Equal(t, "# Changed\n", string(newContent))
}

func TestPlanRunCompletePublisherSkipsIncompleteRuns(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		text     string
		errorMsg string
		cancel   bool
	}{
		{name: "intermediate", text: "Still planning"},
		{name: "marker in prose", text: "Will emit " + plan.ReadyMarker + " later"},
		{name: "errored", text: "# Plan\n" + plan.ReadyMarker, errorMsg: "provider error"},
		{name: "cancelled", text: "# Plan\n" + plan.ReadyMarker, cancel: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			workingDir := t.TempDir()
			runs := pubsub.NewBroker[notify.RunComplete]()
			notices := pubsub.NewBroker[notify.Notification]()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			runEvents := runs.Subscribe(ctx)
			noticeEvents := notices.Subscribe(ctx)
			publisher := &planRunCompletePublisher{workingDir: workingDir, notify: notices, downstream: runs}
			complete := notify.RunComplete{
				SessionID: uuid.NewString(), MessageID: uuid.NewString(),
				Text: tc.text, Error: tc.errorMsg, Cancelled: tc.cancel,
			}
			publisher.PublishMustDeliver(ctx, pubsub.UpdatedEvent, complete)
			require.Equal(t, complete, (<-runEvents).Payload)
			select {
			case notice := <-noticeEvents:
				t.Fatalf("unexpected plan notice: %+v", notice)
			default:
			}
			_, err := os.Stat(filepath.Join(workingDir, ".crush", "plans"))
			require.ErrorIs(t, err, os.ErrNotExist)
		})
	}
}

func TestPlanRunCompletePublisherReportsSaveResult(t *testing.T) {
	t.Parallel()
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[fail], func(t *testing.T) {
			t.Parallel()
			workingDir := t.TempDir()
			if fail {
				require.NoError(t, os.WriteFile(filepath.Join(workingDir, ".crush"), []byte("occupied"), 0o600))
			}
			runs := pubsub.NewBroker[notify.RunComplete]()
			notices := pubsub.NewBroker[notify.Notification]()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			runEvents := runs.Subscribe(ctx)
			noticeEvents := notices.Subscribe(ctx)
			publisher := &planRunCompletePublisher{workingDir: workingDir, notify: notices, downstream: runs}
			complete := notify.RunComplete{
				SessionID: uuid.NewString(), MessageID: uuid.NewString(),
				Text: plan.StartMarker + "\n# Plan\n" + plan.ReadyMarker,
			}
			publisher.PublishMustDeliver(ctx, pubsub.UpdatedEvent, complete)
			notice := (<-noticeEvents).Payload
			require.Equal(t, complete.SessionID, notice.SessionID)
			if fail {
				require.Equal(t, notify.TypePlanSaveError, notice.Type)
				require.Contains(t, notice.Message, "create plan directory")
			} else {
				require.Equal(t, notify.TypePlanSaved, notice.Type)
				require.FileExists(t, filepath.Join(workingDir, notice.Message))
			}
			require.Equal(t, complete, (<-runEvents).Payload)
		})
	}
}
