package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

type mockPermissionService struct {
	*pubsub.Broker[permission.PermissionRequest]
}

func (m *mockPermissionService) Request(ctx context.Context, req permission.CreatePermissionRequest) (bool, error) {
	return true, nil
}

func (m *mockPermissionService) Grant(req permission.PermissionRequest) bool { return true }

func (m *mockPermissionService) Deny(req permission.PermissionRequest) bool { return true }

func (m *mockPermissionService) GrantPersistent(req permission.PermissionRequest) bool {
	return true
}

func (m *mockPermissionService) AutoApproveSession(sessionID string) {}

func (m *mockPermissionService) SetSkipRequests(skip bool) {}

func (m *mockPermissionService) SkipRequests() bool {
	return false
}

func (m *mockPermissionService) SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[permission.PermissionNotification] {
	return make(<-chan pubsub.Event[permission.PermissionNotification])
}

type mockHistoryService struct {
	*pubsub.Broker[history.File]
}

func (m *mockHistoryService) Create(ctx context.Context, sessionID, path, content string) (history.File, error) {
	return history.File{Path: path, Content: content}, nil
}

func (m *mockHistoryService) CreateVersion(ctx context.Context, sessionID, path, content string) (history.File, error) {
	return history.File{}, nil
}

func (m *mockHistoryService) GetByPathAndSession(ctx context.Context, path, sessionID string) (history.File, error) {
	return history.File{Path: path, Content: ""}, nil
}

func (m *mockHistoryService) Get(ctx context.Context, id string) (history.File, error) {
	return history.File{}, nil
}

func (m *mockHistoryService) ListBySession(ctx context.Context, sessionID string) ([]history.File, error) {
	return nil, nil
}

func (m *mockHistoryService) ListLatestSessionFiles(ctx context.Context, sessionID string) ([]history.File, error) {
	return nil, nil
}

func (m *mockHistoryService) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *mockHistoryService) DeleteSessionFiles(ctx context.Context, sessionID string) error {
	return nil
}

