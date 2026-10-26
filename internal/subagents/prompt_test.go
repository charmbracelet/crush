package subagents

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToPromptXML(t *testing.T) {
	t.Parallel()

	active := []*Subagent{
		{Name: "go-test-writer", Description: "Writes Go tests before implementation."},
		{Name: "code-reviewer", Description: "Reviews diffs & flags issues."},
	}

	xml := ToPromptXML(active)

	require.Contains(t, xml, "<available_subagents>")
	require.Contains(t, xml, "<name>go-test-writer</name>")
	require.Contains(t, xml, "<description>Writes Go tests before implementation.</description>")
	require.Contains(t, xml, "&amp;") // XML escaping
	require.Contains(t, xml, "</available_subagents>")
}

func TestToPromptXMLEmpty(t *testing.T) {
	t.Parallel()
	require.Empty(t, ToPromptXML(nil))
	require.Empty(t, ToPromptXML([]*Subagent{}))
}

func TestToPromptXML_EscapesSpecialChars(t *testing.T) {
	t.Parallel()

	active := []*Subagent{
		{Name: "html-writer", Description: `Handles <script> tags & "quotes" 'n stuff.`},
	}

	xml := ToPromptXML(active)

	require.NotContains(t, xml, "<script>")
	require.Contains(t, xml, "&lt;script&gt;")
	require.Contains(t, xml, "&amp;")
	require.Contains(t, xml, "&quot;quotes&quot;")
	require.Contains(t, xml, "&apos;n")
}
