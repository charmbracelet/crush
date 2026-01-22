package list

// FocusedRenderCallback is a helper function that returns a render callback
// that marks items as focused during rendering.
func FocusedRenderCallback(list *List) RenderCallback {
	return func(idx, selectedIdx int, item Item) Item {
		if focusable, ok := item.(Focusable); ok {
			focusable.SetFocused(list.Focused() && idx == selectedIdx)
			return focusable.(Item)
		}
		return item
	}
}
