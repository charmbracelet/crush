package model

// Unsent editor content per session, so switching session or workspace never
// loses what the user was typing.
//
// In memory (text + attachments) the draft of every session left behind lives
// in UI.drafts, keyed by session ID ("" = the landing / new-session screen);
// the one on screen lives in the editor. On quit SaveDrafts writes the TEXT of
// non-empty drafts to <data dir>/drafts.json; attachments are never persisted.
// Disk IO happens only in New (constructor carve-out, see workspace_cache.go)
// and in SaveDrafts, which the caller runs after the program has exited.

import (
	"encoding/json"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
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

// stashDraft moves the editor content into the draft of the session on screen.
// Call it before m.session changes. It never sends or cancels anything.
func (m *UI) stashDraft() {
	text := m.textarea.Value()
	if m.bangMode {
		text = "!" + text
	}
	d := draft{text: text, attachments: slices.Clone(m.attachments.List())}
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
}

// draftsPathFor is where a workspace keeps its drafts ("" when unknown).
func draftsPathFor(cfg *config.Config) string {
	if cfg == nil || cfg.Options == nil || cfg.Options.DataDirectory == "" {
		return ""
	}
	return filepath.Join(cfg.Options.DataDirectory, draftsFileName)
}

// loadDrafts reads drafts.json into the in-memory drafts. A missing or
// corrupt file means no drafts; the file is left alone until SaveDrafts has
// something different to write.
func (m *UI) loadDrafts() {
	m.draftsOnDisk = nil
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
	m.draftsOnDisk = texts
	for id, text := range texts {
		if text != "" {
			if m.drafts == nil {
				m.drafts = make(map[string]draft)
			}
			m.drafts[id] = draft{text: text}
		}
	}
}

// SaveDrafts merges this UI's changes into drafts.json, atomically: under a
// file lock it re-reads the file and applies only the keys that differ from
// what this UI loaded (set if changed, delete if it was loaded and is gone), so
// another TUI on the same workspace keeps its drafts and a draft it already
// sent is not resurrected. It does blocking IO: call it after the program has
// exited, never from Update.
func (m *UI) SaveDrafts() error {
	if m.draftsPath == "" {
		return nil
	}
	m.stashDraft()
	texts := make(map[string]string, len(m.drafts))
	for id, d := range m.drafts {
		if d.text != "" {
			texts[id] = d.text
		}
	}
	if maps.Equal(texts, m.draftsOnDisk) {
		return nil
	}
	unlock, err := lockDraftsFile(m.draftsPath + ".lock")
	if err != nil {
		return err
	}
	defer unlock()

	merged := map[string]string{}
	if raw, err := os.ReadFile(m.draftsPath); err == nil {
		if err := json.Unmarshal(raw, &merged); err != nil {
			// Keep the unreadable file aside; only our own changes survive.
			bad := m.draftsPath + ".corrupt-" + strconv.FormatInt(time.Now().Unix(), 10)
			if err := os.Rename(m.draftsPath, bad); err != nil {
				return err
			}
			merged = map[string]string{}
		}
	}
	for id, text := range texts {
		if text != m.draftsOnDisk[id] {
			merged[id] = text
		}
	}
	for id := range m.draftsOnDisk {
		if _, ok := texts[id]; !ok {
			delete(merged, id)
		}
	}
	raw, err := json.Marshal(merged)
	if err != nil {
		return err
	}
	tmp := m.draftsPath + ".tmp." + strconv.Itoa(os.Getpid())
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, m.draftsPath); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	// Our view, not the merged file: the next save diffs only our changes.
	m.draftsOnDisk = texts
	return nil
}
