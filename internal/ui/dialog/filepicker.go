package dialog

import (
	"image"
	_ "image/jpeg" // register JPEG format
	_ "image/png"  // register PNG format
	"os"
	"strings"

	"charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/home"
	"github.com/charmbracelet/crush/internal/ui/common"
	fimage "github.com/charmbracelet/crush/internal/ui/image"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// FilePickerID is the identifier for the FilePicker dialog.
const FilePickerID = "filepicker"

// FilePicker is a dialog that allows users to select files or directories.
type FilePicker struct {
	com *common.Common

	imgEnc                      fimage.Encoding
	imgPrevWidth, imgPrevHeight int
	cellSizeW, cellSizeH        int

	fp              filepicker.Model
	help            help.Model
	previewingImage bool // indicates if an image is being previewed
	isTmux          bool

	km struct {
		Select,
		Down,
		Up,
		Forward,
		Backward,
		Navigate,
		Close key.Binding
	}
}

// CellSize returns the cell size used for image rendering.
func (f *FilePicker) CellSize() fimage.CellSize {
	return fimage.CellSize{
		Width:  f.cellSizeW,
		Height: f.cellSizeH,
	}
}

var _ Dialog = (*FilePicker)(nil)

// NewFilePicker creates a new [FilePicker] dialog.
func NewFilePicker(com *common.Common) (*FilePicker, tea.Cmd) {
	f := new(FilePicker)
	f.com = com

	help := help.New()
	help.Styles = com.Styles.DialogHelpStyles()

	f.help = help

	f.km.Select = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "accept"),
	)
	f.km.Down = key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("down/j", "move down"),
	)
	f.km.Up = key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("up/k", "move up"),
	)
	f.km.Forward = key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("right/l", "move forward"),
	)
	f.km.Backward = key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("left/h", "move backward"),
	)
	f.km.Navigate = key.NewBinding(
		key.WithKeys("right", "l", "left", "h", "up", "k", "down", "j"),
		key.WithHelp("↑↓←→", "navigate"),
	)
	f.km.Close = key.NewBinding(
		key.WithKeys("esc", "alt+esc"),
		key.WithHelp("esc", "close/exit"),
	)

	fp := filepicker.New()
	fp.AllowedTypes = common.AllowedImageTypes
	fp.ShowPermissions = false
	fp.ShowSize = false
	fp.AutoHeight = false
	fp.Styles = com.Styles.FilePicker
	fp.Cursor = ""
	fp.CurrentDirectory = f.WorkingDir()

	f.fp = fp

	return f, f.fp.Init()
}

// SetImageCapabilities sets the image capabilities for the [FilePicker].
func (f *FilePicker) SetImageCapabilities(caps *common.Capabilities) {
	if caps != nil {
		if caps.SupportsKittyGraphics() {
			f.imgEnc = fimage.EncodingKitty
		}
		f.cellSizeW, f.cellSizeH = caps.CellSize()
		_, f.isTmux = caps.Env.LookupEnv("TMUX")
	}
}

// WorkingDir returns the current working directory of the [FilePicker].
func (f *FilePicker) WorkingDir() string {
	wd := f.com.Workspace.WorkingDir()
	if len(wd) > 0 {
		return wd
	}

	cwd, err := os.Getwd()
	if err != nil {
		return home.Dir()
	}

	return cwd
}

// ShortHelp returns the short help key bindings for the [FilePicker] dialog.
func (f *FilePicker) ShortHelp() []key.Binding {
	return []key.Binding{
		f.km.Navigate,
		f.km.Select,
		f.km.Close,
	}
}

// FullHelp returns the full help key bindings for the [FilePicker] dialog.
func (f *FilePicker) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{
			f.km.Select,
			f.km.Down,
			f.km.Up,
			f.km.Forward,
		},
		{
			f.km.Backward,
			f.km.Close,
		},
	}
}

// ID returns the identifier of the [FilePicker] dialog.
func (f *FilePicker) ID() string {
	return FilePickerID
}

