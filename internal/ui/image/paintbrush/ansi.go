// Package paintbrush renders images as ANSI art. It is a copy of
// github.com/jordanella/go-ansi-paintbrush at commit b7ad996ecf3d (MIT, see
// LICENSE.md), which is no longer maintained. The one change is in
// [Canvas.Paint], which now collects every result before using them.
package paintbrush

import (
	"image"
	"sync"
)

type Canvas struct {
	// Input and Rendering Configuration
	Font                Font              // Font used for rendering
	Image               image.Image       // Input image to be processed
	Width               int               // Output width in characters
	Height              int               // Output height in characters
	AspectRatio         float64           // Aspect ratio for output
	GlyphWidth          int               // Width of each glyph
	GlyphHeight         int               // Height of each glyph
	RuneStart           int               // Starting Unicode code point for character selection
	RuneLimit           int               // Ending Unicode code point for character selection
	Threads             int               // Number of threads for parallel processing
	ForbiddenCharacters map[rune]struct{} // Characters to exclude from rendering
	Weights             map[rune]float64  // Custom weights for character selection

	// Output Results
	Result           string // Raw output string
	ResultC          string // C-style string output
	ResultBash       string // Bash command string output
	ResultRGBABytes  []byte // RGBA byte slice of the rendered image
	ResultRGBAWidth  int    // Width of the RGBA output
	ResultRGBAHeight int    // Height of the RGBA output

	// Internal State
	Progress float32    // Current progress of rendering (0.0 to 1.0)
	mu       sync.Mutex // Mutex for thread-safe operations
}

// New creates and returns a new Canvas instance with default settings.
func New() *Canvas {
	return &Canvas{
		AspectRatio:         1,
		GlyphWidth:          7,
		GlyphHeight:         14,
		RuneStart:           32,
		RuneLimit:           95,
		Threads:             4,
		ForbiddenCharacters: make(map[rune]struct{}),
		Weights:             make(map[rune]float64),
	}
}

// SetWidth sets the width of the output in characters.
func (c *Canvas) SetWidth(width int) {
	c.Width = width
}

// SetHeight sets the height of the output in characters.
func (c *Canvas) SetHeight(height int) {
	c.Height = height
}

// GetImage returns the currently set image.
func (c *Canvas) GetImage() image.Image {
	return c.Image
}

// SetThreads sets the number of threads to use for rendering.
func (c *Canvas) SetThreads(threads int) {
	c.Threads = threads
}

// GetResult returns the raw result string without any formatting.
func (c *Canvas) GetResult() string {
	return c.Result
}

// GetResultC returns the result as a C-style string.
func (c *Canvas) GetResultC() string {
	return c.ResultC
}

// GetResultBash returns the result as a Bash command string.
func (c *Canvas) GetResultBash() string {
	return c.ResultBash
}

// GetResultRGBABytes returns the result as RGBA bytes.
func (c *Canvas) GetResultRGBABytes() []byte {
	return c.ResultRGBABytes
}

// GetResultRGBADimensions returns the width and height of the RGBA result.
func (c *Canvas) GetResultRGBADimensions() (width, height int) {
	return c.ResultRGBAWidth, c.ResultRGBAHeight
}

// AddForbiddenCharacter adds a character to the list of forbidden characters.
func (c *Canvas) AddForbiddenCharacter(char rune) {
	c.ForbiddenCharacters[char] = struct{}{}
}

// RemoveForbiddenCharacter removes a character from the list of forbidden characters.
func (c *Canvas) RemoveForbiddenCharacter(char rune) {
	delete(c.ForbiddenCharacters, char)
}

// ClearForbiddenCharacters removes all characters from the list of forbidden characters.
func (c *Canvas) ClearForbiddenCharacters() {
	c.ForbiddenCharacters = make(map[rune]struct{})
}

// GetForbiddenCharacters returns a slice of all forbidden characters.
func (c *Canvas) GetForbiddenCharacters() []rune {
	chars := make([]rune, 0, len(c.ForbiddenCharacters))
	for char := range c.ForbiddenCharacters {
		chars = append(chars, char)
	}
	return chars
}

// IsForbiddenCharacter checks if a character is in the list of forbidden characters.
func (c *Canvas) IsForbiddenCharacter(char rune) bool {
	_, forbidden := c.ForbiddenCharacters[char]
	return forbidden
}

// SetAspectRatio sets the aspect ratio for the output.
func (c *Canvas) SetAspectRatio(ratio float64) {
	c.AspectRatio = ratio
}

// GetAspectRatio returns the current aspect ratio.
func (c *Canvas) GetAspectRatio() float64 {
	return c.AspectRatio
}

// SetGlyphDimensions sets the width and height of glyphs.
func (c *Canvas) SetGlyphDimensions(width, height int) {
	c.GlyphWidth = width
	c.GlyphHeight = height
}

// GetGlyphDimensions returns the current width and height of glyphs.
func (c *Canvas) GetGlyphDimensions() (width, height int) {
	return c.GlyphWidth, c.GlyphHeight
}

// SetRuneLimits sets the start and end rune limits for character selection.
func (c *Canvas) SetRuneLimits(start, end int) {
	c.RuneStart = start
	c.RuneLimit = end
}

// GetRuneLimits returns the current start and end rune limits.
func (c *Canvas) GetRuneLimits() (int, int) {
	return c.RuneStart, c.RuneLimit
}

// SetWeights sets the weights for characters used in rendering.
func (c *Canvas) SetWeights(weights map[rune]float64) {
	c.Weights = weights
}

// AddWeights adds the provided weights to the existing weight map.
// If a character already has a weight, it will be updated with the new value.
func (c *Canvas) AddWeights(weights map[rune]float64) {
	for char, weight := range weights {
		c.Weights[char] = weight
	}
}
