package common

import (
	"image"

	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
)

// Common defines common UI options and configurations.
type Common struct {
	App    *app.App
	Styles *styles.Styles
}

// Config returns the configuration associated with this [Common] instance.
func (c *Common) Config() *config.Config {
	return c.App.Config()
}

// DefaultCommon returns the default common UI configurations.
func DefaultCommon(app *app.App) *Common {
	s := styles.DefaultStyles()
	return &Common{
		App:    app,
		Styles: &s,
	}
}

// CenterRect returns a new [Rectangle] centered within the given area with the
// specified width and height.
func CenterRect(area uv.Rectangle, width, height int) uv.Rectangle {
	centerX := area.Min.X + area.Dx()/2
	centerY := area.Min.Y + area.Dy()/2
	minX := centerX - width/2
	minY := centerY - height/2
	maxX := minX + width
	maxY := minY + height
	return image.Rect(minX, minY, maxX, maxY)
}
