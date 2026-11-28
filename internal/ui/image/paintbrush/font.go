package paintbrush

import (
	"embed"
	"fmt"
	"image"
	"os"

	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

//go:embed assets/FiraMono-Regular.ttf
var EmbeddedFonts embed.FS
var FiraMonoRegular = "assets/FiraMono-Regular.ttf"

type Font struct {
	GlyphHeight int
	GlyphWidth  int
	Aspect      float64
	Glyphs      map[rune]Glyph
}

type Glyph struct {
	Unicode int
	UTF8    string
	Pixels  []uint8
	Weight  float64
}

// LoadFont loads a font from the specified file path.
func (c *Canvas) LoadFont(path string) error {
	fontData, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	err = c.SetFont(fontData)
	if err != nil {
		return err
	}
	return nil
}

// SetFont sets the font using the provided byte slice of font data.
func (c *Canvas) SetFont(data []byte) error {
	f, err := truetype.Parse(data)
	if err != nil {
		return err
	}

	// Set fixed glyph dimensions
	c.Font.GlyphWidth = c.GlyphWidth // You can adjust these values
	c.Font.GlyphHeight = c.GlyphHeight
	c.Font.Aspect = (float64(c.Font.GlyphHeight) / float64(c.Font.GlyphWidth)) * c.AspectRatio

	// Set font size and DPI
	opts := truetype.Options{
		Size:    float64(c.Font.GlyphHeight), // Use glyph height as font size
		DPI:     72,
		Hinting: font.HintingFull,
	}

	face := truetype.NewFace(f, &opts)

	c.Font.Glyphs = make(map[rune]Glyph)
	for r := rune(c.RuneStart); r < rune(c.RuneLimit); r++ {
		glyph, err := c.generateGlyph(face, r)
		if err != nil {
			fmt.Printf("Error generating glyph for rune %d: %v\n", r, err)
			continue
		}
		glyph.Weight = 1.0 // Default weight
		c.Font.Glyphs[r] = glyph
	}

	// Apply custom weights
	for char, weight := range c.Weights {
		if glyph, exists := c.Font.Glyphs[char]; exists {
			glyph.Weight = weight
			c.Font.Glyphs[char] = glyph
		} else {
			glyph, err := c.generateGlyph(face, char)
			if err != nil {
				fmt.Printf("Error generating glyph for rune %d: %v\n", char, err)
				continue
			}
			glyph.Weight = weight
			c.Font.Glyphs[char] = glyph
		}
	}

	return nil
}

func (c *Canvas) generateGlyph(face font.Face, r rune) (Glyph, error) {
	// Create an image to draw the glyph
	img := image.NewGray(image.Rect(0, 0, c.Font.GlyphWidth, c.Font.GlyphHeight))
	d := &font.Drawer{
		Dst:  img,
		Src:  image.White,
		Face: face,
	}

	// Get glyph metrics
	advance, ok := face.GlyphAdvance(r)
	if !ok {
		return Glyph{}, fmt.Errorf("glyph not found for rune %v", r)
	}

	// Calculate position to center the glyph
	x := (fixed.I(c.Font.GlyphWidth) - advance) / 2
	y := fixed.I(c.Font.GlyphHeight * 4 / 5) // Adjust this value to vertically center the glyph

	// Draw the glyph
	d.Dot = fixed.Point26_6{X: x, Y: y}
	d.DrawString(string(r))

	// Convert image to pixel array
	pixels := make([]uint8, c.Font.GlyphWidth*c.Font.GlyphHeight)
	for y := 0; y < c.Font.GlyphHeight; y++ {
		for x := 0; x < c.Font.GlyphWidth; x++ {
			pixels[y*c.Font.GlyphWidth+x] = img.GrayAt(x, y).Y
		}
	}

	return Glyph{
		Unicode: int(r),
		UTF8:    string(r),
		Pixels:  pixels,
	}, nil
}