func TestApplyEditToContentPartialSuccess(t *testing.T) {
	t.Parallel()

	content := "line 1\nline 2\nline 3\n"

	// Test successful edit.
	newContent, err := applyEditToContent(content, MultiEditOperation{
		OldString: "line 1",
		NewString: "LINE 1",
	})
	require.NoError(t, err)
	require.Contains(t, newContent, "LINE 1")
	require.Contains(t, newContent, "line 2")

	// Test failed edit (string not found).
	_, err = applyEditToContent(content, MultiEditOperation{
		OldString: "line 99",
		NewString: "LINE 99",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestApplyEditToContentReplacementModes(t *testing.T) {
	t.Parallel()

	content := "alpha\nbeta\nalpha\n"

	newContent, err := applyEditToContent(content, MultiEditOperation{
		OldString:  "alpha",
		NewString:  "ALPHA",
		ReplaceAll: true,
	})
	require.NoError(t, err)
	require.Equal(t, "ALPHA\nbeta\nALPHA\n", newContent)

	_, err = applyEditToContent(content, MultiEditOperation{
		OldString: "alpha",
		NewString: "ALPHA",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "multiple times")

	newContent, err = applyEditToContent(content, MultiEditOperation{})
	require.NoError(t, err)
	require.Equal(t, content, newContent)
}

func TestMultiEditSequentialApplication(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create test file.
	content := "line 1\nline 2\nline 3\nline 4\n"
	err := os.WriteFile(testFile, []byte(content), 0o644)
	require.NoError(t, err)

	// Manually test the sequential application logic.
	currentContent := content

	// Apply edits sequentially, tracking failures.
	edits := []MultiEditOperation{
		{OldString: "line 1", NewString: "LINE 1"},   // Should succeed
		{OldString: "line 99", NewString: "LINE 99"}, // Should fail - doesn't exist
		{OldString: "line 3", NewString: "LINE 3"},   // Should succeed
		{OldString: "line 2", NewString: "LINE 2"},   // Should succeed - still exists
	}

	var failedEdits []FailedEdit
	successCount := 0

	for i, edit := range edits {
		newContent, err := applyEditToContent(currentContent, edit)
		if err != nil {
			failedEdits = append(failedEdits, FailedEdit{
				Index: i + 1,
				Error: err.Error(),
				Edit:  edit,
			})
			continue
		}
		currentContent = newContent
		successCount++
	}

	// Verify results.
	require.Equal(t, 3, successCount, "Expected 3 successful edits")
	require.Len(t, failedEdits, 1, "Expected 1 failed edit")

	// Check failed edit details.
	require.Equal(t, 2, failedEdits[0].Index)
	require.Contains(t, failedEdits[0].Error, "not found")

	// Verify content changes.
	require.Contains(t, currentContent, "LINE 1")
	require.Contains(t, currentContent, "LINE 2")
	require.Contains(t, currentContent, "LINE 3")
	require.Contains(t, currentContent, "line 4") // Original unchanged
	require.NotContains(t, currentContent, "LINE 99")
}

func TestMultiEditAllEditsSucceed(t *testing.T) {
	t.Parallel()

	content := "line 1\nline 2\nline 3\n"

	edits := []MultiEditOperation{
		{OldString: "line 1", NewString: "LINE 1"},
		{OldString: "line 2", NewString: "LINE 2"},
		{OldString: "line 3", NewString: "LINE 3"},
	}

	currentContent := content
	successCount := 0

	for _, edit := range edits {
		newContent, err := applyEditToContent(currentContent, edit)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		currentContent = newContent
		successCount++
	}

	require.Equal(t, 3, successCount)
	require.Contains(t, currentContent, "LINE 1")
	require.Contains(t, currentContent, "LINE 2")
	require.Contains(t, currentContent, "LINE 3")
}

func TestMultiEditAllEditsFail(t *testing.T) {
	t.Parallel()

	content := "line 1\nline 2\n"

	edits := []MultiEditOperation{
		{OldString: "line 99", NewString: "LINE 99"},
		{OldString: "line 100", NewString: "LINE 100"},
	}

	currentContent := content
	var failedEdits []FailedEdit

	for i, edit := range edits {
		newContent, err := applyEditToContent(currentContent, edit)
		if err != nil {
			failedEdits = append(failedEdits, FailedEdit{
				Index: i + 1,
				Error: err.Error(),
				Edit:  edit,
			})
			continue
		}
		currentContent = newContent
	}

	require.Len(t, failedEdits, 2)
	require.Equal(t, content, currentContent, "Content should be unchanged")
}

func TestProcessMultiEditExistingFilePartialFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("one\ntwo\nthree\n"), 0o644))

	edit := editContext{
		ctx:         context.WithValue(t.Context(), SessionIDContextKey, "session"),
		permissions: &mockPermissionService{},
		files:       &mockHistoryService{},
		filetracker: &mockEditFileTracker{lastRead: time.Now().Add(time.Second)},
		workingDir:  dir,
	}
	params := MultiEditParams{
		FilePath: filePath,
		Edits: []MultiEditOperation{
			{OldString: "two", NewString: "TWO"},
			{OldString: "missing", NewString: "MISSING"},
		},
	}

	resp, err := processMultiEditExistingFile(edit, params, fantasy.ToolCall{ID: "call"})
	require.NoError(t, err)
	require.False(t, resp.IsError)
	require.Contains(t, resp.Content, "Applied 1 of 2 edits")

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	require.Equal(t, "one\nTWO\nthree\n", string(content))

	var meta MultiEditResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
	require.Equal(t, 1, meta.EditsApplied)
	require.Len(t, meta.EditsFailed, 1)
	require.Equal(t, 2, meta.EditsFailed[0].Index)
	require.Equal(t, "one\ntwo\nthree\n", meta.OldContent)
	require.Equal(t, "one\nTWO\nthree\n", meta.NewContent)
}

func TestProcessMultiEditWithCreationPartialFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "nested", "test.txt")

	edit := editContext{
		ctx:         context.WithValue(t.Context(), SessionIDContextKey, "session"),
		permissions: &mockPermissionService{},
		files:       &mockHistoryService{},
		filetracker: &mockEditFileTracker{},
		workingDir:  dir,
	}
	params := MultiEditParams{
		FilePath: filePath,
		Edits: []MultiEditOperation{
			{OldString: "", NewString: "one\ntwo\nthree\n"},
			{OldString: "two", NewString: "TWO"},
			{OldString: "missing", NewString: "MISSING"},
		},
	}

	resp, err := processMultiEditWithCreation(edit, params, fantasy.ToolCall{ID: "call"})
	require.NoError(t, err)
	require.False(t, resp.IsError)
	require.Contains(t, resp.Content, "File created with 2 of 3 edits")

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	require.Equal(t, "one\nTWO\nthree\n", string(content))

	var meta MultiEditResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
	require.Equal(t, 2, meta.EditsApplied)
	require.Len(t, meta.EditsFailed, 1)
	require.Equal(t, 3, meta.EditsFailed[0].Index)
	require.Equal(t, "", meta.OldContent)
	require.Equal(t, "one\nTWO\nthree\n", meta.NewContent)
}
