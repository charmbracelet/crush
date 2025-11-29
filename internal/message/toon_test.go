package message

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConvertJSONToTOON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "non-json text",
			input:    "plain text",
			expected: "plain text",
		},
		{
			name:     "simple json object",
			input:    `{"name":"Alice","age":30}`,
			expected: "age: 30\nname: Alice",
		},
		{
			name:     "json array",
			input:    `[{"id":1,"name":"Alice"},{"id":2,"name":"Bob"}]`,
			expected: "[2]{id,name}:\n  1,Alice\n  2,Bob",
		},
		{
			name:     "malformed json",
			input:    `{"name":"Alice"`,
			expected: `{"name":"Alice"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := ConvertJSONToTOON(test.input)
			assert.Equal(t, test.expected, result)
		})
	}
}
