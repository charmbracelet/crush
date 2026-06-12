package compact

import (
	"cmp"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/x/ansi"
)

// Per-item byte limits; Compact drops whole items if the total overflows.
const (
	goalBytes       = 2000
	latestBytes     = 1000
	pathBytes       = 200
	commandBytes    = 120
	failureBytes    = 300
	transcriptBytes = 600
)

var (
	// ackRe matches replies that steer but do not state a request.
	ackRe = regexp.MustCompile(`(?i)^(ok(ay)?|y(es)?|yep|yeah|sure|no|go( on| ahead)?|continue|proceed|do it|thanks|thank you|thx|ty|lgtm|k)[.!]*$`)
	// cwdRe matches the working directory trailer of bash results.
	cwdRe = regexp.MustCompile(`\s*<cwd>[^<]*</cwd>\s*$`)
	// exitRe matches the line bash appends when a command fails.
	exitRe = regexp.MustCompile(`^(Exit code [1-9]\d*|Command was aborted before completion)$`)
	// errorLineRe picks the most telling line of a failure.
	errorLineRe = regexp.MustCompile(`(?i)\b(error|fail(ed|ure)?|panic|fatal|cannot|undefined|not found|no such|denied|invalid|unexpected)\b`)
)

func extract(msgs []message.Message) brief {
	var (
		b                                brief
		prompts                          []string
		modified, read, failed, searches recent
		succeeded                        = make(map[string]int)
		tick                             int
	)
	// Each fact gets a fresh tick so ordering never ties.
	next := func() int {
		tick++
		return tick
	}
	byCall := toolResults(msgs)
	for _, m := range msgs {
		if m.IsSummaryMessage {
			continue
		}
		text := visibleText(m.Parts)
		switch m.Role {
		case message.User:
			// Bang-mode shell commands are not prompts.
			if text != "" && m.IsPrompt() {
				prompts = append(prompts, text)
				b[transcriptSection] = append(b[transcriptSection], "User: "+clip(text, transcriptBytes))
			}
			continue
		case message.Assistant:
			if text != "" {
				b[transcriptSection] = append(b[transcriptSection], "Assistant: "+clip(text, transcriptBytes))
			}
		default:
			continue
		}
		for _, c := range m.ToolCalls() {
			use := newToolUse(c, byCall[c.ID])
			key := use.name + " " + use.subject()
			if use.failed() {
				failed.add(key, next(), failureText(use))
				continue
			}
			if use.result == "" {
				// No result, e.g. the call was canceled.
				continue
			}
			succeeded[key] = next()
			path := use.input.FilePath
			switch use.name {
			case tools.EditToolName, tools.MultiEditToolName, tools.WriteToolName, tools.ReplaceSymbolToolName:
				if path != "" {
					modified.add(path, next(), clip(path, pathBytes))
				}
			case tools.ViewToolName:
				if path != "" {
					read.add(path, next(), clip(path, pathBytes))
				}
			case tools.GrepToolName, tools.GlobToolName:
				if strings.HasPrefix(strings.TrimSpace(use.result), "No files found") {
					searches.add(key, next(), searchText(use))
				}
			}
		}
	}

	if goal, latest := goals(prompts); goal != "" {
		b[goalSection] = []string{clip(goal, goalBytes)}
		if latest != "" {
			b[latestSection] = []string{clip(latest, latestBytes)}
		}
	}
	b[modifiedSection] = texts(modified.list())
	b[searchSection] = texts(searches.list())
	for _, it := range read.list() {
		if _, ok := modified.items[it.key]; !ok {
			b[readSection] = append(b[readSection], it.text)
		}
	}
	for _, it := range failed.list() {
		if succeeded[it.key] > it.seq {
			it.text += " (later succeeded)"
		}
		b[failedSection] = append(b[failedSection], it.text)
	}
	return b
}

// goals returns the first real prompt and, when it is a different one, the
// latest real prompt. Bare acknowledgements only count if nothing else does.
func goals(prompts []string) (goal, latest string) {
	first, last := -1, -1
	for i, p := range prompts {
		if ackRe.MatchString(p) {
			continue
		}
		if first < 0 {
			first = i
		}
		last = i
	}
	switch {
	case first < 0 && len(prompts) > 0:
		return prompts[0], ""
	case first < 0:
		return "", ""
	case last > first:
		return prompts[first], prompts[last]
	}
	return prompts[first], ""
}

// subject is what the tool acted on: a command, file, or pattern.
func (u *toolUse) subject() string {
	switch {
	case u.input.Command != "":
		return oneLine(u.input.Command)
	case u.input.FilePath != "":
		return u.input.FilePath
	case u.input.Pattern != "":
		return u.input.Pattern
	}
	return u.input.Path
}

// failed reports a tool error, or a bash command Crush marked with a
// non-zero exit code or an abort in place of an error flag.
func (u *toolUse) failed() bool {
	if u.isError {
		return true
	}
	if u.name != tools.BashToolName {
		return false
	}
	out := strings.TrimSpace(cwdRe.ReplaceAllString(u.result, ""))
	return exitRe.MatchString(out[strings.LastIndexByte(out, '\n')+1:])
}

func failureText(u *toolUse) string {
	text := u.name
	if s := u.subject(); s != "" {
		text += " `" + clip(s, commandBytes) + "`"
	}
	if line := errorLine(u.result); line != "" {
		text += ": " + line
	}
	return clip(text, failureBytes)
}

// errorLine is the first line that reads like an error, else the first
// line with content, else the exit code line.
func errorLine(result string) string {
	var first, exit string
	for line := range strings.SplitSeq(ansi.Strip(result), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "" || cwdRe.MatchString(line):
		case exitRe.MatchString(line):
			exit = line
		case errorLineRe.MatchString(line):
			return line
		case first == "":
			first = line
		}
	}
	return cmp.Or(first, exit)
}

func searchText(u *toolUse) string {
	text := u.name + " `" + clip(u.input.Pattern, commandBytes) + "`"
	if u.input.Path != "" {
		text += " in " + clip(u.input.Path, pathBytes)
	}
	return text
}

// oneLine is the first non-empty line of s, marked when more follows.
func oneLine(s string) string {
	s = strings.TrimSpace(s)
	if first, rest, ok := strings.Cut(s, "\n"); ok && strings.TrimSpace(rest) != "" {
		return strings.TrimSpace(first) + " …"
	}
	return s
}

// recent is an insertion-ordered set; adding a key again moves it last.
type recent struct {
	items map[string]recentItem
}

type recentItem struct {
	key, text string
	seq       int
}

func (r *recent) add(key string, seq int, text string) {
	if r.items == nil {
		r.items = make(map[string]recentItem)
	}
	r.items[key] = recentItem{key: key, text: text, seq: seq}
}

// list returns the items oldest first.
func (r *recent) list() []recentItem {
	items := slices.Collect(maps.Values(r.items))
	slices.SortFunc(items, func(a, b recentItem) int {
		return cmp.Compare(a.seq, b.seq)
	})
	return items
}

func texts(items []recentItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.text
	}
	return out
}
