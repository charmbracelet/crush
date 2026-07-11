package memory

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStoreMemoryLifecycle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dir := t.TempDir()
	store, err := Open(ctx, Options{
		Directory:             dir,
		AutoApproveConfidence: 0.90,
		MaxRecall:             5,
		MaxIndexEntries:       20,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })

	project := Project{ID: "project-1", Name: "example", Root: t.TempDir()}
	record, err := store.SaveObservation(ctx, project, Observation{
		Scope:           ScopeGlobal,
		Kind:            KindFeedback,
		Name:            "Testing policy",
		Description:     "Integration tests use real database instances",
		Content:         "Use a real database for integration tests instead of mocks.",
		Confidence:      0.95,
		SourceSessionID: "session-1",
		SourceMessageID: "message-1",
		SourceKind:      "recorder",
	})
	require.NoError(t, err)
	require.Equal(t, StatusActive, record.Status)
	require.FileExists(t, record.FilePath)

	index, err := os.ReadFile(filepath.Join(dir, "global", "MEMORY.md"))
	require.NoError(t, err)
	require.Contains(t, string(index), "Testing policy")

	updated, err := store.SaveObservation(ctx, project, Observation{
		Scope:       ScopeGlobal,
		Kind:        KindFeedback,
		Name:        "Testing policy",
		Description: "Integration tests use real database instances",
		Content:     "Use the repository's real test database fixture, not mocks.",
		Confidence:  0.97,
		SourceKind:  "recorder",
	})
	require.NoError(t, err)
	require.Equal(t, record.ID, updated.ID)
	require.Contains(t, updated.Content, "fixture")

	pending, err := store.SaveObservation(ctx, project, Observation{
		Scope:       ScopeProject,
		Kind:        KindProject,
		Name:        "Release coordination",
		Description: "Release remains paused until verification finishes",
		Content:     "The current release is paused until the verification pass finishes.",
		Confidence:  0.70,
	})
	require.NoError(t, err)
	require.Equal(t, StatusPending, pending.Status)
	require.Empty(t, pending.FilePath)
	require.NoError(t, store.SetStatus(ctx, pending.ID, StatusActive))
	pending, err = store.Get(ctx, pending.ID)
	require.NoError(t, err)
	require.FileExists(t, pending.FilePath)

	manifest, err := store.Manifest(ctx, project.ID)
	require.NoError(t, err)
	require.Len(t, manifest, 2)
	require.Empty(t, manifest[0].Content)

	require.NoError(t, store.SetPinned(ctx, updated.ID, true))
	require.NoError(t, store.RecordRetrieval(ctx, Retrieval{
		SessionID: "session-1",
		ProjectID: project.ID,
		Query:     "How should tests access the database?",
		Selected:  []string{updated.ID},
		Available: len(manifest),
	}))
	updated, err = store.Get(ctx, updated.ID)
	require.NoError(t, err)
	require.True(t, updated.Pinned)
	require.EqualValues(t, 1, updated.RecallCount)

	require.NoError(t, store.AdvanceRecorderCursor(ctx, "session-1", Cursor{CreatedAt: 123, MessageID: "message-1"}))
	cursor, err := store.RecorderCursor(ctx, "session-1")
	require.NoError(t, err)
	require.Equal(t, Cursor{CreatedAt: 123, MessageID: "message-1"}, cursor)

	stats, err := store.Stats(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 2, stats.Active)
}

func TestSessionRecordingMode(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, err := Open(ctx, Options{Directory: t.TempDir()})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })

	mode, err := store.SessionRecordingMode(ctx, "session-1")
	require.NoError(t, err)
	require.Equal(t, SessionRecordingEnabled, mode)

	require.NoError(t, store.SetSessionRecordingMode(ctx, "session-1", SessionRecordingDisabled))
	require.NoError(t, store.MarkSessionExternalContext(ctx, "session-1"))
	mode, err = store.SessionRecordingMode(ctx, "session-1")
	require.NoError(t, err)
	require.Equal(t, SessionRecordingDisabled, mode, "external context must not override an explicit opt-out")

	require.NoError(t, store.SetSessionRecordingMode(ctx, "session-1", SessionRecordingEnabled))
	require.NoError(t, store.MarkSessionExternalContext(ctx, "session-1"))
	mode, err = store.SessionRecordingMode(ctx, "session-1")
	require.NoError(t, err)
	require.Equal(t, SessionRecordingPolluted, mode)
}

