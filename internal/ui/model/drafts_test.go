package model

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
)

// draftWS records AgentRun so tests can see which session a prompt went to.
type draftWS struct {
	*countingWorkspace
	runs [][2]string // {sessionID, prompt}
}

func (w *draftWS) AgentRun(_ context.Context, id, prompt string, _ ...message.Attachment) error {
	w.runs = append(w.runs, [2]string{id, prompt})
	return nil
}

// newDraftUI builds a UI on session s1 with a focused editor.
func newDraftUI(t *testing.T) (*UI, *draftWS) {
	t.Helper()
	ws := &draftWS{countingWorkspace: &countingWorkspace{ready: true}}
	m := newBusyUI(ws.countingWorkspace)
	m.com = common.DefaultCommon(ws) // route AgentRun to the recorder
	m.status = NewStatus(m.com, nil)
	warmCaches(m, false)
	m.textarea.Focus()
	m.promptHistory.index = -1 // Như New(): chưa duyệt lịch.
	return m, ws
}

func typeText(m *UI, s string) {
	for _, r := range s {
		m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

func switchTo(m *UI, id string) {
	m.Update(loadSessionMsg{session: &session.Session{ID: id}})
}

func attNames(m *UI) []string {
	var out []string
	for _, a := range m.attachments.List() {
		out = append(out, a.FileName)
	}
	return out
}

func TestDraftSurvivesSessionSwitch(t *testing.T) {
	m, _ := newDraftUI(t)
	typeText(m, "a")
	switchTo(m, "s2")
	require.Equal(t, "", m.textarea.Value(), "s2 starts empty")
	typeText(m, "b")
	switchTo(m, "s1")
	require.Equal(t, "a", m.textarea.Value())
	switchTo(m, "s2")
	require.Equal(t, "b", m.textarea.Value())
}

func TestEnterSendsToCurrentSessionAndClearsItsDraft(t *testing.T) {
	m, ws := newDraftUI(t)
	typeText(m, "a")
	switchTo(m, "s2")
	typeText(m, "b")
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	runCmds(m, cmd)
	require.Equal(t, [][2]string{{"s2", "b"}}, ws.runs)
	require.Equal(t, "", m.textarea.Value())
	switchTo(m, "s1")
	require.Equal(t, "a", m.textarea.Value(), "s1 keeps its draft")
	switchTo(m, "s2")
	require.Equal(t, "", m.textarea.Value(), "sent draft does not come back")
	require.Len(t, ws.runs, 1, "switching never sends")
}

func TestNewSessionKeepsDraftOfLeftSession(t *testing.T) {
	m, ws := newDraftUI(t)
	typeText(m, "a")
	m.Update(tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl})
	require.Nil(t, m.session)
	require.Equal(t, "", m.textarea.Value(), "landing starts empty")
	typeText(m, "z")
	switchTo(m, "s1")
	require.Equal(t, "a", m.textarea.Value())
	require.Zero(t, ws.cancelCalls)
	require.Empty(t, ws.runs)
	m.Update(tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl})
	require.Equal(t, "z", m.textarea.Value(), "landing draft comes back")
}

func TestAttachmentsFollowSession(t *testing.T) {
	m, _ := newDraftUI(t)
	m.attachments.Update(message.Attachment{FileName: "one.txt"})
	switchTo(m, "s2")
	require.Empty(t, attNames(m), "not visible in s2")
	m.attachments.Update(message.Attachment{FileName: "two.txt"})
	switchTo(m, "s1")
	require.Equal(t, []string{"one.txt"}, attNames(m))
	switchTo(m, "s2")
	require.Equal(t, []string{"two.txt"}, attNames(m))
}

func TestDeletedSessionDropsDraft(t *testing.T) {
	m, _ := newDraftUI(t)
	switchTo(m, "s2")
	typeText(m, "b")
	switchTo(m, "s1")
	require.Contains(t, m.drafts, "s2")
	m.Update(pubsub.Event[session.Session]{Type: pubsub.DeletedEvent, Payload: session.Session{ID: "s2"}})
	require.NotContains(t, m.drafts, "s2")
}

func TestDraftsPersistAcrossRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "drafts.json")

	m1, _ := newDraftUI(t)
	m1.draftsPath = path
	typeText(m1, "keep")
	switchTo(m1, "s2") // s2 stays empty and must not be written
	m1.attachments.Update(message.Attachment{FileName: "lost.txt"})
	require.NoError(t, m1.SaveDrafts())
	st, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), st.Mode().Perm())
	raw, _ := os.ReadFile(path)
	require.JSONEq(t, `{"s1":"keep"}`, string(raw))

	m2, _ := newDraftUI(t)
	m2.session = nil // fresh process: nothing open yet
	m2.draftsPath = path
	m2.loadDrafts()
	switchTo(m2, "s1")
	require.Equal(t, "keep", m2.textarea.Value())
	require.Empty(t, attNames(m2), "attachments are not persisted")
}

func TestDraftsFileMissingOrCorruptIsHarmless(t *testing.T) {
	path := filepath.Join(t.TempDir(), "drafts.json")

	m, _ := newDraftUI(t)
	m.draftsPath = path
	m.loadDrafts() // missing
	require.NoError(t, m.SaveDrafts())
	require.NoFileExists(t, path, "nothing to save, nothing written")

	require.NoError(t, os.WriteFile(path, []byte("{not json"), 0o600))
	m, _ = newDraftUI(t)
	m.draftsPath = path
	m.loadDrafts()
	switchTo(m, "s2")
	require.Equal(t, "", m.textarea.Value())
	require.NoError(t, m.SaveDrafts())
	raw, _ := os.ReadFile(path)
	require.Equal(t, "{not json", string(raw), "corrupt file kept until a new draft is saved")

	typeText(m, "x")
	require.NoError(t, m.SaveDrafts())
	raw, _ = os.ReadFile(path)
	require.JSONEq(t, `{"s2":"x"}`, string(raw))
	kept, err := filepath.Glob(path + ".corrupt-*")
	require.NoError(t, err)
	require.Len(t, kept, 1, "unreadable file is kept aside, not lost")
	raw, _ = os.ReadFile(kept[0])
	require.Equal(t, "{not json", string(raw))
}

// draftUIOn is a UI that loaded path and sits on session id.
func draftUIOn(t *testing.T, path, id string) *UI {
	t.Helper()
	m, _ := newDraftUI(t)
	m.session = nil // fresh process: nothing open yet
	m.draftsPath = path
	m.loadDrafts()
	switchTo(m, id)
	return m
}

func readDrafts(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(raw)
}

func TestSaveDraftsKeepsOtherTUIsDrafts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "drafts.json")
	a := draftUIOn(t, path, "s1")
	b := draftUIOn(t, path, "s2") // loaded before A saved
	typeText(a, "from-a")
	typeText(b, "from-b")
	require.NoError(t, a.SaveDrafts())
	require.NoError(t, b.SaveDrafts())
	require.JSONEq(t, `{"s1":"from-a","s2":"from-b"}`, readDrafts(t, path))
	require.NoFileExists(t, path+".tmp."+strconv.Itoa(os.Getpid()))
}

func TestSaveDraftsDoesNotResurrectSentDraft(t *testing.T) {
	path := filepath.Join(t.TempDir(), "drafts.json")
	seed := draftUIOn(t, path, "s1")
	typeText(seed, "old")
	require.NoError(t, seed.SaveDrafts())

	a := draftUIOn(t, path, "s1")
	b := draftUIOn(t, path, "s3") // also loaded s1="old"
	require.Equal(t, "old", a.textarea.Value())

	_, cmd := a.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // A sends s1
	runCmds(a, cmd)
	require.NoError(t, a.SaveDrafts())
	require.JSONEq(t, `{}`, readDrafts(t, path))

	typeText(b, "new")
	require.NoError(t, b.SaveDrafts())
	require.JSONEq(t, `{"s3":"new"}`, readDrafts(t, path), "s1 stays deleted")
}

func TestSaveDraftsConcurrentKeepsEveryKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "drafts.json")
	const n = 8
	uis := make([]*UI, n)
	for i := range uis {
		uis[i] = draftUIOn(t, path, "k"+strconv.Itoa(i))
		typeText(uis[i], "v"+strconv.Itoa(i))
	}
	var wg sync.WaitGroup
	for _, m := range uis {
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.NoError(t, m.SaveDrafts())
		}()
	}
	wg.Wait()
	var got map[string]string
	require.NoError(t, json.Unmarshal([]byte(readDrafts(t, path)), &got))
	require.Len(t, got, n)
	for i := range n {
		require.Equal(t, "v"+strconv.Itoa(i), got["k"+strconv.Itoa(i)])
	}
}

