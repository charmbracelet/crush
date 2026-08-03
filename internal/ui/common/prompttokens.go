package common

import (
	"strings"
	"sync"
)

// PromptTokenKind distinguishes the two things the prompt marks up.
type PromptTokenKind int

const (
	// PromptTokenFile is an "@path" file mention.
	PromptTokenFile PromptTokenKind = iota
	// PromptTokenSkill is a "/name" skill reference.
	PromptTokenSkill
)

// PromptToken is a highlighted span within a single line, expressed in rune
// offsets: Start is inclusive, End exclusive.
type PromptToken struct {
	Start int
	End   int
	Kind  PromptTokenKind
}

// promptSkills is the set of skill names a "/token" may refer to.
//
// It lives here, rather than being threaded through every message-item
// constructor, because both the prompt editor and the posted-message
// renderer must agree on it exactly — a token that lights up while you type
// has to stay lit after you hit enter. The UI event loop is the only
// writer; renderers read it. This mirrors the package-level discovery-state
// mirror in internal/skills.
var promptSkills struct {
	mu    sync.RWMutex
	names []string
}

// SetPromptSkillNames replaces the known-skill set used to validate "/"
// tokens. Called whenever the effective skill list changes.
func SetPromptSkillNames(names []string) {
	promptSkills.mu.Lock()
	promptSkills.names = names
	promptSkills.mu.Unlock()
}

// IsPromptSkillPrefix reports whether name is a prefix of some known skill
// name. Prefix rather than exact match is deliberate: it is what gives live
// feedback as a skill name is typed, and using the same rule after the
// prompt is submitted keeps the highlight from flickering off mid-word.
func IsPromptSkillPrefix(name string) bool {
	if name == "" {
		return false
	}
	promptSkills.mu.RLock()
	defer promptSkills.mu.RUnlock()
	for _, s := range promptSkills.names {
		if strings.HasPrefix(s, name) {
			return true
		}
	}
	return false
}

// ScanPromptTokens finds the @file and /skill tokens in a single line.
//
// Tokens start at a word boundary (start of line or after whitespace) and
// run to the next whitespace, so they never span lines. An "@" token always
// counts — the file may not exist yet, and the completions popup is the
// discovery path — while a "/" token counts only when it matches a known
// skill, so ordinary prose and absolute paths are left alone.
func ScanPromptTokens(line []rune) []PromptToken {
	var tokens []PromptToken
	i := 0
	for i < len(line) {
		atWordStart := i == 0 || isSpaceRune(line[i-1])
		if atWordStart && (line[i] == '@' || line[i] == '/') && i+1 < len(line) && !isSpaceRune(line[i+1]) {
			end := i + 1
			for end < len(line) && !isSpaceRune(line[end]) {
				end++
			}
			if line[i] == '@' {
				tokens = append(tokens, PromptToken{Start: i, End: end, Kind: PromptTokenFile})
			} else if IsPromptSkillPrefix(string(line[i+1 : end])) {
				tokens = append(tokens, PromptToken{Start: i, End: end, Kind: PromptTokenSkill})
			}
			i = end
			continue
		}
		i++
	}
	return tokens
}

func isSpaceRune(r rune) bool {
	return r == ' ' || r == '\t'
}