func TestStoreRejectsUnsafeOrDerivableObservations(t *testing.T) {
	t.Parallel()

	store, err := Open(context.Background(), Options{Directory: t.TempDir()})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	project := Project{ID: "project-1"}

	_, err = store.SaveObservation(context.Background(), project, Observation{
		Scope:       ScopeGlobal,
		Kind:        KindReference,
		Name:        "Provider credential",
		Description: "LM Studio access credential",
		Content:     "api_key = abcdefghijklmnopqrstuvwxyz",
		Confidence:  1,
	})
	require.ErrorIs(t, err, ErrRejected)

	_, err = store.SaveObservation(context.Background(), project, Observation{
		Scope:       ScopeProject,
		Kind:        KindProject,
		Name:        "Source layout",
		Description: "The agent package is under internal agent",
		Content:     "The implementation is in internal/agent/agent.go.",
		Confidence:  1,
		Derivable:   true,
	})
	require.ErrorIs(t, err, ErrRejected)
}

func TestImportJSONL(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, err := Open(ctx, Options{Directory: t.TempDir()})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })

	input := strings.NewReader("" +
		`{"type":"entity","name":"Preferred response style","entityType":"preference","observations":["Keep routine status updates concise."]}` + "\n" +
		`{"type":"relation","from":"a","to":"b","relationType":"uses"}` + "\n" +
		`not-json` + "\n")
	result, err := store.ImportJSONL(ctx, input, Project{ID: "project-1"})
	require.NoError(t, err)
	require.Equal(t, 1, result.Imported)
	require.Equal(t, 2, result.Skipped)

	records, err := store.List(ctx, "project-1", StatusActive, 10)
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, KindFeedback, records[0].Kind)
	require.Equal(t, ScopeGlobal, records[0].Scope)
}

func TestProjectionEditsReconcileAndDeletionForgets(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, err := Open(ctx, Options{Directory: t.TempDir()})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	project := Project{ID: "project-1"}

	record, err := store.SaveObservation(ctx, project, Observation{
		Scope:       ScopeGlobal,
		Kind:        KindFeedback,
		Name:        "Response style",
		Description: "Routine responses should remain concise",
		Content:     "Keep routine responses concise.",
		Confidence:  0.95,
	})
	require.NoError(t, err)

	data, err := os.ReadFile(record.FilePath)
	require.NoError(t, err)
	data = []byte(strings.Replace(string(data), "Keep routine responses concise.", "Keep routine responses concise and factual.", 1))
	require.NoError(t, os.WriteFile(record.FilePath, data, 0o600))
	require.NoError(t, store.SyncFromDisk(ctx, project))

	record, err = store.Get(ctx, record.ID)
	require.NoError(t, err)
	require.Equal(t, "Keep routine responses concise and factual.", record.Content)

	require.NoError(t, os.Remove(record.FilePath))
	require.NoError(t, store.SyncFromDisk(ctx, project))
	record, err = store.Get(ctx, record.ID)
	require.NoError(t, err)
	require.Equal(t, StatusDeleted, record.Status)
}

func TestNormalizeObservationRejectsLowSignalReaction(t *testing.T) {
	t.Parallel()

	_, err := NormalizeObservation(Observation{
		Scope:       ScopeGlobal,
		Kind:        KindFeedback,
		Name:        "Reaction",
		Description: "User reaction",
		Content:     "thanks",
		Confidence:  1,
	}, Project{}, 0.88)
	require.True(t, errors.Is(err, ErrRejected))
}

