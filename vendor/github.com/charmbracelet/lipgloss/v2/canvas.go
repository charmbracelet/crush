package lipgloss

import (
	"image"
	"sort"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// Canvas is a collection of layers that can be composed together to form a
// single frame of text.
type Canvas struct {
	layers []*Layer
}

// NewCanvas creates a new Canvas with the given layers. This is a convenient
// way to create a Canvas with one or more layers.
func NewCanvas(layers ...*Layer) (c *Canvas) {
	c = new(Canvas)
	c.AddLayers(layers...)
	return
}

// InBounds returns true if the point is within the bounds of the Canvas.
func (c *Canvas) InBounds(x, y int) bool {
	return image.Pt(x, y).In(c.Bounds())
}

// Bounds returns the bounds of the Canvas.
func (c *Canvas) Bounds() image.Rectangle {
	// Figure out the size of the canvas
	x0, y0, x1, y1 := 0, 0, 0, 0
	for _, l := range c.layers {
		x0 = min(x0, l.rect.Min.X)
		y0 = min(y0, l.rect.Min.Y)
		x1 = max(x1, l.rect.Max.X)
		y1 = max(y1, l.rect.Max.Y)
	}

	// Adjust the size of the canvas if it's negative
	x0, y0 = max(x0, 0), max(y0, 0)

	return image.Rect(x0, y0, x1, y1)
}

// Hit returns the [Layer.ID] at the given point. If no Layer is found,
// nil is returned.
func (c *Canvas) Hit(x, y int) string {
	for i := len(c.layers) - 1; i >= 0; i-- {
		if c.layers[i].InBounds(x, y) {
			return c.layers[i].Hit(x, y).GetID()
		}
	}
	return ""
}

// AddLayers adds the given layers to the Canvas.
func (c *Canvas) AddLayers(layers ...*Layer) {
	c.layers = append(c.layers, layers...)
	sortLayers(c.layers, false)
}

// Get returns the Layer with the given ID. If the ID is not found, nil is
// returned.
func (c *Canvas) Get(id string) *Layer {
	for _, l := range c.layers {
		if la := l.Get(id); la != nil {
			return la
		}
	}
	return nil
}

// Draw draws the [Canvas] into the given screen and area.
func (c *Canvas) Draw(scr uv.Screen, area image.Rectangle) {
	for _, l := range c.layers {
		l.Draw(scr, area)
	}
}

// Render renders the Canvas to a string.
func (c *Canvas) Render() string {
	area := c.Bounds()
	buf := uv.NewScreenBuffer(area.Dx(), area.Dy())
	buf.Method = ansi.GraphemeWidth
	c.Draw(buf, area)
	return buf.Render()
}

// Layer represents a window layer that can be composed with other layers.
type Layer struct {
	rect     image.Rectangle
	zIndex   int
	children []*Layer
	id       string
	content  string
}

// NewLayer creates a new Layer with the given content. It calculates the size
// based on the widest line and the number of lines in the content.
func NewLayer(content string) (l *Layer) {
	l = new(Layer)
	l.content = content
	height := Height(content)
	width := Width(content)
	l.rect = image.Rect(0, 0, width, height)
	return l
}

// InBounds returns true if the point is within the bounds of the Layer.
func (l *Layer) InBounds(x, y int) bool {
	return image.Pt(x, y).In(l.Bounds())
}

// Bounds returns the bounds of the Layer.
func (l *Layer) Bounds() image.Rectangle {
	return l.rect
}

// Hit returns the [Layer.ID] at the given point. If no Layer is found,
// returns nil is returned.
func (l *Layer) Hit(x, y int) *Layer {
	// Reverse the order of the layers so that the top-most layer is checked
	// first.
	for i := len(l.children) - 1; i >= 0; i-- {
		if l.children[i].InBounds(x, y) {
			return l.children[i].Hit(x, y)
		}
	}

	if image.Pt(x, y).In(l.Bounds()) {
		return l
	}

	return nil
}

// ID sets the ID of the Layer. The ID can be used to identify the Layer when
// performing hit tests.
func (l *Layer) ID(id string) *Layer {
	l.id = id
	return l
}

// GetID returns the ID of the Layer.
func (l *Layer) GetID() string {
	return l.id
}

// X sets the x-coordinate of the Layer.
func (l *Layer) X(x int) *Layer {
	l.rect = l.rect.Add(image.Pt(x, 0))
	return l
}

// Y sets the y-coordinate of the Layer.
func (l *Layer) Y(y int) *Layer {
	l.rect = l.rect.Add(image.Pt(0, y))
	return l
}

// Z sets the z-index of the Layer.
func (l *Layer) Z(z int) *Layer {
	l.zIndex = z
	return l
}

// GetX returns the x-coordinate of the Layer.
func (l *Layer) GetX() int {
	return l.rect.Min.X
}

// GetY returns the y-coordinate of the Layer.
func (l *Layer) GetY() int {
	return l.rect.Min.Y
}

// GetZ returns the z-index of the Layer.
func (l *Layer) GetZ() int {
	return l.zIndex
}

// Width sets the width of the Layer.
func (l *Layer) Width(width int) *Layer {
	l.rect.Max.X = l.rect.Min.X + width
	return l
}

// Height sets the height of the Layer.
func (l *Layer) Height(height int) *Layer {
	l.rect.Max.Y = l.rect.Min.Y + height
	return l
}

// GetWidth returns the width of the Layer.
func (l *Layer) GetWidth() int {
	return l.rect.Dx()
}

// GetHeight returns the height of the Layer.
func (l *Layer) GetHeight() int {
	return l.rect.Dy()
}

// AddLayers adds child layers to the Layer.
func (l *Layer) AddLayers(layers ...*Layer) *Layer {
	// Make children relative to the parent
	for _, child := range layers {
		child.rect = child.rect.Add(l.rect.Min)
		child.zIndex += l.zIndex
	}
	l.children = append(l.children, layers...)
	sortLayers(l.children, false)
	return l
}

// SetContent sets the content of the Layer.
func (l *Layer) SetContent(content string) *Layer {
	l.content = content
	return l
}

// Content returns the content of the Layer.
func (l *Layer) Content() string {
	return l.content
}

// Draw draws the Layer onto the given screen buffer.
func (l *Layer) Draw(scr uv.Screen, _ image.Rectangle) {
	ss := uv.NewStyledString(l.content)
	ss.Draw(scr, l.Bounds())
	for _, child := range l.children {
		ss := uv.NewStyledString(child.content)
		ss.Draw(scr, child.Bounds())
	}
}

// Get returns the Layer with the given ID. If the ID is not found, it returns
// nil.
func (l *Layer) Get(id string) *Layer {
	if l.id == id {
		return l
	}
	for _, child := range l.children {
		if child.id == id {
			return child
		}
	}
	return nil
}

// sortLayers sorts the layers by z-index, from lowest to highest.
func sortLayers(ls []*Layer, reverse bool) {
	if reverse {
		sort.Stable(sort.Reverse(layers(ls)))
	} else {
		sort.Stable(layers(ls))
	}
}

// layers implements sort.Interface for []*Layer.
type layers []*Layer

func (l layers) Len() int           { return len(l) }
func (l layers) Less(i, j int) bool { return l[i].zIndex < l[j].zIndex }
func (l layers) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }
