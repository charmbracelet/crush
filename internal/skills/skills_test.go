package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		content     string
		wantName    string
		wantDesc    string
		wantLicense string
		wantCompat  string
		wantMeta    map[string]string
		wantTools   string
		wantInstr   string
		wantErr     bool
	}{
		{
			name: "full skill",
			content: `---
name: pdf-processing
description: Extracts text and tables from PDF files, fills PDF forms, and merges multiple PDFs.
license: Apache-2.0
compatibility: Requires python 3.8+, pdfplumber, pdfrw libraries
metadata:
  author: example-org
  version: "1.0"
---

# PDF Processing

## When to use this skill
Use this skill when the user needs to work with PDF files.
`,
			wantName:    "pdf-processing",
			wantDesc:    "Extracts text and tables from PDF files, fills PDF forms, and merges multiple PDFs.",
			wantLicense: "Apache-2.0",
			wantCompat:  "Requires python 3.8+, pdfplumber, pdfrw libraries",
			wantMeta:    map[string]string{"author": "example-org", "version": "1.0"},
			wantInstr:   "# PDF Processing\n\n## When to use this skill\nUse this skill when the user needs to work with PDF files.",
		},
		{
			name: "minimal skill",
			content: `---
name: my-skill
description: A simple skill for testing.
---

# My Skill

Instructions here.
`,
			wantName:  "my-skill",
			wantDesc:  "A simple skill for testing.",
			wantInstr: "# My Skill\n\nInstructions here.",
		},
		{
			name: "frontmatter with utf8 bom",
			content: "\uFEFF---\n" +
				"name: bom-skill\n" +
				"description: Skill with bom.\n" +
				"---\n\n" +
				"# BOM Skill\n",
			wantName:  "bom-skill",
			wantDesc:  "Skill with bom.",
			wantInstr: "# BOM Skill",
		},
		{
			name: "frontmatter with leading blank lines",
			content: "\n\n---\n" +
				"name: blank-prefix\n" +
				"description: Skill with leading blank lines.\n" +
				"---\n\n" +
				"# Blank Prefix\n",
			wantName:  "blank-prefix",
			wantDesc:  "Skill with leading blank lines.",
			wantInstr: "# Blank Prefix",
		},
		{
			name: "frontmatter delimiter with trailing spaces",
			content: "---   \n" +
				"name: spaced-delimiter\n" +
				"description: Delimiter has spaces.\n" +
				"---   \n\n" +
				"# Spaced Delimiter\n",
			wantName:  "spaced-delimiter",
			wantDesc:  "Delimiter has spaces.",
			wantInstr: "# Spaced Delimiter",
		},
		{
			name:    "no frontmatter",
			content: "# Just Markdown\n\nNo frontmatter here.",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Write content to temp file.
			dir := t.TempDir()
			path := filepath.Join(dir, "SKILL.md")
			require.NoError(t, os.WriteFile(path, []byte(tt.content), 0o644))

			skill, err := Parse(path)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			require.Equal(t, tt.wantName, skill.Name)
			require.Equal(t, tt.wantDesc, skill.Description)
			require.Equal(t, tt.wantLicense, skill.License)
			require.Equal(t, tt.wantCompat, skill.Compatibility)

			if tt.wantMeta != nil {
				require.Equal(t, tt.wantMeta, skill.Metadata)
			}

			require.Equal(t, tt.wantInstr, skill.Instructions)
		})
	}
}

func TestSkillValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		skill   Skill
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid skill",
			skill: Skill{
				Name:        "pdf-processing",
				Description: "Processes PDF files.",
				Path:        "/skills/pdf-processing",
			},
		},
		{
			name:    "missing name",
			skill:   Skill{Description: "Some description."},
			wantErr: true,
			errMsg:  "name is required",
		},
		{
			name:    "missing description",
			skill:   Skill{Name: "my-skill", Path: "/skills/my-skill"},
			wantErr: true,
			errMsg:  "description is required",
		},
		{
			name:    "name too long",
			skill:   Skill{Name: strings.Repeat("a", 65), Description: "Some description."},
			wantErr: true,
			errMsg:  "exceeds",
		},
		{
			name:    "valid name - mixed case",
			skill:   Skill{Name: "MySkill", Description: "Some description.", Path: "/skills/MySkill"},
			wantErr: false,
		},
		{
			name:    "invalid name - starts with hyphen",
			skill:   Skill{Name: "-my-skill", Description: "Some description."},
			wantErr: true,
			errMsg:  "alphanumeric with hyphens",
		},
		{
			name:    "name doesn't match directory",
			skill:   Skill{Name: "my-skill", Description: "Some description.", Path: "/skills/other-skill"},
			wantErr: true,
			errMsg:  "must match directory",
		},
		{
			name:    "description too long",
			skill:   Skill{Name: "my-skill", Description: strings.Repeat("a", 1025), Path: "/skills/my-skill"},
			wantErr: true,
			errMsg:  "description exceeds",
		},
		{
			name:    "compatibility too long",
			skill:   Skill{Name: "my-skill", Description: "desc", Compatibility: strings.Repeat("a", 501), Path: "/skills/my-skill"},
			wantErr: true,
			errMsg:  "compatibility exceeds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.skill.Validate()
			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDiscover(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create valid skill 1.
	skill1Dir := filepath.Join(tmpDir, "skill-one")
	require.NoError(t, os.MkdirAll(skill1Dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skill1Dir, "SKILL.md"), []byte(`---
name: skill-one
description: First test skill.
---
# Skill One
`), 0o644))

	// Create valid skill 2 in nested directory.
	skill2Dir := filepath.Join(tmpDir, "nested", "skill-two")
	require.NoError(t, os.MkdirAll(skill2Dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skill2Dir, "SKILL.md"), []byte(`---
name: skill-two
description: Second test skill.
---
# Skill Two
`), 0o644))

	// Create invalid skill (won't be included).
	invalidDir := filepath.Join(tmpDir, "invalid-dir")
	require.NoError(t, os.MkdirAll(invalidDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(invalidDir, "SKILL.md"), []byte(`---
name: wrong-name
description: Name doesn't match directory.
---
`), 0o644))

	skills := Discover([]string{tmpDir})
	require.Len(t, skills, 2)
	require.Equal(t, []string{"skill-two", "skill-one"}, []string{skills[0].Name, skills[1].Name})

	names := make(map[string]bool)
	for _, s := range skills {
		names[s.Name] = true
	}
	require.True(t, names["skill-one"])
	require.True(t, names["skill-two"])
}

