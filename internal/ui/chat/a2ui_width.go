package chat

// a2uiCardChromeWidth is the border+padding chrome a2tea's Card renderer
// subtracts from its budget (a2tea render/card.go: w := s.width - 4).
const a2uiCardChromeWidth = 4

// ToolA2UISurfaceWidth returns the interior width, in cells, of a generic
// tool result's A2UI surface card for a chat viewport of the given width —
// the number servers pre-rendering bar geometry should size rows to (the
// ?w= hint read_mcp_resource sends).
//
// It mirrors the actual render chain rather than hand-summed constants:
// the message list reserves one cell for its scrollbar, the tool item and
// RenderTool each apply cappedMessageWidth, the body is indented by
// toolBodyLeftPaddingTotal, and a2tea's card chrome takes the rest. Keeping
// the computation here, next to the constants it depends on, is what stops
// the hint drifting from the real card interior when the layout changes.
//
// Returns 0 (no hint) when the viewport is too small for a meaningful
// width.
func ToolA2UISurfaceWidth(chatWidth int) int {
	listWidth := chatWidth - 1 // scrollbar reservation in model/chat.go SetSize
	interior := cappedMessageWidth(cappedMessageWidth(listWidth)) - toolBodyLeftPaddingTotal - a2uiCardChromeWidth
	return max(0, interior)
}
