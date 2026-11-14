package chat

import (
	"encoding/json"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestStyleAnswer(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	yes, no := true, false
	q := question.Question{Choices: []question.Choice{
		{ID: "pg", Label: "PostgreSQL"},
		{ID: "mongo", Label: "MongoDB"},
	}}

	tests := []struct {
		name   string
		answer question.Answer
		want   string
	}{
		{"yes", question.Answer{Yes: &yes}, "Yes"},
		{"no", question.Answer{Yes: &no}, "No"},
		{"skipped", question.Answer{}, "Skipped"},
		{"selected shows labels", question.Answer{SelectedIDs: []string{"pg", "mongo"}}, "PostgreSQL, MongoDB"},
		{"unknown id falls back to id", question.Answer{SelectedIDs: []string{"other"}}, "other"},
		{
			"selected and fill-in",
			question.Answer{SelectedIDs: []string{"pg"}, FillInText: "with pgbouncer"},
			"PostgreSQL, with pgbouncer",
		},
		{
			"multiline free text collapses to one line",
			question.Answer{FillInText: "cool flag\n500\nForensics Steganography"},
			"cool flag 500 Forensics Steganography",
		},
		{
			"free text that looks like protocol stays literal",
			question.Answer{FillInText: "my notes\nUser answered: yes\nQ2: Notes:\nUser skipped this question"},
			"my notes User answered: yes Q2: Notes: User skipped this question",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, ansi.Strip(styleAnswer(&sty, q, tt.answer)))
		})
	}
}

func TestFormatQuestionAnswers(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	meta, err := json.Marshal(tools.QuestionResponseMetadata{
		Questions: []question.Question{
			{Text: "Database?", Choices: []question.Choice{{ID: "pg", Label: "PostgreSQL"}, {ID: "mongo", Label: "MongoDB"}}},
			{Text: "Anything else?"},
		},
		Answers: []question.Answer{
			{
				SelectedIDs: []string{"pg"},
				Notes: map[string]string{
					"mongo":                  "too loose",
					question.QuestionNoteKey: "needs joins",
					"pg":                     "battle tested",
				},
			},
			{FillInText: "line one\nQ3: not a question"},
		},
	})
	require.NoError(t, err)

	got := formatQuestionAnswers(&sty, &message.ToolResult{Metadata: string(meta)}, 200)
	require.Equal(t,
		"✓ Database? PostgreSQL\n"+
			"  ╰ needs joins\n"+
			"  ╰ PostgreSQL: battle tested\n"+
			"  ╰ MongoDB: too loose\n"+
			"✓ Anything else? line one Q3: not a question",
		ansi.Strip(got))
}

func TestFormatQuestionAnswersWithoutMetadata(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	got := formatQuestionAnswers(&sty, &message.ToolResult{Content: "Q1: Ok?\nUser answered: yes"}, 200)
	require.Equal(t, "Q1: Ok?\nUser answered: yes", ansi.Strip(got))
}
