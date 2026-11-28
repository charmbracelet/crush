package image

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	"io"
	"log/slog"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/util"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/ansi/kitty"
	"github.com/disintegration/imaging"
	paintbrush "github.com/jordanella/go-ansi-paintbrush"
)

// TransmittedMsg is a message indicating that an image has been transmitted to
// the terminal.
type TransmittedMsg struct {
	ID string
}

// Encoding represents the encoding format of the image.
type Encoding byte

// Image encodings.
const (
	EncodingBlocks Encoding = iota
	EncodingKitty
)

type imageKey struct {
	id   string
	cols int
	rows int
}

// Hash returns a hash value for the image key.
// This uses FNV-32a for simplicity and speed.
func (k imageKey) Hash() uint32 {
	h := fnv.New32a()
	_, _ = io.WriteString(h, k.ID())
	return h.Sum32()
}

// ID returns a unique string representation of the image key.
func (k imageKey) ID() string {
	return fmt.Sprintf("%s-%dx%d", k.id, k.cols, k.rows)
}

// CellSize represents the size of a single terminal cell in pixels.
type CellSize struct {
	Width, Height int
}

// maxCachedImages bounds how many previews are kept at once. Only one
// preview is visible at a time, so a handful covers moving back and forth.
const maxCachedImages = 16

// cachedImage records an image that has been prepared for rendering. Decoded
// pixels are not kept: blocks keep only their painted string, and kitty
// images live on the terminal side.
type cachedImage struct {
	blocks string
}

var (
	cachedImages = map[imageKey]cachedImage{}
	cachedOrder  []imageKey
	cachedMutex  sync.RWMutex
)

// ResetCache clears the image cache, freeing all cached renderings.
func ResetCache() {
	cachedMutex.Lock()
	clear(cachedImages)
	cachedOrder = nil
	cachedMutex.Unlock()
}

// storeCached records an image in the cache, evicting the oldest entries
// beyond [maxCachedImages].
func storeCached(key imageKey, entry cachedImage) {
	cachedMutex.Lock()
	defer cachedMutex.Unlock()
	if _, ok := cachedImages[key]; !ok {
		cachedOrder = append(cachedOrder, key)
	}
	cachedImages[key] = entry
	for len(cachedOrder) > maxCachedImages {
		delete(cachedImages, cachedOrder[0])
		cachedOrder[0] = imageKey{}
		cachedOrder = cachedOrder[1:]
	}
}

// fitImage resizes the image to fit within the specified dimensions in
// terminal cells, maintaining the aspect ratio.
func fitImage(img image.Image, cs CellSize, cols, rows int) image.Image {
	if img == nil || cs.Width == 0 || cs.Height == 0 {
		return img
	}
	return imaging.Fit(img, cols*cs.Width, rows*cs.Height, imaging.Lanczos)
}

// HasTransmitted checks if the image with the given ID has already been
// transmitted to the terminal.
func HasTransmitted(id string, cols, rows int) bool {
	key := imageKey{id: id, cols: cols, rows: rows}

	cachedMutex.RLock()
	_, ok := cachedImages[key]
	cachedMutex.RUnlock()
	return ok
}

