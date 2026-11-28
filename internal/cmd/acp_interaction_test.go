package cmd

import (
	"context"
	"strings"
	"testing"

	acp "github.com/coder/acp-go-sdk"
	"github.com/stretchr/testify/require"
)

func TestLineInputSerializesReads(t *testing.T) {
	input := newLineInput(strings.NewReader("first\nsecond\n"))
	first, ok := input.readLine("")
	require.True(t, ok)
	require.Equal(t, "first", first)
	second, ok := input.readLine("")
	require.True(t, ok)
	require.Equal(t, "second", second)
}

func TestSafeTerminalText(t *testing.T) {
	got := safeTerminalText("a\x1b[31mb\u202Ec\u200bd\u0007e\nf")
	require.Equal(t, "abcde\nf", got)
}

func TestPermissionHandler(t *testing.T) {
	input := newLineInput(strings.NewReader("2\n"))
	handler := newPermissionHandler(input)
	resp, err := handler(context.Background(), acp.RequestPermissionRequest{
		ToolCall: acp.ToolCallUpdate{Title: acp.PatchSet("bash")},
		Options: []acp.PermissionOption{
			{OptionId: "allow", Name: "Allow"},
			{OptionId: "reject", Name: "Reject"},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Outcome.Selected)
	require.Equal(t, acp.PermissionOptionId("reject"), resp.Outcome.Selected.OptionId)

	input = newLineInput(strings.NewReader(""))
	handler = newPermissionHandler(input)
	resp, err = handler(context.Background(), acp.RequestPermissionRequest{
		Options: []acp.PermissionOption{{OptionId: "allow", Name: "Allow"}},
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Outcome.Cancelled)
}

func TestParseElicitationValues(t *testing.T) {
	value, err := parseElicitationValue(acp.ElicitationPropertySchema{
		String: &acp.ElicitationPropertySchemaString{Type: "string"},
	}, "hello")
	require.NoError(t, err)
	require.Equal(t, acp.ElicitationContentValueString("hello"), *value.String)

	value, err = parseElicitationValue(acp.ElicitationPropertySchema{
		Boolean: &acp.ElicitationPropertySchemaBoolean{Type: "boolean"},
	}, "true")
	require.NoError(t, err)
	require.Equal(t, acp.ElicitationContentValueBoolean(true), *value.Boolean)

	_, err = parseElicitationValue(acp.ElicitationPropertySchema{
		String: &acp.ElicitationPropertySchemaString{Type: "string", Enum: []string{"a", "b"}},
	}, "c")
	require.Error(t, err)
}

func TestElicitationCancel(t *testing.T) {
	response := cancelElicitation()
	require.NotNil(t, response.Cancel)
	require.Equal(t, "cancel", response.Cancel.Action)
}
