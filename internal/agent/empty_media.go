package agent

import (
	"charm.land/fantasy"
)

// emptyMediaNote replaces a tool's media when it carries no bytes, since
// providers reject an empty media block by failing the whole request.
const emptyMediaNote = "The tool returned empty media data, so there is nothing to show here."

// emptyMediaText is the text a tool result carries once its empty media is
// dropped, keeping whatever the tool said alongside it.
func emptyMediaText(text string) string {
	if text == "" {
		return emptyMediaNote
	}
	return text + "\n" + emptyMediaNote
}

// dropEmptyToolMedia replaces every tool result whose media has no data with
// a text note, so no tool can put an empty media block on the wire.
func dropEmptyToolMedia(messages []fantasy.Message) []fantasy.Message {
	for i, msg := range messages {
		if msg.Role != fantasy.MessageRoleTool {
			continue
		}
		var content []fantasy.MessagePart
		for j, part := range msg.Content {
			result, ok := fantasy.AsMessagePart[fantasy.ToolResultPart](part)
			if !ok {
				continue
			}
			media, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentMedia](result.Output)
			if !ok || media.Data != "" {
				continue
			}
			if content == nil {
				content = append([]fantasy.MessagePart(nil), msg.Content...)
			}
			result.Output = fantasy.ToolResultOutputContentText{Text: emptyMediaText(media.Text)}
			content[j] = result
		}
		if content != nil {
			messages[i].Content = content
		}
	}
	return messages
}