// Transmit transmits the image data to the terminal if needed. This is used to
// cache the image on the terminal for later rendering. For blocks, the image
// is painted once here, off the UI goroutine, so [Encoding.Render] only
// returns the cached result.
func (e Encoding) Transmit(id string, img image.Image, cs CellSize, cols, rows int, tmux bool) tea.Cmd {
	if img == nil {
		return nil
	}

	key := imageKey{id: id, cols: cols, rows: rows}
	if HasTransmitted(id, cols, rows) {
		return nil
	}

	cmd := func() tea.Msg {
		if e != EncodingKitty {
			storeCached(key, cachedImage{blocks: paintBlocksFunc(img, cols, rows)})
			return TransmittedMsg{ID: key.ID()}
		}

		var buf bytes.Buffer
		img := fitImage(img, cs, cols, rows)
		bounds := img.Bounds()
		imgWidth := bounds.Dx()
		imgHeight := bounds.Dy()
		imgID := int(key.Hash())
		if err := kitty.EncodeGraphics(&buf, img, &kitty.Options{
			ID:               imgID,
			Action:           kitty.TransmitAndPut,
			Transmission:     kitty.Direct,
			Format:           kitty.RGBA,
			ImageWidth:       imgWidth,
			ImageHeight:      imgHeight,
			Columns:          cols,
			Rows:             rows,
			VirtualPlacement: true,
			Quiet:            1,
			Chunk:            true,
			ChunkFormatter: func(chunk string) string {
				if tmux {
					return ansi.TmuxPassthrough(chunk)
				}
				return chunk
			},
		}); err != nil {
			slog.Error("Failed to encode image for kitty graphics", "err", err)
			return util.InfoMsg{
				Type: util.InfoTypeError,
				Msg:  "failed to encode image",
			}
		}

		storeCached(key, cachedImage{})
		return tea.RawMsg{Msg: buf.String()}
	}

	return cmd
}

// paintBlocksFunc paints images for the blocks encoding. Tests replace it to
// count paints without running the rasterizer.
var paintBlocksFunc = paintBlocks

// paintBlocks renders the image as Unicode block characters filling the
// given number of cells.
func paintBlocks(img image.Image, cols, rows int) string {
	canvas := paintbrush.New()
	canvas.SetImage(img)
	canvas.SetWidth(cols)
	canvas.SetHeight(rows)
	canvas.Weights = map[rune]float64{
		'': .95,
		'': .95,
		'▁': .9,
		'▂': .9,
		'▃': .9,
		'▄': .9,
		'▅': .9,
		'▆': .85,
		'█': .85,
		'▊': .95,
		'▋': .95,
		'▌': .95,
		'▍': .95,
		'▎': .95,
		'▏': .95,
		'●': .95,
		'◀': .95,
		'▲': .95,
		'▶': .95,
		'▼': .9,
		'○': .8,
		'◉': .95,
		'◧': .9,
		'◨': .9,
		'◩': .9,
		'◪': .9,
	}
	canvas.Paint()
	return strings.TrimSpace(canvas.GetResult())
}

// Render renders the given image within the specified dimensions using the
// specified encoding.
func (e Encoding) Render(id string, cols, rows int) string {
	key := imageKey{id: id, cols: cols, rows: rows}
	cachedMutex.RLock()
	cached, ok := cachedImages[key]
	cachedMutex.RUnlock()
	if !ok {
		return ""
	}

	switch e {
	case EncodingBlocks:
		return cached.blocks
	case EncodingKitty:
		// Build Kitty graphics unicode place holders
		var fg color.Color
		var extra int
		var r, g, b int
		hashedID := key.Hash()
		id := int(hashedID)
		extra, r, g, b = id>>24&0xff, id>>16&0xff, id>>8&0xff, id&0xff

		if id <= 255 {
			fg = ansi.IndexedColor(b)
		} else {
			fg = color.RGBA{
				R: uint8(r), //nolint:gosec
				G: uint8(g), //nolint:gosec
				B: uint8(b), //nolint:gosec
				A: 0xff,
			}
		}

		fgStyle := ansi.NewStyle().ForegroundColor(fg).String()

		var buf bytes.Buffer
		for y := range rows {
			// As an optimization, we only write the fg color sequence id, and
			// column-row data once on the first cell. The terminal will handle
			// the rest.
			buf.WriteString(fgStyle)
			buf.WriteRune(kitty.Placeholder)
			buf.WriteRune(kitty.Diacritic(y))
			buf.WriteRune(kitty.Diacritic(0))
			if extra > 0 {
				buf.WriteRune(kitty.Diacritic(extra))
			}
			for x := 1; x < cols; x++ {
				buf.WriteString(fgStyle)
				buf.WriteRune(kitty.Placeholder)
			}
			if y < rows-1 {
				buf.WriteByte('\n')
			}
		}

		return buf.String()

	default:
		return ""
	}
}
