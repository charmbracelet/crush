package playground

import (
	"github.com/nom-nom-hub/blush/internal/tui/components/playground"
	"github.com/nom-nom-hub/blush/internal/tui/page"
)

// PlaygroundPageID is the unique identifier for the playground page
var PlaygroundPageID page.PageID = "playground"

// New creates a new playground page
func New() *playground.Model {
	return playground.New()
}