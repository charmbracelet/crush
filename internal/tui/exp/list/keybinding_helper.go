package list

import (
	"regexp"

	"charm.land/bubbles/v2/key"
)

// KeyBindingHelper provides utilities for updating key bindings
type KeyBindingHelper struct {
	alphanumericRegex *regexp.Regexp
}

// NewKeyBindingHelper creates a new key binding helper
func NewKeyBindingHelper(alphanumericRegex *regexp.Regexp) *KeyBindingHelper {
	return &KeyBindingHelper{
		alphanumericRegex: alphanumericRegex,
	}
}

// RemoveLettersAndNumbers removes letters and numbers from bindings
func (kb *KeyBindingHelper) RemoveLettersAndNumbers(bindings []string) []string {
	var keep []string
	for _, b := range bindings {
		if len(b) != 1 {
			keep = append(keep, b)
			continue
		}
		if b == " " {
			continue
		}
		m := kb.alphanumericRegex.MatchString(b)
		if !m {
			keep = append(keep, b)
		}
	}
	return keep
}

// UpdateBinding updates a binding with filtered keys
func (kb *KeyBindingHelper) UpdateBinding(binding key.Binding) key.Binding {
	newKeys := kb.RemoveLettersAndNumbers(binding.Keys())
	if len(newKeys) == 0 {
		binding.SetEnabled(false)
		return binding
	}
	binding.SetKeys(newKeys...)
	return binding
}
