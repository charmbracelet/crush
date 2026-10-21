package tabmgr

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/split"
	"github.com/stretchr/testify/require"
)

func TestPersistenceSaveAndLoad(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := NewPersistence(dir)

	mgr := New()
	tab1 := mgr.AddTab("main", "/project", PaneSession)
	tab1.GitBranch = "main"
	tab1.GitDirty = true
	tab1.SplitFocused(split.Horizontal, PaneShell)

	mgr.AddTab("shell", "/tmp", PaneShell)

	require.NoError(t, mgr.SelectTab(0))

	// Save.
	err := p.SaveLayout(mgr)
	require.NoError(t, err)

	// Verify file exists.
	_, err = os.Stat(filepath.Join(dir, sessionFileName))
	require.NoError(t, err)

	// Load.
	layout, err := p.LoadLayout()
	require.NoError(t, err)
	require.NotNil(t, layout)
	require.Equal(t, 1, layout.Version)
	require.Equal(t, 0, layout.ActiveTab)
	require.Len(t, layout.Tabs, 2)

	// Check tab 1.
	require.Equal(t, "main", layout.Tabs[0].Name)
	require.Equal(t, "/project", layout.Tabs[0].CWD)
	require.NotNil(t, layout.Tabs[0].SplitTree)
	require.Equal(t, "H", layout.Tabs[0].SplitTree.Dir)

	// Check tab 2.
	require.Equal(t, "shell", layout.Tabs[1].Name)
}

func TestPersistenceRestore(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := NewPersistence(dir)

	// Create and save original.
	mgr := New()
	tab := mgr.AddTab("project", "/code", PaneSession)
	tab.SplitFocused(split.Vertical, PaneShell)
	mgr.AddTab("other", "/other", PaneShell)
	require.NoError(t, mgr.SelectTab(1))

	require.NoError(t, p.SaveLayout(mgr))

	// Restore.
	layout, err := p.LoadLayout()
	require.NoError(t, err)

	restored := p.RestoreTabs(layout)
	require.Equal(t, 2, restored.Len())
	require.Equal(t, 1, restored.ActiveIndex())

	// First tab should have 2 panes.
	tab0 := restored.GetTab(0)
	require.Equal(t, "project", tab0.Name)
	require.Equal(t, "/code", tab0.CWD)
	require.Equal(t, 2, tab0.PaneCount())

	// Second tab should have 1 pane.
	tab1 := restored.GetTab(1)
	require.Equal(t, "other", tab1.Name)
	require.Equal(t, 1, tab1.PaneCount())
}

func TestPersistenceLoadNonexistent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := NewPersistence(dir)

	layout, err := p.LoadLayout()
	require.NoError(t, err)
	require.Nil(t, layout)
}

func TestPersistenceLoadCorrupt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := NewPersistence(dir)

	os.WriteFile(filepath.Join(dir, sessionFileName), []byte("{invalid json"), 0o644)

	_, err := p.LoadLayout()
	require.Error(t, err)
}

func TestPersistenceLoadWrongVersion(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := NewPersistence(dir)

	os.WriteFile(filepath.Join(dir, sessionFileName), []byte(`{"version":99}`), 0o644)

	_, err := p.LoadLayout()
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported")
}

func TestPersistencePaneTypes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := NewPersistence(dir)

	mgr := New()
	tab := mgr.AddTab("test", "/test", PaneSession)
	tab.SplitFocused(split.Horizontal, PaneShell)

	require.NoError(t, p.SaveLayout(mgr))

	layout, err := p.LoadLayout()
	require.NoError(t, err)

	// Check that pane types are preserved.
	tree := layout.Tabs[0].SplitTree
	require.Equal(t, "session", tree.A.PaneType)
	require.Equal(t, "shell", tree.B.PaneType)

	// Restore and verify.
	restored := p.RestoreTabs(layout)
	tab0 := restored.GetTab(0)
	for _, meta := range tab0.Panes {
		if meta.Type == PaneShell {
			require.Equal(t, PaneShell, meta.Type)
		}
	}
}

func TestPersistenceSessionAndModelFields(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := NewPersistence(dir)

	mgr := New()
	tab := mgr.AddTab("project", "/code", PaneSession)

	// Inject session/model metadata directly into the pane meta.
	for _, meta := range tab.Panes {
		meta.SessionID = "sess-abc123"
		meta.ModelProvider = "anthropic"
		meta.ModelID = "claude-opus-4-5"
	}

	require.NoError(t, p.SaveLayout(mgr))

	layout, err := p.LoadLayout()
	require.NoError(t, err)
	require.NotNil(t, layout)

	// Verify the serialized node has the fields.
	tree := layout.Tabs[0].SplitTree
	require.Equal(t, "sess-abc123", tree.SessionID)
	require.Equal(t, "anthropic", tree.ModelProvider)
	require.Equal(t, "claude-opus-4-5", tree.ModelID)

	// Restore and verify the fields come back on PaneMeta.
	restored := p.RestoreTabs(layout)
	tab0 := restored.GetTab(0)
	for _, meta := range tab0.Panes {
		require.Equal(t, "sess-abc123", meta.SessionID)
		require.Equal(t, "anthropic", meta.ModelProvider)
		require.Equal(t, "claude-opus-4-5", meta.ModelID)
	}
}

func TestSerializeDeserializeRoundTrip(t *testing.T) {
	t.Parallel()

	// Build a complex tree: H(session, V(shell, session))
	root := split.NewSplit(split.Horizontal, 0.6,
		split.NewLeaf("p1"),
		split.NewSplit(split.Vertical, 0.4,
			split.NewLeaf("p2"),
			split.NewLeaf("p3"),
		),
	)
	panes := map[string]*PaneMeta{
		"p1": {ID: "p1", Type: PaneSession},
		"p2": {ID: "p2", Type: PaneShell},
		"p3": {ID: "p3", Type: PaneSession},
	}

	// Serialize.
	sn := serializeNode(root, panes)
	require.Equal(t, "H", sn.Dir)
	require.InDelta(t, 0.6, sn.Ratio, 0.001)
	require.Equal(t, "session", sn.A.PaneType)
	require.Equal(t, "V", sn.B.Dir)

	// Deserialize.
	restored, restoredPanes := deserializeNode(sn)
	require.Equal(t, 3, split.LeafCount(restored))
	require.Len(t, restoredPanes, 3)
	require.Equal(t, PaneSession, restoredPanes["p1"].Type)
	require.Equal(t, PaneShell, restoredPanes["p2"].Type)
	require.Equal(t, PaneSession, restoredPanes["p3"].Type)
}
