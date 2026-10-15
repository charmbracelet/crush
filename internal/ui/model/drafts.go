package model

// Unsent editor content per session, so switching session or workspace never
// loses what the user was typing.
//
// In memory (text + attachments) the draft of every session left behind lives
// in UI.drafts, keyed by session ID ("" = the landing / new-session screen);
// the one on screen lives in the editor. The TEXT of non-empty drafts is
// written to <data dir>/drafts.json (attachments are never persisted), by a
// debounced save a couple of seconds after the last change (a tea.Cmd, see
// trackDraftChange) and once more by SaveDrafts after the program has exited.
// Disk IO happens only in New (constructor carve-out, see workspace_cache.go),
// in those Cmds, and in SaveDrafts.

import (
	"encoding/json"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
)

const draftsFileName = "drafts.json"

// draft is the unsent editor content of one session.
type draft struct {
	text        string
	attachments []message.Attachment
}

// draftKey is the key of the draft on screen.
func (m *UI) draftKey() string {
	if m.hasSession() {
		return m.session.ID
	}
	return ""
}

// editorDraftText là chữ của bản nháp đang hiện (thêm "!" ở đầu khi ở bang
// mode). Đang duyệt lịch prompt bằng ↑ thì ô nhập hiện một mục lịch sử chứ không
// phải bản nháp của người dùng; bản nháp thật nằm ở promptHistory.draft, và đó
// mới là thứ cần cất.
func (m *UI) editorDraftText() string {
	if m.promptHistory.index >= 0 {
		return m.promptHistory.draft
	}
	text := m.textarea.Value()
	if m.bangMode {
		text = "!" + text
	}
	return text
}

// stashDraft moves the editor content into the draft of the session on screen.
// Call it before m.session changes. It never sends or cancels anything.
func (m *UI) stashDraft() {
	d := draft{text: m.editorDraftText(), attachments: slices.Clone(m.attachments.List())}
	if d.text == "" && len(d.attachments) == 0 {
		delete(m.drafts, m.draftKey())
		return
	}
	if m.drafts == nil {
		m.drafts = make(map[string]draft)
	}
	m.drafts[m.draftKey()] = d
}

// restoreDraft fills the editor from the draft of the session on screen
// (empty when it has none). Call it after m.session changed.
func (m *UI) restoreDraft() {
	d := m.drafts[m.draftKey()]
	prevHeight := m.textarea.Height()
	if m.bangMode {
		m.bangMode = false
		m.setEditorPrompt(m.yoloModeCached())
	}
	m.textarea.SetValue(d.text)
	m.checkBangModeAfterPaste()
	m.attachments.Reset()
	for _, a := range d.attachments {
		m.attachments.Update(a)
	}
	m.closeCompletions()
	m.handleTextareaHeightChange(prevHeight)
	// Ô nhập vừa nhận nội dung của session khác: lần duyệt lịch (↑) dở dang của
	// session cũ không còn nghĩa, và editorDraftText sẽ cất nhầm bản nháp cũ.
	m.historyReset()
}

// draftsPathFor is where a workspace keeps its drafts ("" when unknown).
func draftsPathFor(cfg *config.Config) string {
	if cfg == nil || cfg.Options == nil || cfg.Options.DataDirectory == "" {
		return ""
	}
	return filepath.Join(cfg.Options.DataDirectory, draftsFileName)
}

// draftsDisk giữ phần drafts.json mà UI này đã đọc hoặc đã ghi (known), để mỗi
// lần ghi chỉ áp thay đổi của chính nó lên tệp (xem write). Luồng Update, Cmd
// ghi nền và SaveDrafts cùng dùng nó nên có mutex riêng; giữa các tiến trình thì
// khoá tệp.
//
// Mỗi lần ghi mang số thứ tự của ảnh chụp chữ nó ghi (snapshot): ảnh chụp cũ
// đến muộn, sau khi một ảnh chụp mới hơn đã nằm trên đĩa (Cmd chạy chậm hơn lần
// thoát sạch), thì bị bỏ thay vì đè bản mới bằng bản cũ.
type draftsDisk struct {
	mu      sync.Mutex
	known   map[string]string
	applied uint64 // Số thứ tự của ảnh chụp mới nhất đã xử lý (dưới mu).

	issued atomic.Uint64
}

// snapshot cấp số thứ tự cho một ảnh chụp chữ sắp ghi. Gọi ngay lúc chụp.
func (d *draftsDisk) snapshot() uint64 { return d.issued.Add(1) }

// disk trả kho ghi của UI này, tạo lúc cần đến.
func (m *UI) disk() *draftsDisk {
	if m.draftsDisk == nil {
		m.draftsDisk = &draftsDisk{}
	}
	return m.draftsDisk
}

