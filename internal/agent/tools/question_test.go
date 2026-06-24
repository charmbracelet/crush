package tools

import (
	"encoding/json"
	"testing"

	"github.com/charmbracelet/crush/internal/question"
	"github.com/stretchr/testify/require"
)

func TestQuestionParamsUnmarshalJSON_NativeArray(t *testing.T) {
	t.Parallel()
	input := `{"questions": [{"type": "yes_no", "question": "OK?", "description": "test"}]}`
	var p QuestionParams
	require.NoError(t, json.Unmarshal([]byte(input), &p))
	require.Len(t, p.Questions, 1)
	require.Equal(t, "OK?", p.Questions[0].Question)
}

func TestQuestionParamsUnmarshalJSON_StringEncodedArray(t *testing.T) {
	t.Parallel()
	// Simulates a model that double-serializes the questions field.
	inner := `[{"type":"yes_no","question":"OK?","description":"test"}]`
	encoded, _ := json.Marshal(inner)
	input := `{"questions": ` + string(encoded) + `}`
	var p QuestionParams
	require.NoError(t, json.Unmarshal([]byte(input), &p))
	require.Len(t, p.Questions, 1)
	require.Equal(t, "OK?", p.Questions[0].Question)
}

func TestQuestionParamsUnmarshalJSON_StringEncodedWithWhitespace(t *testing.T) {
	t.Parallel()
	inner := `  [{"type":"single_choice","question":"Pick","description":"d","choices":[{"id":"a","label":"A"}]}]  `
	encoded, _ := json.Marshal(inner)
	input := `{"questions": ` + string(encoded) + `, "confirm_title": "Go?"}`
	var p QuestionParams
	require.NoError(t, json.Unmarshal([]byte(input), &p))
	require.Len(t, p.Questions, 1)
	require.Equal(t, "Pick", p.Questions[0].Question)
	require.Equal(t, "Go?", p.ConfirmTitle)
}

func TestQuestionParamsUnmarshalJSON_InvalidString(t *testing.T) {
	t.Parallel()
	encoded, _ := json.Marshal("not valid json")
	input := `{"questions": ` + string(encoded) + `}`
	var p QuestionParams
	require.Error(t, json.Unmarshal([]byte(input), &p))
}

func TestFormatAnswer_MultiChoiceWithFillIn(t *testing.T) {
	answer := question.Answer{
		SelectedIDs: []string{"speed", "readability"},
		FillInText:  "maintainability",
	}
	resp, err := formatAnswer(&answer, question.TypeMultiChoice)
	require.NoError(t, err)
	require.Contains(t, resp.Content, `User selected: ["speed","readability"]`)
	require.Contains(t, resp.Content, "User provided: maintainability")
}

func TestFormatAnswer_SelectionsOnly(t *testing.T) {
	answer := question.Answer{SelectedIDs: []string{"gardening"}}
	resp, err := formatAnswer(&answer, question.TypeSingleChoice)
	require.NoError(t, err)
	require.Contains(t, resp.Content, `User selected: ["gardening"]`)
	require.NotContains(t, resp.Content, "User provided")
}

func TestFormatAnswer_Skipped(t *testing.T) {
	answer := question.Answer{}
	resp, err := formatAnswer(&answer, question.TypeFreeText)
	require.NoError(t, err)
	require.Equal(t, "User skipped this question", resp.Content)
}
