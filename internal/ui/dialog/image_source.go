package dialog

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	uv "github.com/charmbracelet/ultraviolet"
)

// ImageSourceID is the identifier for the image source picker dialog.
const ImageSourceID = "image_source"

const (
	imageSourceDialogMaxWidth  = 58
	imageSourceDialogMaxHeight = 10
)

// ImageSource lets the user choose where an image attachment should come from.
type ImageSource struct {
	com  *common.Common
	help help.Model
	list *list.FilterableList

	keyMap struct {
		Select,
		Next,
		Previous,
		UpDown,
		Close key.Binding
	}
}

var _ Dialog = (*ImageSource)(nil)

// NewImageSource creates a new image source picker dialog.
func NewImageSource(com *common.Common) *ImageSource {
	s := &ImageSource{com: com}

	h := help.New()
	h.Styles = com.Styles.DialogHelpStyles()
	s.help = h

	s.list = list.NewFilterableList(
		NewCommandItem(com.Styles, "clipboard_ssh", "Clipboard (SSH)", "", ActionClipboardImageSelected{}),
		NewCommandItem(com.Styles, "vps_file", "VPS image file", "", ActionOpenDialog{DialogID: FilePickerID}),
	)
	s.list.Focus()
	s.list.SetSelected(0)

	s.keyMap.Select = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "confirm"),
	)
	s.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("down", "next item"),
	)
	s.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("up", "previous item"),
	)
	s.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("up/down", "choose"),
	)
	s.keyMap.Close = CloseKey

	return s
}

// ID implements Dialog.
func (s *ImageSource) ID() string {
	return ImageSourceID
}

// HandleMsg implements Dialog.
func (s *ImageSource) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, s.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, s.keyMap.Previous):
			s.list.Focus()
			if s.list.IsSelectedFirst() {
				s.list.SelectLast()
				s.list.ScrollToBottom()
				break
			}
			s.list.SelectPrev()
			s.list.ScrollToSelected()
		case key.Matches(msg, s.keyMap.Next):
			s.list.Focus()
			if s.list.IsSelectedLast() {
				s.list.SelectFirst()
				s.list.ScrollToTop()
				break
			}
			s.list.SelectNext()
			s.list.ScrollToSelected()
		case key.Matches(msg, s.keyMap.Select):
			selectedItem := s.list.SelectedItem()
			if selectedItem == nil {
				break
			}
			item, ok := selectedItem.(*CommandItem)
			if !ok {
				break
			}
			return item.Action()
		}
	}
	return nil
}

// Draw implements Dialog.
func (s *ImageSource) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := s.com.Styles
	width := max(0, min(imageSourceDialogMaxWidth, area.Dx()))
	height := max(0, min(imageSourceDialogMaxHeight, area.Dy()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()

	s.list.SetSize(innerWidth, height-heightOffset)
	s.help.SetWidth(innerWidth)

	rc := NewRenderContext(t, width)
	rc.Title = "Add Image"
	listView := t.Dialog.List.Height(s.list.Height()).Render(s.list.Render())
	rc.AddPart(listView)
	rc.Help = s.help.View(s)

	view := rc.Render()
	DrawCenter(scr, area, view)
	return nil
}

// ShortHelp implements help.KeyMap.
func (s *ImageSource) ShortHelp() []key.Binding {
	return []key.Binding{
		s.keyMap.UpDown,
		s.keyMap.Select,
		s.keyMap.Close,
	}
}

// FullHelp implements help.KeyMap.
func (s *ImageSource) FullHelp() [][]key.Binding {
	return [][]key.Binding{s.ShortHelp()}
}
