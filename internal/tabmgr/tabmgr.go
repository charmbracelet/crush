package tabmgr

import (
	"fmt"
	"sync"
)

// TabManager manages a list of tabs with an active tab selection.
type TabManager struct {
	mu        sync.RWMutex
	tabs      []*Tab
	activeIdx int
}

// New creates a new TabManager with no tabs.
func New() *TabManager {
	return &TabManager{}
}

// AddTab appends a new tab and returns it. The new tab becomes active.
func (m *TabManager) AddTab(name, cwd string, paneType PaneType) *Tab {
	m.mu.Lock()
	defer m.mu.Unlock()

	tab := NewTab(name, cwd, paneType)
	m.tabs = append(m.tabs, tab)
	m.activeIdx = len(m.tabs) - 1
	return tab
}

// CloseTab removes the tab at the given index. Returns an error if the index
// is out of range. If the active tab is closed, the previous tab becomes active
// (or the next one if closing the first tab).
func (m *TabManager) CloseTab(idx int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if idx < 0 || idx >= len(m.tabs) {
		return fmt.Errorf("tab index %d out of range [0, %d)", idx, len(m.tabs))
	}

	m.tabs = append(m.tabs[:idx], m.tabs[idx+1:]...)

	if len(m.tabs) == 0 {
		m.activeIdx = 0
		return nil
	}

	if m.activeIdx >= len(m.tabs) {
		m.activeIdx = len(m.tabs) - 1
	} else if m.activeIdx > idx {
		m.activeIdx--
	}

	return nil
}

// CloseActiveTab closes the currently active tab. The operation is atomic —
// no TOCTOU race between reading the active index and closing.
func (m *TabManager) CloseActiveTab() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	idx := m.activeIdx
	if idx < 0 || idx >= len(m.tabs) {
		return fmt.Errorf("no active tab")
	}

	m.tabs = append(m.tabs[:idx], m.tabs[idx+1:]...)

	if len(m.tabs) == 0 {
		m.activeIdx = 0
		return nil
	}
	if m.activeIdx >= len(m.tabs) {
		m.activeIdx = len(m.tabs) - 1
	} else if m.activeIdx > idx {
		m.activeIdx--
	}
	return nil
}

// SelectTab sets the active tab by index.
func (m *TabManager) SelectTab(idx int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if idx < 0 || idx >= len(m.tabs) {
		return fmt.Errorf("tab index %d out of range [0, %d)", idx, len(m.tabs))
	}
	m.activeIdx = idx
	return nil
}

// SelectTabByID sets the active tab by tab ID.
func (m *TabManager) SelectTabByID(tabID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, t := range m.tabs {
		if t.ID == tabID {
			m.activeIdx = i
			return nil
		}
	}
	return fmt.Errorf("tab %q not found", tabID)
}

// NextTab cycles to the next tab (wraps around).
func (m *TabManager) NextTab() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.tabs) == 0 {
		return
	}
	m.activeIdx = (m.activeIdx + 1) % len(m.tabs)
}

// PrevTab cycles to the previous tab (wraps around).
func (m *TabManager) PrevTab() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.tabs) == 0 {
		return
	}
	m.activeIdx = (m.activeIdx - 1 + len(m.tabs)) % len(m.tabs)
}

// MoveTabUp moves the active tab one position up in the list.
func (m *TabManager) MoveTabUp() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.activeIdx <= 0 || len(m.tabs) < 2 {
		return
	}
	m.tabs[m.activeIdx], m.tabs[m.activeIdx-1] = m.tabs[m.activeIdx-1], m.tabs[m.activeIdx]
	m.activeIdx--
}

// MoveTabDown moves the active tab one position down in the list.
func (m *TabManager) MoveTabDown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.activeIdx >= len(m.tabs)-1 || len(m.tabs) < 2 {
		return
	}
	m.tabs[m.activeIdx], m.tabs[m.activeIdx+1] = m.tabs[m.activeIdx+1], m.tabs[m.activeIdx]
	m.activeIdx++
}

// ActiveTab returns the currently active tab, or nil if no tabs exist.
func (m *TabManager) ActiveTab() *Tab {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.tabs) == 0 {
		return nil
	}
	return m.tabs[m.activeIdx]
}

// ActiveIndex returns the index of the active tab.
func (m *TabManager) ActiveIndex() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeIdx
}

// Tabs returns a copy of the tab list (safe for iteration).
func (m *TabManager) Tabs() []*Tab {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Tab, len(m.tabs))
	copy(result, m.tabs)
	return result
}

// Len returns the number of tabs.
func (m *TabManager) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.tabs)
}

// GetTab returns the tab at the given index.
func (m *TabManager) GetTab(idx int) *Tab {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if idx < 0 || idx >= len(m.tabs) {
		return nil
	}
	return m.tabs[idx]
}

// FindTab returns the tab with the given ID, or nil.
func (m *TabManager) FindTab(tabID string) *Tab {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, t := range m.tabs {
		if t.ID == tabID {
			return t
		}
	}
	return nil
}

// RenameTab renames the tab at the given index.
func (m *TabManager) RenameTab(idx int, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if idx < 0 || idx >= len(m.tabs) {
		return fmt.Errorf("tab index %d out of range", idx)
	}
	m.tabs[idx].Name = name
	return nil
}
