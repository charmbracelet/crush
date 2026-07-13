package dialog

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
	fimage "github.com/charmbracelet/crush/internal/ui/image"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

const (
	SourcePreviewID          = "source_preview"
	sourcePreviewMaxWidth    = 88
	sourcePreviewMaxHeight   = 30
	sourcePreviewImageWidth  = 56
	sourcePreviewImageHeight = 14
	sourcePreviewReadLimit   = 64 * 1024
)

// SourcePreview displays a source without adding its contents to model context.
type SourcePreview struct {
	com      *common.Common
	source   session.Source
	help     help.Model
	close    key.Binding
	content  string
	image    bool
	imageEnc fimage.Encoding
	cellSize fimage.CellSize
	isTmux   bool
}

var _ Dialog = (*SourcePreview)(nil)

// NewSourcePreview creates a lazy source preview and its image transmit command.
func NewSourcePreview(com *common.Common, caps *common.Capabilities, source session.Source) (*SourcePreview, tea.Cmd) {
	p := &SourcePreview{com: com, source: source, close: CloseKey}
	p.help = help.New()
	p.help.Styles = com.Styles.DialogHelpStyles()
	if caps != nil {
		if caps.SupportsKittyGraphics() {
			p.imageEnc = fimage.EncodingKitty
		}
		width, height := caps.CellSize()
		p.cellSize = fimage.CellSize{Width: width, Height: height}
		_, p.isTmux = caps.Env.LookupEnv("TMUX")
	}

	if source.Kind == session.SourceKindFile && isPreviewImage(source.Location) {
		img, err := loadImage(source.Location)
		if err == nil {
			p.image = true
			return p, p.imageEnc.Transmit(source.Location, img, p.cellSize, sourcePreviewImageWidth, sourcePreviewImageHeight, p.isTmux)
		}
		p.content = fmt.Sprintf("Unable to preview image: %v", err)
	} else {
		p.content = sourcePreviewText(source)
	}
	return p, nil
}

func (p *SourcePreview) ID() string {
	return SourcePreviewID
}

func (p *SourcePreview) HandleMsg(msg tea.Msg) Action {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok && key.Matches(keyMsg, p.close) {
		return ActionClose{}
	}
	return nil
}

func (p *SourcePreview) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := p.com.Styles
	width := max(0, min(sourcePreviewMaxWidth, area.Dx()-2))
	height := max(0, min(sourcePreviewMaxHeight, area.Dy()-2))
	innerWidth := max(1, width-t.Dialog.View.GetHorizontalFrameSize())
	p.help.SetWidth(innerWidth)

	rc := NewRenderContext(t, width)
	rc.Gap = 1
	rc.Title = "Source Preview"
	rc.TitleInfo = t.Dialog.ListItem.InfoBlurred.Render(" " + string(p.source.Kind))
	rc.AddPart(t.Dialog.Arguments.Description.Render(p.source.Label))
	if p.source.Location != "" {
		rc.AddPart(t.Dialog.HelpView.Render(ansi.Truncate(p.source.Location, innerWidth-1, "...")))
	}
	if p.image {
		preview := p.imageEnc.Render(p.source.Location, sourcePreviewImageWidth, sourcePreviewImageHeight)
		rc.AddPart(t.Dialog.ImagePreview.Align(lipgloss.Center).Width(innerWidth).Render(preview))
	} else {
		wrapped := ansi.Wordwrap(strings.TrimSpace(p.content), innerWidth, "")
		lines := strings.Split(wrapped, "\n")
		maxLines := max(3, height-10)
		if len(lines) > maxLines {
			lines = append(lines[:maxLines], "...")
		}
		rc.AddPart(t.Dialog.HelpView.Render(strings.Join(lines, "\n")))
	}
	rc.Help = p.help.View(p)
	DrawCenter(scr, area, rc.Render())
	return nil
}

func (p *SourcePreview) ShortHelp() []key.Binding {
	return []key.Binding{p.close}
}

func (p *SourcePreview) FullHelp() [][]key.Binding {
	return [][]key.Binding{p.ShortHelp()}
}

func isPreviewImage(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg", ".png":
		return true
	default:
		return false
	}
}

func sourcePreviewText(source session.Source) string {
	switch source.Kind {
	case session.SourceKindText:
		return source.Content
	case session.SourceKindURL:
		return "URL sources remain lazy. Ask the agent to read this source when its page contents are needed."
	case session.SourceKindFile:
		file, err := os.Open(source.Location)
		if err != nil {
			return fmt.Sprintf("Unable to open source: %v", err)
		}
		defer file.Close()
		content, err := io.ReadAll(io.LimitReader(file, sourcePreviewReadLimit+1))
		if err != nil {
			return fmt.Sprintf("Unable to read source: %v", err)
		}
		if !utf8.Valid(content) {
			return "Binary file preview is unavailable. The source remains attached and can be resolved by a compatible tool."
		}
		truncated := len(content) > sourcePreviewReadLimit
		if truncated {
			content = content[:sourcePreviewReadLimit]
		}
		text := string(content)
		if truncated {
			text += "\n\n[Preview truncated]"
		}
		return text
	default:
		return "Source preview is unavailable."
	}
}
