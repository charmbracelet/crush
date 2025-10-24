package ui

import "github.com/charmbracelet/crush/internal/config"

// Common defines common UI options and configurations.
type Common struct {
	Config *config.Config
	Styles Styles
}

// DefaultCommon returns the default common UI configurations.
func DefaultCommon(cfg *config.Config) *Common {
	return &Common{
		Config: cfg,
		Styles: DefaultStyles(),
	}
}
