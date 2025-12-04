// Based on the implementation by @trashhalo at:
// https://github.com/trashhalo/imgcat
package image

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
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

func loadURL(url string) tea.Cmd {
	var r io.ReadCloser
	var err error

	if strings.HasPrefix(url, "http") {
		var resp *http.Request
		resp, err = http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
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
	defer msg.Close()

	img, err := readerToImage(m.width, m.height, m.url, msg)
	if err != nil {
		return m, func() tea.Msg { return errMsg{err} }
	}
	m.image = img
	return m, nil
}

func imageToString(width, height uint, img image.Image) (string, error) {
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
		for x := range w {
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

func readerToImage(width uint, height uint, url string, r io.Reader) (string, error) {
	if strings.HasSuffix(strings.ToLower(url), ".svg") {
		return svgToImage(width, height, r)
	}

	img, _, err := imageorient.Decode(r)
	if err != nil {
		return "", err
	}

	return imageToString(width, height, img)
}

func svgToImage(width uint, height uint, r io.Reader) (string, error) {
	// Original author: https://stackoverflow.com/users/10826783/usual-human
	// https://stackoverflow.com/questions/42993407/how-to-create-and-export-svg-to-png-jpeg-in-golang
	// Adapted to use size from SVG, and to use temp file.

	tmpPngFile, err := os.CreateTemp("", "img.*.png")
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
	return imageToString(width, height, img)
}

// ImageFromBase64 renders an image from base64-encoded data.
func ImageFromBase64(width, height uint, data, mediaType string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", err
	}

	r := bytes.NewReader(decoded)

	if strings.Contains(mediaType, "svg") {
		return svgToImage(width, height, r)
	}

	img, _, err := imageorient.Decode(r)
	if err != nil {
		return "", err
	}

	return imageToString(width, height, img)
}
