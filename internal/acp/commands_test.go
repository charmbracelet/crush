package acp

import (
	"io"
	"log/slog"
	"testing"

	"github.com/charmbracelet/crush/internal/commands"
	"github.com/charmbracelet/crush/internal/skills"
	"github.com/stretchr/testify/require"
)

func newResolverAgent() *crushAgent {
	return &crushAgent{log: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func TestSplitCommand(t *testing.T) {
	tests := map[string]struct {
		prompt string
		name   string
		rest   string
		ok     bool
	}{
		"bare slash":        {prompt: "/", ok: false},
		"plain prose":       {prompt: "hello", ok: false},
		"name only":         {prompt: "/review", name: "review", ok: true},
		"with args":         {prompt: "/review src/main.go now", name: "review", rest: "src/main.go now", ok: true},
		"leading space":     {prompt: "/review   padded", name: "review", rest: "padded", ok: true},
		"newline arg":       {prompt: "/review\ntext", name: "review", rest: "text", ok: true},
		"namespaced":        {prompt: "/user:foo bar", name: "user:foo", rest: "bar", ok: true},
		"slash in the name": {prompt: "/project:sub/cmd", name: "project:sub/cmd", ok: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			gotName, gotRest, gotOK := splitCommand(tc.prompt)
			require.Equal(t, tc.ok, gotOK)
			require.Equal(t, tc.name, gotName)
			require.Equal(t, tc.rest, gotRest)
		})
	}
}

func TestResolveCommandCustomCommandSubstitutesArgs(t *testing.T) {
	matches := map[string]commandMatch{
		"review": {
			name:    "review",
			content: "Review the changes in $PATH and report $FORMAT",
			args: []commands.Argument{
				{ID: "PATH", Required: true},
				{ID: "FORMAT", Required: true},
			},
		},
	}
	a := newResolverAgent()
	got := a.resolveAgainst(matches, "/review src/ md")
	require.Equal(t, "Review the changes in src/ and report md", got)
}

func TestResolveCommandSkillKeepsTrailingText(t *testing.T) {
	matches := map[string]commandMatch{
		"commit": {
			name:  "commit",
			skill: &skills.Skill{Name: "commit", Description: "Commit work", Instructions: "Do the commit."},
		},
	}
	a := newResolverAgent()
	got := a.resolveAgainst(matches, "/commit please squash")
	require.Contains(t, got, "<loaded_skill>")
	require.Contains(t, got, "Do the commit.")
	require.Contains(t, got, "please squash")
}

func TestResolveCommandUnknownFallsThrough(t *testing.T) {
	a := newResolverAgent()
	matches := map[string]commandMatch{
		"review": {name: "review", content: "template"},
	}
	// An unadvertised name is ordinary prose that happens to start with a
	// slash, so it must reach the model unchanged.
	require.Equal(t, "/nope do something", a.resolveAgainst(matches, "/nope do something"))
	require.Equal(t, "/", a.resolveAgainst(matches, "/"))
	require.Equal(t, "no slash here", a.resolveAgainst(matches, "no slash here"))
	// No commands at all is the same case.
	require.Equal(t, "/review", a.resolveAgainst(nil, "/review"))
}

func TestResolveCommandMissingRequiredArgsFallsThrough(t *testing.T) {
	matches := map[string]commandMatch{
		"review": {
			name:    "review",
			content: "Review $PATH",
			args:    []commands.Argument{{ID: "PATH", Required: true}},
		},
	}
	a := newResolverAgent()
	// Sending "Review $PATH" would be worse than passing the text through.
	require.Equal(t, "/review", a.resolveAgainst(matches, "/review"))
}

func TestResolveCommandOptionalArgsMayBeOmitted(t *testing.T) {
	matches := map[string]commandMatch{
		"review": {
			name:    "review",
			content: "Review $PATH",
			args:    []commands.Argument{{ID: "PATH", Required: false}},
		},
	}
	a := newResolverAgent()
	got := a.resolveAgainst(matches, "/review")
	require.Equal(t, "Review $PATH", got)
}

func TestLookupCommandPrefersLongestMatch(t *testing.T) {
	matches := map[string]commandMatch{
		"user:foo":     {name: "user:foo", content: "short"},
		"user:foo/bar": {name: "user:foo/bar", content: "long"},
	}
	m, ok := lookupCommand(matches, "user:foo/bar")
	require.True(t, ok)
	require.Equal(t, "long", m.content)

	m, ok = lookupCommand(matches, "user:foo")
	require.True(t, ok)
	require.Equal(t, "short", m.content)

	_, ok = lookupCommand(matches, "other")
	require.False(t, ok)
}

func TestBindArgsBindsPositionally(t *testing.T) {
	declared := []commands.Argument{{ID: "A", Required: true}, {ID: "B", Required: false}}
	require.Equal(t, map[string]string{"A": "one", "B": "two"}, bindArgs(declared, "one two three"))
	require.Equal(t, map[string]string{"A": "one"}, bindArgs(declared, "one"))
	require.Empty(t, bindArgs(declared, ""))
	require.Nil(t, bindArgs(nil, "ignored"))
}

func TestCountRequired(t *testing.T) {
	require.Equal(t, 2, countRequired([]commands.Argument{
		{ID: "A", Required: true},
		{ID: "B", Required: false},
		{ID: "C", Required: true},
	}))
	require.Zero(t, countRequired(nil))
}