// loadDrafts reads drafts.json into the in-memory drafts. A missing or
// corrupt file means no drafts; the file is left alone until a save has
// something different to write.
func (m *UI) loadDrafts() {
	disk := m.disk()
	disk.mu.Lock()
	disk.known = nil
	disk.mu.Unlock()
	if m.draftsPath == "" {
		return
	}
	raw, err := os.ReadFile(m.draftsPath)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("Failed to read drafts", "error", err)
		}
		return
	}
	var texts map[string]string
	if err := json.Unmarshal(raw, &texts); err != nil {
		slog.Warn("Ignoring corrupt drafts file", "path", m.draftsPath, "error", err)
		return
	}
	disk.mu.Lock()
	disk.known = texts
	disk.mu.Unlock()
	for id, text := range texts {
		if text != "" {
			if m.drafts == nil {
				m.drafts = make(map[string]draft)
			}
			m.drafts[id] = draft{text: text}
		}
	}
	m.pruneStaleDrafts()
}

// sessionListKnown reports whether a session list has landed (or been set), so
// "not in the list" means "no longer exists" and not "not fetched yet".
func (m *UI) sessionListKnown() bool {
	return m.sessionList != nil || m.sessionListVersion > 0
}

// pruneStaleDrafts drops, once, the drafts loaded from disk whose session no
// longer exists (deleted from another TUI or the CLI while this one was not
// running). It needs the session list, which is not known when New loads the
// file, so applySessionList calls it again on the first list that lands. The
// landing draft ("") and the session on screen stay. The dropped keys are still
// in the disk baseline, so the next save also removes them from the file.
func (m *UI) pruneStaleDrafts() {
	if m.draftsPruned || !m.sessionListKnown() {
		return
	}
	m.draftsPruned = true
	for id := range m.drafts {
		if id != "" && id != m.currentSessionID() && !m.inSessionList(id) {
			delete(m.drafts, id)
		}
	}
}

// draftTexts là chữ của mọi bản nháp khác rỗng, kể cả cái đang nằm trong ô
// nhập: đúng thứ sẽ được ghi xuống đĩa. Không đổi trạng thái UI, nên an toàn để
// gọi trước khi giao việc ghi cho một Cmd (map trả về là bản riêng).
func (m *UI) draftTexts() map[string]string {
	texts := make(map[string]string, len(m.drafts)+1)
	for id, d := range m.drafts {
		if d.text != "" {
			texts[id] = d.text
		}
	}
	if text := m.editorDraftText(); text != "" {
		texts[m.draftKey()] = text
	} else {
		delete(texts, m.draftKey())
	}
	return texts
}

// SaveDrafts merges this UI's changes into drafts.json, atomically: under a
// file lock it re-reads the file and applies only the keys that differ from
// what this UI loaded or last wrote (set if changed, delete if it was there
// and is gone), so another TUI on the same workspace keeps its drafts and a
// draft it already sent is not resurrected. It does blocking IO: call it after
// the program has exited, never from Update.
func (m *UI) SaveDrafts() error {
	if m.draftsPath == "" {
		return nil
	}
	disk := m.disk()
	seq := disk.snapshot()
	return disk.write(m.draftsPath, seq, m.draftTexts())
}

// write merges texts (snapshot number seq) into the file at path and remembers
// texts as what this UI has on disk. It does nothing when they already match or
// when a newer snapshot has been written already.
func (d *draftsDisk) write(path string, seq uint64, texts map[string]string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if seq < d.applied {
		return nil
	}
	if maps.Equal(texts, d.known) {
		d.applied = seq
		return nil
	}
	unlock, err := lockDraftsFile(path + ".lock")
	if err != nil {
		return err
	}
	defer unlock()

	merged := map[string]string{}
	if raw, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(raw, &merged); err != nil {
			// Keep the unreadable file aside; only our own changes survive.
			bad := path + ".corrupt-" + strconv.FormatInt(time.Now().Unix(), 10)
			if err := os.Rename(path, bad); err != nil {
				return err
			}
			merged = map[string]string{}
		}
		// Tệp chứa đúng chữ "null" giải mã thành map nil: không ghi vào đó được.
		if merged == nil {
			merged = map[string]string{}
		}
	}
	for id, text := range texts {
		if text != d.known[id] {
			merged[id] = text
		}
	}
	for id := range d.known {
		if _, ok := texts[id]; !ok {
			delete(merged, id)
		}
	}
	raw, err := json.Marshal(merged)
	if err != nil {
		return err
	}
	tmp := path + ".tmp." + strconv.Itoa(os.Getpid())
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		// Ghi dở (hết chỗ, lỗi đĩa) có thể để lại tệp tạm: dọn đi.
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	// Our view, not the merged file: the next save diffs only our changes.
	d.known, d.applied = texts, seq
	return nil
}

