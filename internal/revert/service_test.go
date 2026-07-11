package revert

import (
	"database/sql"
	"testing"

	"github.com/charmbracelet/crush/internal/history"
)

// ver builds a history.File version. An empty msgID produces a legacy row
// (NULL message_id) that must fall back to timestamp correlation.
func ver(id, path, content, msgID string, version, createdAt int64) history.File {
	m := sql.NullString{}
	if msgID != "" {
		m = sql.NullString{String: msgID, Valid: true}
	}
	return history.File{
		ID:        id,
		Path:      path,
		Content:   content,
		Version:   version,
		MessageID: m,
		CreatedAt: createdAt,
	}
}

// verNew builds a history.File version marked is_new (the agent created the
// file this turn; it did not exist before).
func verNew(id, path, content, msgID string, version, createdAt int64) history.File {
	f := ver(id, path, content, msgID, version, createdAt)
	f.IsNew = true
	return f
}

func findAction(actions []fileAction, path string) (fileAction, bool) {
	for _, a := range actions {
		if a.path == path {
			return a, true
		}
	}
	return fileAction{}, false
}

func TestPlanFileRevert(t *testing.T) {
	// cut = messages produced by the reverted turn.
	cut := map[string]struct{}{"assistant-cut": {}}
	const checkpoint int64 = 100

	t.Run("agent-edited file restores to pre-checkpoint content", func(t *testing.T) {
		versions := []history.File{
			ver("v0", "/a.go", "old", "assistant-before", 0, 90),
			ver("v1", "/a.go", "new", "assistant-cut", 1, 110),
		}
		actions := planFileRevert(versions, cut, checkpoint)
		a, ok := findAction(actions, "/a.go")
		if !ok {
			t.Fatalf("expected an action for /a.go, got %+v", actions)
		}
		if !a.restore {
			t.Errorf("expected restore, got delete")
		}
		if a.content != "old" {
			t.Errorf("restore content = %q, want %q", a.content, "old")
		}
		if len(a.versionIDs) != 1 || a.versionIDs[0] != "v1" {
			t.Errorf("versionIDs = %v, want [v1]", a.versionIDs)
		}
	})

	t.Run("agent-created file is deleted", func(t *testing.T) {
		versions := []history.File{
			verNew("v0", "/new.go", "created", "assistant-cut", 0, 110),
		}
		actions := planFileRevert(versions, cut, checkpoint)
		a, ok := findAction(actions, "/new.go")
		if !ok {
			t.Fatalf("expected an action for /new.go, got %+v", actions)
		}
		if a.restore {
			t.Errorf("expected delete, got restore")
		}
		if len(a.versionIDs) != 1 || a.versionIDs[0] != "v0" {
			t.Errorf("versionIDs = %v, want [v0]", a.versionIDs)
		}
	})

	// Regression guard for the data-loss bug: a PRE-EXISTING file first touched
	// in the reverted turn has all of its versions in the cut, but its baseline
	// is NOT is_new. It must be restored to that baseline content, not deleted.
	// (This is exactly the case that deleted write.go.)
	t.Run("pre-existing file first touched is restored, not deleted", func(t *testing.T) {
		versions := []history.File{
			ver("v0", "/exist.go", "OLD", "assistant-cut", 0, 110), // baseline, not is_new
			ver("v1", "/exist.go", "NEW", "assistant-cut", 1, 111), // edit
		}
		actions := planFileRevert(versions, cut, checkpoint)
		a, ok := findAction(actions, "/exist.go")
		if !ok {
			t.Fatalf("expected an action for /exist.go, got %+v", actions)
		}
		if !a.restore {
			t.Fatalf("expected restore (not delete) for a pre-existing file")
		}
		if a.content != "OLD" {
			t.Errorf("restore content = %q, want %q", a.content, "OLD")
		}
		if len(a.versionIDs) != 2 {
			t.Errorf("versionIDs = %v, want both versions dropped", a.versionIDs)
		}
	})

	// This is the regression guard for the message-id correlation fix: a file
	// edited in an EARLIER turn whose version timestamp collides with the
	// checkpoint second must NOT be reverted, because its producing message is
	// not in the cut set. Pure timestamp correlation (created_at >= checkpoint)
	// would have wrongly swept it.
	t.Run("same-second prior-turn edit is not swept", func(t *testing.T) {
		versions := []history.File{
			ver("v0", "/b.go", "kept", "assistant-before", 0, checkpoint),
		}
		actions := planFileRevert(versions, cut, checkpoint)
		if _, ok := findAction(actions, "/b.go"); ok {
			t.Errorf("/b.go should be untouched, got action %+v", actions)
		}
	})

	t.Run("untouched file produces no action", func(t *testing.T) {
		versions := []history.File{
			ver("v0", "/c.go", "x", "assistant-before", 0, 50),
		}
		if actions := planFileRevert(versions, cut, checkpoint); len(actions) != 0 {
			t.Errorf("expected no actions, got %+v", actions)
		}
	})

	t.Run("legacy rows fall back to timestamp", func(t *testing.T) {
		versions := []history.File{
			ver("v0", "/legacy.go", "old", "", 0, 90),  // before checkpoint
			ver("v1", "/legacy.go", "new", "", 1, 110), // at/after checkpoint
		}
		actions := planFileRevert(versions, cut, checkpoint)
		a, ok := findAction(actions, "/legacy.go")
		if !ok {
			t.Fatalf("expected an action for /legacy.go, got %+v", actions)
		}
		if !a.restore || a.content != "old" {
			t.Errorf("expected restore to %q, got restore=%v content=%q", "old", a.restore, a.content)
		}
		if len(a.versionIDs) != 1 || a.versionIDs[0] != "v1" {
			t.Errorf("versionIDs = %v, want [v1]", a.versionIDs)
		}
	})
}
