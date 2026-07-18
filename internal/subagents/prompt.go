package subagents

import (
	"fmt"
	"strings"
)

var promptReplacer = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&apos;")

func escape(s string) string {
	return promptReplacer.Replace(s)
}

// ToPromptXML generates XML for injection into the coder system prompt so the
// model can proactively notice a matching subagent before deciding how to
// handle a task. Mirrors skills.ToPromptXML.
func ToPromptXML(active []*Subagent) string {
	if len(active) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("<available_subagents>\n")
	for _, sa := range active {
		sb.WriteString("  <subagent>\n")
		fmt.Fprintf(&sb, "    <name>%s</name>\n", escape(sa.Name))
		fmt.Fprintf(&sb, "    <description>%s</description>\n", escape(sa.Description))
		sb.WriteString("  </subagent>\n")
	}
	sb.WriteString("</available_subagents>")
	return sb.String()
}
