// Based on the implementation by @trashhalo at:
// https://github.com/trashhalo/imgcat
package image

import (
	"image"
	"image/png"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/disintegration/imageorient"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/muesli/termenv"
	"github.com/nfnt/resize"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

type loadMsg struct {
	io.ReadCloser
}

func loadUrl(url string) tea.Cmd {
	var r io.ReadCloser
	var err error

	if strings.HasPrefix(url, "http") {
		var resp *http.Response
		resp, err = http.Get(url)
		r = resp.Body
	} else {
		r, err = os.Open(url)
	}

	if err != nil {
		return func() tea.Msg {
			return errMsg{err}
		}
	}

	return load(r)
}

func load(r io.ReadCloser) tea.Cmd {
	return func() tea.Msg {
		return loadMsg{r}
	}
}

func handleLoadMsg(m Model, msg loadMsg) (Model, tea.Cmd) {
	if m.cancelAnimation != nil {
		m.cancelAnimation()
	}

	// blank out image so it says "loading..."
	m.image = ""

	return handleLoadMsgStatic(m, msg)
}

func handleLoadMsgStatic(m Model, msg loadMsg) (Model, tea.Cmd) {
	defer msg.Close()

	img, err := readerToimage(m.width, m.height, m.url, msg)
	if err != nil {
		return m, func() tea.Msg { return errMsg{err} }
	}
	m.image = img
	return m, nil
}

func imageToString(width, height uint, url string, img image.Image) (string, error) {
	img = resize.Thumbnail(width, height*2-4, img, resize.Lanczos3)
	b := img.Bounds()
	w := b.Max.X
	h := b.Max.Y
	p := termenv.ColorProfile()
	str := strings.Builder{}
	for y := 0; y < h; y += 2 {
		for x := w; x < int(width); x = x + 2 {
			str.WriteString(" ")
		}
		for x := 0; x < w; x++ {
			c1, _ := colorful.MakeColor(img.At(x, y))
			color1 := p.Color(c1.Hex())
			c2, _ := colorful.MakeColor(img.At(x, y+1))
			color2 := p.Color(c2.Hex())
			str.WriteString(termenv.String("▀").
				Foreground(color1).
				Background(color2).
				String())
		}
		str.WriteString("\n")
	}
	return str.String(), nil
}

func readerToimage(width uint, height uint, url string, r io.Reader) (string, error) {
	if strings.HasSuffix(strings.ToLower(url), ".svg") {
		return svgToimage(width, height, url, r)
	}

	img, _, err := imageorient.Decode(r)
	if err != nil {
		return "", err
	}

	return imageToString(width, height, url, img)
}

func svgToimage(width uint, height uint, url string, r io.Reader) (string, error) {
	// Original author: https://stackoverflow.com/users/10826783/usual-human
	// https://stackoverflow.com/questions/42993407/how-to-create-and-export-svg-to-png-jpeg-in-golang
	// Adapted to use size from SVG, and to use temp file.

	tmpPngFile, err := ioutil.TempFile("", "imgcat.*.png")
	if err != nil {
		return "", err
	}
	tmpPngPath := tmpPngFile.Name()
	defer os.Remove(tmpPngPath)
	defer tmpPngFile.Close()

	// Rasterize the SVG:
	icon, err := oksvg.ReadIconStream(r)
	if err != nil {
		return "", err
	}
	w := int(icon.ViewBox.W)
	h := int(icon.ViewBox.H)
	icon.SetTarget(0, 0, float64(w), float64(h))
	rgba := image.NewRGBA(image.Rect(0, 0, w, h))
	icon.Draw(rasterx.NewDasher(w, h, rasterx.NewScannerGV(w, h, rgba, rgba.Bounds())), 1)
	// Write rasterized image as PNG:
	err = png.Encode(tmpPngFile, rgba)
	if err != nil {
		tmpPngFile.Close()
		return "", err
	}
	tmpPngFile.Close()

	rPng, err := os.Open(tmpPngPath)
	if err != nil {
		return "", err
	}
	defer rPng.Close()

	img, _, err := imageorient.Decode(rPng)
	if err != nil {
		return "", err
	}
	return imageToString(width, height, url, img)
}
