package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFrontmatter_ValidFrontmatterWithAllFields(t *testing.T) {
	content := `---
description: Review a pull request
argument-hint: "[pr-number] [priority]"
allowed-tools:
  - View
  - Edit
  - Grep
---
# Review PR

This command reviews a pull request.
`

	fm, remaining, err := ParseFrontmatter(content)

	require.NoError(t, err)
	assert.Equal(t, "Review a pull request", fm.Description)
	assert.Equal(t, "[pr-number] [priority]", fm.ArgumentHint)
	assert.Equal(t, []string{"View", "Edit", "Grep"}, fm.AllowedTools)
	assert.Contains(t, remaining, "# Review PR")
	assert.Contains(t, remaining, "This command reviews a pull request")
}

func TestParseFrontmatter_ValidFrontmatterMinimal(t *testing.T) {
	content := `---
description: Simple command
---
# Content here
`

	fm, remaining, err := ParseFrontmatter(content)

	require.NoError(t, err)
	assert.Equal(t, "Simple command", fm.Description)
	assert.Equal(t, "", fm.ArgumentHint)
	assert.Nil(t, fm.AllowedTools)
	assert.Contains(t, remaining, "# Content here")
}

func TestParseFrontmatter_MissingFrontmatter(t *testing.T) {
	content := `# No frontmatter here

Just regular markdown content.
`

	fm, remaining, err := ParseFrontmatter(content)

	require.NoError(t, err)
	assert.Equal(t, "", fm.Description)
	assert.Equal(t, "", fm.ArgumentHint)
	assert.Nil(t, fm.AllowedTools)
	assert.Equal(t, content, remaining)
}

func TestParseFrontmatter_EmptyContent(t *testing.T) {
	content := ""

	fm, remaining, err := ParseFrontmatter(content)

	require.NoError(t, err)
	assert.Equal(t, "", fm.Description)
	assert.Equal(t, "", remaining)
}

func TestParseFrontmatter_InvalidYAMLSyntax(t *testing.T) {
	content := `---
description: Unclosed quote
argument-hint: "[missing
---
# Content
`

	fm, remaining, err := ParseFrontmatter(content)

	// Should not error, but return empty frontmatter
	require.NoError(t, err)
	assert.Equal(t, "", fm.Description)
	assert.Equal(t, "", fm.ArgumentHint)
	// Should return original content since YAML parsing failed
	assert.Contains(t, remaining, "---")
}

func TestParseFrontmatter_MissingIndividualFields(t *testing.T) {
	tests := []struct {
		name    string
		content string
		check   func(*testing.T, Frontmatter, string)
	}{
		{
			name: "Missing description",
			content: `---
argument-hint: "[arg]"
allowed-tools:
  - View
---
# Content
`,
			check: func(t *testing.T, fm Frontmatter, remaining string) {
				assert.Equal(t, "", fm.Description)
				assert.Equal(t, "[arg]", fm.ArgumentHint)
				assert.Equal(t, []string{"View"}, fm.AllowedTools)
			},
		},
		{
			name: "Missing argument-hint",
			content: `---
description: Test command
allowed-tools:
  - Edit
---
# Content
`,
			check: func(t *testing.T, fm Frontmatter, remaining string) {
				assert.Equal(t, "Test command", fm.Description)
				assert.Equal(t, "", fm.ArgumentHint)
				assert.Equal(t, []string{"Edit"}, fm.AllowedTools)
			},
		},
		{
			name: "Missing allowed-tools",
			content: `---
description: Test command
argument-hint: "[arg1] [arg2]"
---
# Content
`,
			check: func(t *testing.T, fm Frontmatter, remaining string) {
				assert.Equal(t, "Test command", fm.Description)
				assert.Equal(t, "[arg1] [arg2]", fm.ArgumentHint)
				assert.Nil(t, fm.AllowedTools)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, remaining, err := ParseFrontmatter(tt.content)
			require.NoError(t, err)
			tt.check(t, fm, remaining)
		})
	}
}

func TestParseFrontmatter_EmptyValues(t *testing.T) {
	content := `---
description: ""
argument-hint: ""
allowed-tools: []
---
# Content
`

	fm, remaining, err := ParseFrontmatter(content)

	require.NoError(t, err)
	assert.Equal(t, "", fm.Description)
	assert.Equal(t, "", fm.ArgumentHint)
	assert.Empty(t, fm.AllowedTools)
	assert.Contains(t, remaining, "# Content")
}

func TestParseFrontmatter_CommaSeparatedAllowedTools(t *testing.T) {
	// Comma-separated string as single element in array (YAML parser accepts this)
	content := `---
description: Test
allowed-tools:
  - "View, Edit, Grep"
---
# Content
`

	fm, remaining, err := ParseFrontmatter(content)

	require.NoError(t, err)
	// After parsing, the comma-separated string should be split
	assert.Equal(t, []string{"View", "Edit", "Grep"}, fm.AllowedTools)
	assert.Contains(t, remaining, "# Content")
}

func TestParseFrontmatter_NoClosingDelimiter(t *testing.T) {
	content := `---
description: Test
argument-hint: "[arg]"
# Missing closing delimiter
# Content here
`

	fm, remaining, err := ParseFrontmatter(content)

	require.NoError(t, err)
	// Should treat as no frontmatter
	assert.Equal(t, "", fm.Description)
	assert.Equal(t, content, remaining)
}

func TestParseFrontmatter_OnlyOpeningDelimiter(t *testing.T) {
	content := `---
# Content without closing delimiter
`

	fm, remaining, err := ParseFrontmatter(content)

	require.NoError(t, err)
	// Should treat as no frontmatter
	assert.Equal(t, "", fm.Description)
	assert.Equal(t, content, remaining)
}

func TestParseFrontmatter_EmptyYAMLContent(t *testing.T) {
	content := `---
---
# Content
`

	fm, remaining, err := ParseFrontmatter(content)

	require.NoError(t, err)
	// Should treat as no frontmatter when YAML is empty
	assert.Equal(t, "", fm.Description)
	assert.Contains(t, remaining, "# Content")
	assert.NotEmpty(t, remaining)
}

func TestParseFrontmatter_BOMCharacter(t *testing.T) {
	content := "\ufeff---\ndescription: Test\n---\n# Content"

	fm, remaining, err := ParseFrontmatter(content)

	require.NoError(t, err)
	assert.Equal(t, "Test", fm.Description)
	assert.Contains(t, remaining, "# Content")
}

func TestParseFrontmatter_ClosingDelimiterAtEnd(t *testing.T) {
	content := `---
description: Test
---
`

	fm, remaining, err := ParseFrontmatter(content)

	require.NoError(t, err)
	assert.Equal(t, "Test", fm.Description)
	assert.Empty(t, remaining)
}

func TestParseFrontmatter_ComplexContent(t *testing.T) {
	// Quote special characters in YAML to avoid parsing issues
	content := `---
description: "Multi-line description with special chars: <>&"
argument-hint: "[file] [options...]"
allowed-tools:
  - bash
  - edit
  - view
---
# Command Title

This is the command content.

It can have multiple paragraphs.

- List item 1
- List item 2
`

	fm, remaining, err := ParseFrontmatter(content)

	require.NoError(t, err)
	assert.Equal(t, "Multi-line description with special chars: <>&", fm.Description)
	assert.Equal(t, "[file] [options...]", fm.ArgumentHint)
	assert.Equal(t, []string{"bash", "edit", "view"}, fm.AllowedTools)
	assert.Contains(t, remaining, "# Command Title")
	assert.Contains(t, remaining, "This is the command content")
	assert.Contains(t, remaining, "List item 1")
}

