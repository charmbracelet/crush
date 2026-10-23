package dialog

// StyleRefresher is implemented by dialogs, inline editors and list items
// that copy styles or cache renders when they are built. RefreshStyles
// makes them pick up the current styles after the theme was swapped in
// place (a theme switch, a preview, or a host application's theme).
type StyleRefresher interface {
	RefreshStyles()
}

// RefreshStyles has every open dialog pick up the current styles.
func (d *Overlay) RefreshStyles() {
	for _, dialog := range d.dialogs {
		if r, ok := dialog.(StyleRefresher); ok {
			r.RefreshStyles()
		}
	}
}

// refreshItems has every item that implements [StyleRefresher] pick up
// the current styles, including items a filter hides.
func refreshItems[T any](items []T) {
	for _, item := range items {
		if r, ok := any(item).(StyleRefresher); ok {
			r.RefreshStyles()
		}
	}
}

// RefreshStyles implements [StyleRefresher].
func (p *Permissions) RefreshStyles() {
	p.help.Styles = p.com.Styles.DialogHelpStyles()
	// The viewport keeps the rendered content, diffs included, until it
	// is marked dirty.
	p.viewportDirty = true
}

// RefreshStyles implements [StyleRefresher].
func (c *Commands) RefreshStyles() {
	c.help.Styles = c.com.Styles.DialogHelpStyles()
	c.input.SetStyles(c.com.Styles.TextInput)
	c.spinner.Style = c.com.Styles.Dialog.Spinner
	refreshItems(c.list.Items())
}

// RefreshStyles implements [StyleRefresher].
func (m *Models) RefreshStyles() {
	m.help.Styles = m.com.Styles.DialogHelpStyles()
	m.input.SetStyles(m.com.Styles.TextInput)
	for i := range m.list.groups {
		g := &m.list.groups[i]
		if g.Versioned != nil {
			g.Bump()
		}
		refreshItems(g.Items)
	}
}

// RefreshStyles implements [StyleRefresher].
func (s *Session) RefreshStyles() {
	s.help.Styles = s.com.Styles.DialogHelpStyles()
	s.input.SetStyles(s.com.Styles.TextInput)
	refreshItems(s.list.Items())
}

// RefreshStyles implements [StyleRefresher].
func (r *Reasoning) RefreshStyles() {
	r.help.Styles = r.com.Styles.DialogHelpStyles()
	r.input.SetStyles(r.com.Styles.TextInput)
	refreshItems(r.list.Items())
}

// RefreshStyles implements [StyleRefresher].
func (n *Notifications) RefreshStyles() {
	n.help.Styles = n.com.Styles.DialogHelpStyles()
	n.input.SetStyles(n.com.Styles.TextInput)
	refreshItems(n.list.Items())
}

// RefreshStyles implements [StyleRefresher].
func (a *Arguments) RefreshStyles() {
	a.help.Styles = a.com.Styles.DialogHelpStyles()
	for i := range a.inputs {
		a.inputs[i].SetStyles(a.com.Styles.TextInput)
	}
	a.spinner.Style = a.com.Styles.Dialog.Spinner
}

// RefreshStyles implements [StyleRefresher].
func (ed *ThemeEditor) RefreshStyles() {
	ed.help.Styles = ed.com.Styles.DialogHelpStyles()
	ed.input.SetStyles(ed.com.Styles.TextInput)
}

// RefreshStyles implements [StyleRefresher].
func (d *ThemeNew) RefreshStyles() {
	d.help.Styles = d.com.Styles.DialogHelpStyles()
	d.input.SetStyles(d.com.Styles.TextInput)
}

// RefreshStyles implements [StyleRefresher]. The input is restyled on
// every draw, for the state it is in.
func (m *APIKeyInput) RefreshStyles() {
	m.help.Styles = m.com.Styles.DialogHelpStyles()
	m.spinner.Style = m.com.Styles.Dialog.APIKey.Spinner
}

// RefreshStyles implements [StyleRefresher].
func (m *AuthMethod) RefreshStyles() {
	m.help.Styles = m.com.Styles.DialogHelpStyles()
}

// RefreshStyles implements [StyleRefresher].
func (m *AWSSSO) RefreshStyles() {
	m.help.Styles = m.com.Styles.DialogHelpStyles()
	m.spinner.Style = m.com.Styles.Dialog.OAuth.Spinner
}

// RefreshStyles implements [StyleRefresher].
func (m *MCPAuth) RefreshStyles() {
	m.help.Styles = m.com.Styles.DialogHelpStyles()
	m.spinner.Style = m.com.Styles.Dialog.OAuth.Spinner
}

// RefreshStyles implements [StyleRefresher].
func (m *MCPToggles) RefreshStyles() {
	m.help.Styles = m.com.Styles.DialogHelpStyles()
}

// RefreshStyles implements [StyleRefresher].
func (m *OAuth) RefreshStyles() {
	m.help.Styles = m.com.Styles.DialogHelpStyles()
	m.spinner.Style = m.com.Styles.Dialog.OAuth.Spinner
	if m.oAuthProvider.supportsCodeEntry() {
		m.codeInput.SetStyles(m.com.Styles.TextInput)
	}
}

// RefreshStyles implements [StyleRefresher]. The file list is restyled on
// every draw.
func (f *FilePicker) RefreshStyles() {
	f.help.Styles = f.com.Styles.DialogHelpStyles()
}

// RefreshStyles implements [StyleRefresher].
func (f *QuestionForm) RefreshStyles() {
	for _, q := range f.questions {
		if r, ok := q.(StyleRefresher); ok {
			r.RefreshStyles()
		}
	}
}

// RefreshStyles implements [StyleRefresher].
func (e *questionEditor) RefreshStyles() {
	e.fillIn.SetStyles(questionTextareaStyles(e.Styles))
	e.noteEditor.SetStyles(questionTextareaStyles(e.Styles))
}

// RefreshStyles implements [StyleRefresher].
func (d *FreeText) RefreshStyles() {
	d.editor.SetStyles(questionTextareaStyles(d.Styles))
}

// RefreshStyles implements [StyleRefresher].
func (p *PlanHandoffInline) RefreshStyles() {
	p.editor.SetStyles(questionTextareaStyles(p.com.Styles))
}

// RefreshStyles implements [StyleRefresher].
func (c *CommandItem) RefreshStyles() {
	c.cache = nil
	if c.Versioned != nil {
		c.Bump()
	}
}

// RefreshStyles implements [StyleRefresher].
func (m *ModelItem) RefreshStyles() {
	m.cache = nil
	if m.Versioned != nil {
		m.Bump()
	}
}

// RefreshStyles implements [StyleRefresher].
func (s *SessionItem) RefreshStyles() {
	s.cache = nil
	if s.sessionsMode == sessionsModeUpdating {
		s.updateTitleInput.SetStyles(sessionRenameInputStyles(s.t))
	}
	if s.Versioned != nil {
		s.Bump()
	}
}

// RefreshStyles implements [StyleRefresher].
func (r *ReasoningItem) RefreshStyles() {
	r.cache = nil
	if r.Versioned != nil {
		r.Bump()
	}
}

// RefreshStyles implements [StyleRefresher].
func (n *NotificationItem) RefreshStyles() {
	n.cache = nil
	if n.Versioned != nil {
		n.Bump()
	}
}