func TestToPromptXML(t *testing.T) {
	t.Parallel()

	skills := []*Skill{
		{Name: "pdf-processing", Description: "Extracts text from PDFs.", SkillFilePath: "/skills/pdf-processing/SKILL.md"},
		{Name: "data-analysis", Description: "Analyzes datasets & charts.", SkillFilePath: "/skills/data-analysis/SKILL.md"},
	}

	xml := ToPromptXML(skills)

	require.Contains(t, xml, "<available_skills>")
	require.Contains(t, xml, "<name>pdf-processing</name>")
	require.Contains(t, xml, "<description>Extracts text from PDFs.</description>")
	require.Contains(t, xml, "&amp;") // XML escaping
}

func TestToPromptXMLEmpty(t *testing.T) {
	t.Parallel()
	require.Empty(t, ToPromptXML(nil))
	require.Empty(t, ToPromptXML([]*Skill{}))
}

func TestEscape(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "escape xml special chars",
			in:   `<tag attr="x&y">'z'</tag>`,
			want: `&lt;tag attr=&quot;x&amp;y&quot;&gt;&apos;z&apos;&lt;/tag&gt;`,
		},
		{
			name: "plain text unchanged",
			in:   "hello world",
			want: "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, escape(tt.in))
		})
	}
}

func TestToPromptXMLBuiltinType(t *testing.T) {
	t.Parallel()

	skills := []*Skill{
		{Name: "builtin-skill", Description: "A builtin.", SkillFilePath: "crush://skills/builtin-skill/SKILL.md", Builtin: true},
		{Name: "user-skill", Description: "A user skill.", SkillFilePath: "/home/user/.config/crush/skills/user-skill/SKILL.md"},
	}
	xml := ToPromptXML(skills)
	require.Contains(t, xml, "<type>builtin</type>")
	require.Equal(t, 1, strings.Count(xml, "<type>builtin</type>"))
}

func TestParseContent(t *testing.T) {
	t.Parallel()

	content := []byte(`---
name: my-skill
description: A test skill.
---

# My Skill

Instructions here.
`)
	skill, err := ParseContent(content)
	require.NoError(t, err)
	require.Equal(t, "my-skill", skill.Name)
	require.Equal(t, "A test skill.", skill.Description)
	require.Equal(t, "# My Skill\n\nInstructions here.", skill.Instructions)
	require.Empty(t, skill.Path)
	require.Empty(t, skill.SkillFilePath)
}

func TestParseContent_NoFrontmatter(t *testing.T) {
	t.Parallel()

	_, err := ParseContent([]byte("# Just Markdown"))
	require.Error(t, err)
}

func TestDiscoverBuiltin(t *testing.T) {
	t.Parallel()

	discovered := DiscoverBuiltin()
	require.NotEmpty(t, discovered)

	var found bool
	for _, s := range discovered {
		if s.Name == "crush-config" {
			found = true
			require.True(t, strings.HasPrefix(s.SkillFilePath, BuiltinPrefix))
			require.True(t, strings.HasPrefix(s.Path, BuiltinPrefix))
			require.Equal(t, "crush://skills/crush-config/SKILL.md", s.SkillFilePath)
			require.Equal(t, "crush://skills/crush-config", s.Path)
			require.NotEmpty(t, s.Description)
			require.NotEmpty(t, s.Instructions)
			require.True(t, s.Builtin)
		}
	}
	require.True(t, found, "crush-config builtin skill not found")
}

func TestDeduplicate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    []*Skill
		wantLen  int
		wantName string
		wantPath string
	}{
		{
			name:    "no duplicates",
			input:   []*Skill{{Name: "a", Path: "/a"}, {Name: "b", Path: "/b"}},
			wantLen: 2,
		},
		{
			name:     "user overrides builtin",
			input:    []*Skill{{Name: "crush-config", Path: "crush://skills/crush-config"}, {Name: "crush-config", Path: "/user/crush-config"}},
			wantLen:  1,
			wantName: "crush-config",
			wantPath: "/user/crush-config",
		},
		{
			name:    "empty",
			input:   nil,
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := Deduplicate(tt.input)
			require.Len(t, result, tt.wantLen)
			if tt.wantName != "" {
				require.Equal(t, tt.wantName, result[0].Name)
				require.Equal(t, tt.wantPath, result[0].Path)
			}
		})
	}
}

func TestFilter(t *testing.T) {
	t.Parallel()

	all := []*Skill{
		{Name: "a"},
		{Name: "b"},
		{Name: "c"},
	}

	tests := []struct {
		name     string
		disabled []string
		wantLen  int
	}{
		{"no filter", nil, 3},
		{"filter one", []string{"b"}, 2},
		{"filter all", []string{"a", "b", "c"}, 0},
		{"filter nonexistent", []string{"d"}, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := Filter(all, tt.disabled)
			require.Len(t, result, tt.wantLen)
		})
	}
}
