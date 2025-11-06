// Based on the implementation by @trashhalo at:
// https://github.com/trashhalo/imgcat
package image

import (
	"fmt"
	_ "image/jpeg"
	_ "image/png"

	tea "charm.land/bubbletea/v2"
)

type Model struct {
	url    string
	image  string
	width  uint
	height uint
	err    error
}

func New(width, height uint, url string) Model {
	return Model{
		width:  width,
		height: height,
		url:    url,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case errMsg:
		m.err = msg
		return m, nil
	case redrawMsg:
		m.width = msg.width
		m.height = msg.height
		m.url = msg.url
		return m, loadURL(m.url)
	case loadMsg:
		return handleLoadMsg(m, msg)
	}
	return m, nil
}

func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("couldn't load image(s): %v", m.err)
	}
	return m.image
}

type errMsg struct{ error }

func (m Model) Redraw(width uint, height uint, url string) tea.Cmd {
	return func() tea.Msg {
		return redrawMsg{
			width:  width,
			height: height,
			url:    url,
		}
	}
}

func (m Model) UpdateURL(url string) tea.Cmd {
	return func() tea.Msg {
		return redrawMsg{
			width:  m.width,
			height: m.height,
			url:    url,
		}
	}
}

type redrawMsg struct {
	width  uint
	height uint
	url    string
}

func (m Model) IsLoading() bool {
	return m.image == ""
}