func TestReplacingMemoryRemovesOldProjectionWithoutReactivation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, err := Open(ctx, Options{Directory: t.TempDir()})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	project := Project{ID: "project-1"}

	original, err := store.SaveObservation(ctx, project, Observation{
		Scope:       ScopeGlobal,
		Kind:        KindFeedback,
		Name:        "Test database preference",
		Description: "Integration tests should use the repository test database",
		Content:     "Use the repository test database for integration coverage.",
		Confidence:  0.96,
		Explicit:    true,
	})
	require.NoError(t, err)
	require.FileExists(t, original.FilePath)

	replacement, err := store.SaveObservation(ctx, project, Observation{
		Scope:       ScopeGlobal,
		Kind:        KindFeedback,
		Name:        "Test database policy",
		Description: "Integration tests must use an isolated real database",
		Content:     "Use an isolated real database for integration coverage.",
		Confidence:  0.98,
		Explicit:    true,
		ReplacesID:  original.ID,
	})
	require.NoError(t, err)
	require.NotEqual(t, original.ID, replacement.ID)
	require.NoFileExists(t, original.FilePath)

	require.NoError(t, store.SyncFromDisk(ctx, project))
	original, err = store.Get(ctx, original.ID)
	require.NoError(t, err)
	require.Equal(t, StatusSuperseded, original.Status)
}

func TestProjectionWithoutIDIsRewrittenWithCanonicalID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dir := t.TempDir()
	store, err := Open(ctx, Options{Directory: dir})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	project := Project{ID: "project-1"}
	globalDir := filepath.Join(dir, "global")
	require.NoError(t, os.MkdirAll(globalDir, 0o700))
	path := filepath.Join(globalDir, "feedback_manual.md")
	projection := "---\n" +
		"name: Response format\n" +
		"description: Routine updates should state only concrete progress\n" +
		"type: feedback\n" +
		"scope: global\n" +
		"confidence: 1\n" +
		"pinned: false\n" +
		"updated_at: 2026-07-11T00:00:00Z\n" +
		"---\n\n" +
		"Keep routine progress updates concrete and concise.\n"
	require.NoError(t, os.WriteFile(path, []byte(projection), 0o600))

	require.NoError(t, store.SyncFromDisk(ctx, project))
	records, err := store.List(ctx, project.ID, StatusActive, 10)
	require.NoError(t, err)
	require.Len(t, records, 1)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(data), "id: "+records[0].ID)
}

func TestBackupRetentionAndCorruptionRecovery(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dir := t.TempDir()
	store, err := Open(ctx, Options{Directory: dir, MaxBackups: 2})
	require.NoError(t, err)
	project := Project{ID: "project-1"}
	record, err := store.SaveObservation(ctx, project, Observation{
		Scope:       ScopeGlobal,
		Kind:        KindUser,
		Name:        "User role",
		Description: "The user is learning Go agent architecture",
		Content:     "Explain unfamiliar Go agent architecture with concrete examples.",
		Confidence:  1,
		Explicit:    true,
	})
	require.NoError(t, err)
	for range 3 {
		_, err = store.Backup(ctx)
		require.NoError(t, err)
		time.Sleep(time.Millisecond)
	}
	backups, err := listBackups(filepath.Join(dir, "backups"))
	require.NoError(t, err)
	require.Len(t, backups, 2)
	require.NoError(t, store.Close())

	require.NoError(t, os.WriteFile(filepath.Join(dir, "memory.db"), []byte("corrupt"), 0o600))
	recovered, err := Open(ctx, Options{Directory: dir, MaxBackups: 2})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, recovered.Close()) })
	got, err := recovered.Get(ctx, record.ID)
	require.NoError(t, err)
	require.Equal(t, record.Content, got.Content)
	corruptFiles, err := os.ReadDir(filepath.Join(dir, "corrupt"))
	require.NoError(t, err)
	require.NotEmpty(t, corruptFiles)
}