// Tệp @-mention ở session này không được tính là "đã đọc" cho session
// khác: edit dựa vào dấu đã đọc theo session để chặn sửa đè tệp chưa xem.
func TestFileReadsDoNotLeakAcrossSessions(t *testing.T) {
	m, _ := newDraftUI(t)
	m.sessionFileReads = []string{"/repo/db.go"}
	switchTo(m, "s2")
	require.Empty(t, m.sessionFileReads, "s1's mentions must not be recorded as read in s2")
}

// Đang duyệt lịch prompt bằng ↑ thì ô nhập hiện một mục lịch sử, không phải bản
// nháp của người dùng: đổi session lúc đó phải cất bản nháp thật (không phải mục
// lịch sử) và trả lại nguyên vẹn khi quay về.
func TestDraftSurvivesHistoryBrowsingAcrossSessionSwitch(t *testing.T) {
	m, _ := newDraftUI(t)
	m.promptHistory.messages = []string{"older prompt", "oldest prompt"}
	m.promptHistory.index = -1
	typeText(m, "my real draft")

	// ↑ lần đầu đưa con trỏ về đầu, lần hai mới vào lịch sử.
	m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	require.Equal(t, "older prompt", m.textarea.Value())
	require.Equal(t, 0, m.promptHistory.index)

	switchTo(m, "s2")
	require.Equal(t, "", m.textarea.Value(), "s2 starts empty")
	switchTo(m, "s1")
	require.Equal(t, "my real draft", m.textarea.Value(), "the history entry must not replace the real draft")
}

// Thoát lúc đang duyệt lịch cũng ghi bản nháp thật xuống đĩa.
func TestSaveDraftsWhileBrowsingHistoryWritesRealDraft(t *testing.T) {
	path := filepath.Join(t.TempDir(), "drafts.json")
	m, _ := newDraftUI(t)
	m.draftsPath = path
	m.promptHistory.messages = []string{"older prompt"}
	m.promptHistory.index = -1
	typeText(m, "unsent work")
	m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	require.Equal(t, "older prompt", m.textarea.Value())

	require.NoError(t, m.SaveDrafts())
	require.JSONEq(t, `{"s1":"unsent work"}`, readDrafts(t, path))
}

// Tạo session từ màn landing (gửi prompt, chạy lệnh !, tin từ MCP, init) là một
// lần đổi session như mọi lần khác: bản nháp landing phải được cất trước và ô
// nhập của session mới phải rỗng. Trước đây chữ đang gõ ở landing trôi sang
// session mới.
func TestEnsureSessionStashesLandingDraft(t *testing.T) {
	m, _ := newDraftUI(t)
	m.session = nil
	typeText(m, "typed on landing")

	cmd, err := m.ensureSession()
	require.NoError(t, err)
	require.NotNil(t, cmd)
	require.Equal(t, "new", m.currentSessionID())
	require.Equal(t, "", m.textarea.Value(), "the new session starts with an empty editor")
	require.Equal(t, "typed on landing", m.drafts[""].text, "the landing draft is kept for the landing screen")

	// Về màn landing thì bản nháp đó hiện lại.
	m.Update(tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl})
	require.Nil(t, m.session)
	require.Equal(t, "typed on landing", m.textarea.Value())
}

func TestRunShellCommandWithoutSessionStashesLandingDraft(t *testing.T) {
	m, _ := newDraftUI(t)
	m.session = nil
	typeText(m, "half-typed")

	m.runShellCommandInternal("ls", true)
	require.Equal(t, "new", m.currentSessionID())
	require.Equal(t, "ls", m.pendingBangCommand)
	require.Equal(t, "", m.textarea.Value())
	require.Equal(t, "half-typed", m.drafts[""].text)
}

// Prompt đã gửi từ màn landing không hiện lại ở landing lần sau.
func TestSentLandingPromptDoesNotReappear(t *testing.T) {
	m, ws := newDraftUI(t)
	m.session = nil
	typeText(m, "send me")
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	runCmds(m, cmd)
	require.Equal(t, "new", m.currentSessionID())
	require.Equal(t, [][2]string{{"new", "send me"}}, ws.runs)

	m.Update(tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl})
	require.Nil(t, m.session)
	require.Equal(t, "", m.textarea.Value(), "the sent prompt must not come back on the landing screen")
	require.NotContains(t, m.drafts, "")
}
