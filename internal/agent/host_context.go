package agent

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/crush/internal/proto"
)

// notesPrompt tells the model what the user did since its last turn, ahead
// of the prompt. It is "" without notes.
func notesPrompt(notes []string) string {
	if len(notes) == 0 {
		return ""
	}
	return "<user_notes>\nThings the user did since your last message. Take them into account.\n" +
		strings.Join(notes, "\n\n") + "\n</user_notes>\n\n"
}

// hostContextPrompt tells the model what the user has in view in the host
// application, ahead of the prompt it is about. It is "" without a host.
func hostContextPrompt(hc proto.HostContext) string {
	if hc.ActiveFile == "" && len(hc.OpenFiles) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("<host_context>\nThe user is working in an editor next to you. Use this to resolve what they mean by \"here\", \"this file\" or \"this line\".\n")
	if hc.ActiveFile != "" {
		fmt.Fprintf(&b, "Current file: %s", hc.ActiveFile)
		if hc.Line > 0 {
			fmt.Fprintf(&b, ", cursor on line %d", hc.Line)
		}
		b.WriteString("\n")
	}
	if len(hc.OpenFiles) > 0 {
		b.WriteString("Open files:\n")
		for _, f := range hc.OpenFiles {
			fmt.Fprintf(&b, "- %s\n", f)
		}
	}
	b.WriteString("</host_context>\n\n")
	return b.String()
}
