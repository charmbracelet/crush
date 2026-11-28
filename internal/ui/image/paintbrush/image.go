package paintbrush

import (
	"fmt"
	"image"
	"math"
	"os"
)

// LoadImage loads an image from the specified file path.
func (c *Canvas) LoadImage(path string) error {
	file, err := os.Open(path)
	if err != nil {
		fmt.Printf("Error opening image file: %v\n", err)
		return err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		fmt.Printf("Error decoding image: %v\n", err)
		return err
	}

	c.SetImage(img)
	return nil
}

// SetImage sets the image to be rendered.
func (c *Canvas) SetImage(img image.Image) {
	c.Image = img
}

func (c *Canvas) readImageColor(x, y float64) Vec4 {
	if x >= float64(c.Image.Bounds().Dx()) || x < 0 || y >= float64(c.Image.Bounds().Dy()) || y < 0 {
		return Vec4{}
	}
	r, g, b, a := c.Image.At(int(math.Round(x)), int(math.Round(y))).RGBA()
	return Vec4{
		R: float64(r) / 65535.0,
		G: float64(g) / 65535.0,
		B: float64(b) / 65535.0,
		A: float64(a) / 65535.0,
	}
}
