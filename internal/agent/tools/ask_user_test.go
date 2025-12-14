package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateAskUserParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		params  AskUserParams
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty questions",
			params:  AskUserParams{Questions: []AskUserQuestion{}},
			wantErr: true,
			errMsg:  "at least 1 question",
		},
		{
			name: "too many questions",
			params: AskUserParams{
				Questions: []AskUserQuestion{
					{Question: "Q1", Header: "H1", Options: []AskUserOption{{Label: "A"}, {Label: "B"}}},
					{Question: "Q2", Header: "H2", Options: []AskUserOption{{Label: "A"}, {Label: "B"}}},
					{Question: "Q3", Header: "H3", Options: []AskUserOption{{Label: "A"}, {Label: "B"}}},
					{Question: "Q4", Header: "H4", Options: []AskUserOption{{Label: "A"}, {Label: "B"}}},
					{Question: "Q5", Header: "H5", Options: []AskUserOption{{Label: "A"}, {Label: "B"}}},
				},
			},
			wantErr: true,
			errMsg:  "maximum 4 questions",
		},
		{
			name: "header too long",
			params: AskUserParams{
				Questions: []AskUserQuestion{{
					Question: "Test question",
					Header:   "ThisHeaderIsTooLong",
					Options:  []AskUserOption{{Label: "A"}, {Label: "B"}},
				}},
			},
			wantErr: true,
			errMsg:  "12 characters",
		},
		{
			name: "too few options",
			params: AskUserParams{
				Questions: []AskUserQuestion{{
					Question: "Test question",
					Header:   "Test",
					Options:  []AskUserOption{{Label: "A"}},
				}},
			},
			wantErr: true,
			errMsg:  "2-4 options",
		},
		{
			name: "too many options",
			params: AskUserParams{
				Questions: []AskUserQuestion{{
					Question: "Test question",
					Header:   "Test",
					Options: []AskUserOption{
						{Label: "A"},
						{Label: "B"},
						{Label: "C"},
						{Label: "D"},
						{Label: "E"},
					},
				}},
			},
			wantErr: true,
			errMsg:  "2-4 options",
		},
		{
			name: "empty question text",
			params: AskUserParams{
				Questions: []AskUserQuestion{{
					Question: "",
					Header:   "Test",
					Options:  []AskUserOption{{Label: "A"}, {Label: "B"}},
				}},
			},
			wantErr: true,
			errMsg:  "question text cannot be empty",
		},
		{
			name: "whitespace only question text",
			params: AskUserParams{
				Questions: []AskUserQuestion{{
					Question: "   ",
					Header:   "Test",
					Options:  []AskUserOption{{Label: "A"}, {Label: "B"}},
				}},
			},
			wantErr: true,
			errMsg:  "question text cannot be empty",
		},
		{
			name: "empty option label",
			params: AskUserParams{
				Questions: []AskUserQuestion{{
					Question: "Test question",
					Header:   "Test",
					Options:  []AskUserOption{{Label: ""}, {Label: "B"}},
				}},
			},
			wantErr: true,
			errMsg:  "option label cannot be empty",
		},
		{
			name: "valid single question",
			params: AskUserParams{
				Questions: []AskUserQuestion{{
					Question: "Which framework should we use?",
					Header:   "Framework",
					Options: []AskUserOption{
						{Label: "React", Description: "A JavaScript library"},
						{Label: "Vue", Description: "Progressive framework"},
					},
				}},
			},
			wantErr: false,
		},
		{
			name: "valid multiple questions",
			params: AskUserParams{
				Questions: []AskUserQuestion{
					{
						Question: "Which database?",
						Header:   "Database",
						Options: []AskUserOption{
							{Label: "PostgreSQL"},
							{Label: "MySQL"},
						},
					},
					{
						Question:    "Which features?",
						Header:      "Features",
						MultiSelect: true,
						Options: []AskUserOption{
							{Label: "Logging"},
							{Label: "Metrics"},
							{Label: "Tracing"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid with max questions",
			params: AskUserParams{
				Questions: []AskUserQuestion{
					{Question: "Q1", Header: "H1", Options: []AskUserOption{{Label: "A"}, {Label: "B"}}},
					{Question: "Q2", Header: "H2", Options: []AskUserOption{{Label: "A"}, {Label: "B"}}},
					{Question: "Q3", Header: "H3", Options: []AskUserOption{{Label: "A"}, {Label: "B"}}},
					{Question: "Q4", Header: "H4", Options: []AskUserOption{{Label: "A"}, {Label: "B"}}},
				},
			},
			wantErr: false,
		},
		{
			name: "valid with max options",
			params: AskUserParams{
				Questions: []AskUserQuestion{{
					Question: "Test question",
					Header:   "Test",
					Options: []AskUserOption{
						{Label: "A"},
						{Label: "B"},
						{Label: "C"},
						{Label: "D"},
					},
				}},
			},
			wantErr: false,
		},
		{
			name: "valid header exactly 12 chars",
			params: AskUserParams{
				Questions: []AskUserQuestion{{
					Question: "Test question",
					Header:   "123456789012",
					Options:  []AskUserOption{{Label: "A"}, {Label: "B"}},
				}},
			},
			wantErr: false,
		},
		{
			name: "header 13 chars fails",
			params: AskUserParams{
				Questions: []AskUserQuestion{{
					Question: "Test question",
					Header:   "1234567890123",
					Options:  []AskUserOption{{Label: "A"}, {Label: "B"}},
				}},
			},
			wantErr: true,
			errMsg:  "12 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateAskUserParams(tt.params)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAskUserQuestionStruct(t *testing.T) {
	t.Parallel()

	q := AskUserQuestion{
		Question:    "Which auth method?",
		Header:      "Auth",
		MultiSelect: false,
		Options: []AskUserOption{
			{Label: "JWT", Description: "JSON Web Tokens"},
			{Label: "Session", Description: "Server-side sessions"},
		},
	}

	assert.Equal(t, "Which auth method?", q.Question)
	assert.Equal(t, "Auth", q.Header)
	assert.False(t, q.MultiSelect)
	assert.Len(t, q.Options, 2)
	assert.Equal(t, "JWT", q.Options[0].Label)
	assert.Equal(t, "JSON Web Tokens", q.Options[0].Description)
}

func TestAskUserOptionStruct(t *testing.T) {
	t.Parallel()

	opt := AskUserOption{
		Label:       "PostgreSQL",
		Description: "Relational database with advanced features",
	}

	assert.Equal(t, "PostgreSQL", opt.Label)
	assert.Equal(t, "Relational database with advanced features", opt.Description)
}

func TestAskUserParamsStruct(t *testing.T) {
	t.Parallel()

	params := AskUserParams{
		Questions: []AskUserQuestion{
			{
				Question: "Q1",
				Header:   "H1",
				Options: []AskUserOption{
					{Label: "A"},
					{Label: "B"},
				},
			},
		},
	}

	assert.Len(t, params.Questions, 1)
	assert.Equal(t, "Q1", params.Questions[0].Question)
}
