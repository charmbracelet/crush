package keys

import (
	"github.com/charmbracelet/bubbles/v2/key"
)

// Escape creates a standard escape key binding that responds to both "esc" and "alt+esc"
func Escape(opts ...key.BindingOpt) key.Binding {
	// Default options include both "esc" and "alt+esc" keys
	defaultOpts := []key.BindingOpt{
		key.WithKeys("esc", "alt+esc"),
	}
	
	// Append any additional options provided
	allOpts := append(defaultOpts, opts...)
	
	return key.NewBinding(allOpts...)
}