// Lưu bản nháp có trễ: ghi xuống đĩa sau khi người dùng ngừng đổi bản nháp một
// lúc (chứ không phải mỗi phím), để một lần thoát không sạch (đóng cửa sổ
// terminal, treo máy, mất điện) không làm mất những gì đang gõ.

// draftSaveDelay là khoảng yên lặng sau lần đổi bản nháp cuối thì mới ghi.
const draftSaveDelay = 2 * time.Second

type (
	// draftSaveTickMsg báo hết hẹn giờ lưu bản nháp.
	draftSaveTickMsg struct{}
	// draftsSavedMsg báo một lần ghi nền đã xong.
	draftsSavedMsg struct{ err error }
)

// draftSig là những gì xác định bản nháp sẽ ghi: đổi thì có gì đó để lưu.
type draftSig struct {
	key, text string
	stored    int // Số bản nháp của session khác đang giữ.
}

// draftSaver là trạng thái của việc lưu có trễ; chỉ đụng tới trên luồng Update.
type draftSaver struct {
	delay     time.Duration // 0 là draftSaveDelay; test đặt ngắn lại.
	sig       draftSig
	changedAt time.Time
	dirty     bool // Có thay đổi chưa ghi.
	armed     bool // Đang có một hẹn giờ chưa nổ.
	inFlight  bool // Đang có một lần ghi chạy ngầm.
}

func (s *draftSaver) wait() time.Duration {
	if s.delay > 0 {
		return s.delay
	}
	return draftSaveDelay
}

// affectsDraft cho biết message có thể đổi bản nháp (gõ, dán, chuyển session,
// soạn bằng editor ngoài, xoá session). Chỉ những message này làm tốn công so
// sánh chữ trong ô nhập.
func affectsDraft(msg tea.Msg) bool {
	switch msg.(type) {
	case tea.KeyPressMsg, tea.PasteMsg, loadSessionMsg, openEditorMsg, pubsub.Event[session.Session]:
		return true
	}
	return false
}

// trackDraftChange chạy ở cuối Update: nếu bản nháp đã đổi thì ghi nhớ thời
// điểm và hẹn giờ lưu (một hẹn giờ tại một thời điểm, không phải mỗi phím).
func (m *UI) trackDraftChange(msg tea.Msg) tea.Cmd {
	if m.draftsPath == "" || !affectsDraft(msg) {
		return nil
	}
	sig := draftSig{key: m.draftKey(), text: m.editorDraftText(), stored: len(m.drafts)}
	s := &m.draftSave
	if sig == s.sig {
		return nil
	}
	s.sig, s.changedAt, s.dirty = sig, time.Now(), true
	return m.armDraftSave(s.wait())
}

func (m *UI) armDraftSave(d time.Duration) tea.Cmd {
	if m.draftSave.armed {
		return nil
	}
	m.draftSave.armed = true
	return tea.Tick(d, func(time.Time) tea.Msg { return draftSaveTickMsg{} })
}

// handleDraftSaveTick xử lý hết hẹn giờ: còn đang gõ (chưa yên lặng đủ lâu) thì
// hẹn lại phần còn thiếu; yên lặng đủ rồi thì giao việc ghi cho một Cmd. Ảnh
// chụp chữ lấy ngay ở đây, trên luồng Update; Cmd chỉ giữ bản chụp ấy.
func (m *UI) handleDraftSaveTick() tea.Cmd {
	s := &m.draftSave
	s.armed = false
	if !s.dirty || m.draftsPath == "" {
		return nil
	}
	if left := s.wait() - time.Since(s.changedAt); left > 0 {
		return m.armDraftSave(left)
	}
	if s.inFlight {
		return nil // handleDraftsSaved hẹn lại vì dirty vẫn còn.
	}
	s.dirty, s.inFlight = false, true
	disk, path := m.disk(), m.draftsPath
	seq, texts := disk.snapshot(), m.draftTexts()
	return func() tea.Msg { return draftsSavedMsg{err: disk.write(path, seq, texts)} }
}

// handleDraftsSaved xử lý một lần ghi nền đã xong. Ghi hỏng thì không thử lại
// liên tục (đĩa đầy sẽ hỏng mãi): lần đổi bản nháp kế tiếp mới thử lại.
func (m *UI) handleDraftsSaved(msg draftsSavedMsg) tea.Cmd {
	s := &m.draftSave
	s.inFlight = false
	if msg.err != nil {
		slog.Warn("Failed to save drafts", "error", msg.err)
		return nil
	}
	if s.dirty { // Đổi tiếp trong lúc đang ghi.
		return m.armDraftSave(s.wait())
	}
	return nil
}
