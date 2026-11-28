package dialog

import (
	"image"
	_ "image/jpeg" // register JPEG format
	_ "image/png"  // register PNG format
	"os"
	"strings"
	"sync/atomic"

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

	// previewKey is the preview last requested; a load is scheduled only
	// when it changes. previewGen is bumped with it so loads for previews
	// the user has already moved past skip decoding.
	previewKey filePickerPreviewKey
	previewGen atomic.Uint64

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
		key.WithHelp("↓/j", "move down"),
	)
	f.km.Up = key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "move up"),
	)
	f.km.Forward = key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("→/l", "move forward"),
	)
	f.km.Backward = key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/h", "move backward"),
	)
	f.km.Navigate = key.NewBinding(
		key.WithKeys("right", "l", "left", "h", "up", "k", "down", "j"),
		key.WithHelp("↑/↓/←/→", "navigate"),
	)
	f.km.Close = CloseKey

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
	return dialogHelp(f.km.Select, f.km.Close, f.km.Navigate)
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
	// The UI drops the actions returned for mouse messages, so a load
	// scheduled there would never run. The file list ignores the mouse, so
	// nothing about the preview changes on those anyway.
	if _, isMouse := msg.(tea.MouseMsg); !isMouse {
		if loadCmd := f.updatePreview(); loadCmd != nil {
			cmds = append(cmds, loadCmd)
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

	const gap = 1
	rc := NewRenderContext(t, width)
	rc.Gap = gap
	rc.Title = "Add Image"
	rc.Help = renderDialogHelp(t, &f.help, f, innerWidth)
	// Give the file list its rows first so it is never pushed off-screen.
	avail := max(0, area.Dy()-rc.ChromeHeight())
	height := min(filePickerMinHeight, avail)

	// Preview in the height left, hidden when there is nothing or too little room.
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

// filePickerPreviewKey identifies an image preview at a given size.
type filePickerPreviewKey struct {
	path          string
	width, height int
}

// filePickerPreviewLoadedMsg is sent once a preview load has finished, so
// the picker re-checks whether its preview is ready.
type filePickerPreviewLoadedMsg struct{}

// updatePreview refreshes the preview state for the highlighted file and
// returns a command loading its image when it is not cached yet. Decoding
// happens in the command, never on the UI goroutine.
func (f *FilePicker) updatePreview() tea.Cmd {
	selFile := f.fp.HighlightedPath()
	allowed := selFile != "" && f.isAllowedImage(selFile)

	var key filePickerPreviewKey
	if allowed && f.imgPrevWidth > 0 && f.imgPrevHeight > 0 {
		key = filePickerPreviewKey{path: selFile, width: f.imgPrevWidth, height: f.imgPrevHeight}
	}

	f.previewingImage = key != (filePickerPreviewKey{}) &&
		fimage.HasTransmitted(key.path, key.width, key.height)

	if key == f.previewKey {
		return nil
	}
	f.previewKey = key
	gen := f.previewGen.Add(1)
	if key == (filePickerPreviewKey{}) || f.previewingImage {
		return nil
	}
	return f.loadPreview(key, gen)
}

// isAllowedImage reports whether the path has an image extension the picker
// can preview.
func (f *FilePicker) isAllowedImage(path string) bool {
	lower := strings.ToLower(path)
	for _, allowedExt := range f.fp.AllowedTypes {
		if strings.HasSuffix(lower, allowedExt) {
			return true
		}
	}
	return false
}

// loadPreview decodes and transmits the image for key, then reports back
// with [filePickerPreviewLoadedMsg]. A load the user has already moved past
// is dropped before it decodes anything.
func (f *FilePicker) loadPreview(key filePickerPreviewKey, gen uint64) tea.Cmd {
	return tea.Sequence(
		f.previewLoader(key, gen),
		func() tea.Msg { return filePickerPreviewLoadedMsg{} },
	)
}

// previewLoader returns the command half of [FilePicker.loadPreview] that
// decodes and transmits the image. It only reads state captured up front
// and the atomic generation, so it is safe to run off the UI goroutine.
func (f *FilePicker) previewLoader(key filePickerPreviewKey, gen uint64) tea.Cmd {
	enc, cs, tmux := f.imgEnc, f.CellSize(), f.isTmux
	return func() tea.Msg {
		if f.previewGen.Load() != gen {
			return nil
		}
		img, err := loadImage(key.path)
		if err != nil {
			return nil
		}
		if cmd := enc.Transmit(key.path, img, cs, key.width, key.height, tmux); cmd != nil {
			return cmd()
		}
		return nil
	}
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