// HandleMsg updates the [FilePicker] dialog based on the given message.
func (f *FilePicker) HandleMsg(msg tea.Msg) Action {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, f.km.Close):
			return ActionClose{}
		}
	}

	var cmd tea.Cmd
	f.fp, cmd = f.fp.Update(msg)
	if selFile := f.fp.HighlightedPath(); selFile != "" {
		var allowed bool
		for _, allowedExt := range f.fp.AllowedTypes {
			if strings.HasSuffix(strings.ToLower(selFile), allowedExt) {
				allowed = true
				break
			}
		}

		f.previewingImage = allowed
		if allowed && !fimage.HasTransmitted(selFile, f.imgPrevWidth, f.imgPrevHeight) {
			f.previewingImage = false
			img, err := loadImage(selFile)
			if err == nil {
				cmds = append(cmds, tea.Sequence(
					f.imgEnc.Transmit(selFile, img, f.CellSize(), f.imgPrevWidth, f.imgPrevHeight, f.isTmux),
					func() tea.Msg {
						f.previewingImage = true
						return nil
					},
				))
			}
		}
	}
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	if didSelect, path := f.fp.DidSelectFile(msg); didSelect {
		return ActionFilePickerSelected{Path: path}
	}

	return ActionCmd{tea.Batch(cmds...)}
}

const (
	filePickerMinWidth  = 70
	filePickerMinHeight = 10
	// filePickerMinPreviewHeight is the shortest image preview worth
	// showing; below it the preview is hidden to leave room for files.
	filePickerMinPreviewHeight = 5
)

// Draw renders the [FilePicker] dialog as a string.
func (f *FilePicker) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := f.com.Styles
	width := max(0, min(filePickerMinWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()

	// The dialog is the title, a gap, an optional image preview and gap,
	// the file list, a gap, and the help line, inside the border. Give
	// the file list its rows first so it is never pushed off-screen.
	const gap = 1
	const helpContentHeight = 1
	avail := max(0, area.Dy()-t.Dialog.View.GetVerticalFrameSize()-
		t.Dialog.Title.GetVerticalFrameSize()-titleContentHeight-gap-
		gap-t.Dialog.HelpView.GetVerticalFrameSize()-helpContentHeight)
	height := min(filePickerMinHeight, avail)

	// Show the image preview in whatever height is left, capped to its
	// normal size. Hide it entirely when there is nothing to preview or
	// the dialog is too short for a useful preview.
	imgPrevHeight := 0
	if f.fp.HighlightedPath() != "" {
		imgPrevHeight = min(
			filePickerMinHeight*2-t.Dialog.ImagePreview.GetVerticalFrameSize(),
			avail-height-gap-t.Dialog.ImagePreview.GetVerticalFrameSize(),
		)
		if imgPrevHeight < filePickerMinPreviewHeight {
			imgPrevHeight = 0
		}
	}
	imgPrevWidth := innerWidth - t.Dialog.ImagePreview.GetHorizontalFrameSize()
	f.imgPrevWidth = imgPrevWidth
	f.imgPrevHeight = imgPrevHeight
	f.fp.SetHeight(height)

	styles := t.FilePicker
	styles.File = styles.File.Width(innerWidth)
	styles.Directory = styles.Directory.Width(innerWidth)
	styles.Selected = styles.Selected.PaddingLeft(1).Width(innerWidth)
	styles.DisabledSelected = styles.DisabledSelected.PaddingLeft(1).Width(innerWidth)
	f.fp.Styles = styles

	rc := NewRenderContext(t, width)
	rc.Gap = gap
	rc.Title = "Add Image"
	rc.Help = renderDialogHelp(t, &f.help, f, innerWidth)

	if imgPrevHeight > 0 {
		imgPreview := t.Dialog.ImagePreview.Align(lipgloss.Center).Width(innerWidth).Render(f.imagePreview(imgPrevWidth, imgPrevHeight))
		rc.AddPart(imgPreview)
	}

	// Trim only trailing blank rows: leading spaces are the list's own
	// left padding (e.g. before "Empty directory").
	files := strings.TrimRight(f.fp.View(), " \n")
	rc.AddPart(files)

	view := rc.Render()

	DrawCenter(scr, area, view)
	return nil
}

// imagePreview returns the image preview section of the [FilePicker] dialog.
func (f *FilePicker) imagePreview(imgPrevWidth, imgPrevHeight int) string {
	if !f.previewingImage {
		// Until an image is ready, show a quiet hint centered in the
		// preview area instead of a solid block.
		hint := f.com.Styles.Dialog.ImagePreview.UnsetPadding().Render(
			ansi.Truncate("No preview", imgPrevWidth, "…"))
		return lipgloss.Place(imgPrevWidth, imgPrevHeight, lipgloss.Center, lipgloss.Center, hint)
	}

	if id := f.fp.HighlightedPath(); id != "" {
		r := f.imgEnc.Render(id, imgPrevWidth, imgPrevHeight)
		return r
	}

	return ""
}

func loadImage(path string) (img image.Image, err error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err = image.Decode(file)
	if err != nil {
		return nil, err
	}

	return img, nil
}
