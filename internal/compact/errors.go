package compact

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	tscErrorRe = regexp.MustCompile(`error TS\d+:.+`)
	testFailRe = regexp.MustCompile(`(?i)(?:FAIL|✗|✘|×)\s|(\d+)\s+(?:failed|failure|failing)`)
	emptyResRe = regexp.MustCompile(`(?i)^(?:No matches? found\.?|No files? matched\.?|0 results?|No results?\.?)$`)
	blockerRe  = regexp.MustCompile(`(?i)\b(fail(ed|s|ure|ing)?|broken|cannot|can't|won't work|does not work|doesn't work|still (broken|failing|wrong)|blocked|blocker|not (fixed|resolved|working)|crash(es|ed|ing)?)\b`)
	tscFileRe  = regexp.MustCompile(`^\[tsc\]\s+(\S+)\(\d+,\d+\)`)
)

const (
	bashOutputScanLimit = 8000
	maxOutstandingItems = 8
	outstandingTailSize = 25
)

// ExtractOutstandingContext extracts error signals from the tail of the
// conversation. It detects bash failures, compiler errors, test failures,
// empty search results, tool errors, and user-reported blockers. Resolved
// errors (where the file was subsequently edited) are tagged [RESOLVED].
func ExtractOutstandingContext(blocks []NormalizedBlock) []string {
	tail := blocks
	if len(tail) > outstandingTailSize {
		tail = tail[len(tail)-outstandingTailSize:]
	}

	type item struct {
		text    string
		tailIdx int
	}
	var items []item
	seen := make(map[string]bool)

	push := func(text string, tailIdx int) {
		if !seen[text] {
			seen[text] = true
			items = append(items, item{text: text, tailIdx: tailIdx})
		}
	}

	for bi, b := range tail {
		switch b.Kind {
		case BlockBash:
			if b.ExitCode > 0 {
				cmd := firstNonEmptyLine(b.Command)
				if cmd == "" {
					cmd = firstNonEmptyLine(b.Output)
				}
				cmd = Clip(cmd, 80)
				outLine := FirstLine(b.Output, 120)
				entry := "[bash:exit " + strconv.Itoa(b.ExitCode) + "] " + cmd
				if outLine != "" && outLine != cmd {
					entry += " → " + outLine
				}
				push(entry, bi)
				continue
			}
			outputHead := b.Output
			if len(outputHead) > bashOutputScanLimit {
				outputHead = outputHead[:bashOutputScanLimit]
			}
			if tscErrorRe.MatchString(outputHead) {
				for line := range strings.SplitSeq(outputHead, "\n") {
					if tscErrorRe.MatchString(strings.TrimSpace(line)) {
						push("[tsc] "+Clip(strings.TrimSpace(line), 150), bi)
					}
				}
				continue
			}
			if testFailRe.MatchString(outputHead) {
				push("[tests] "+FirstLine(b.Output, 150), bi)
				continue
			}

		case BlockToolResult:
			if (b.Name == "grep" || b.Name == "glob") && emptyResRe.MatchString(strings.TrimSpace(b.ResultText)) {
				push("[no matches] "+b.Name, bi)
				continue
			}
			if b.IsError {
				if tscErrorRe.MatchString(b.ResultText) {
					for line := range strings.SplitSeq(b.ResultText, "\n") {
						if tscErrorRe.MatchString(strings.TrimSpace(line)) {
							push("[tsc] "+Clip(strings.TrimSpace(line), 150), bi)
						}
					}
					continue
				}
				if testFailRe.MatchString(b.ResultText) {
					push("[tests] "+FirstLine(b.ResultText, 150), bi)
					continue
				}
				push("["+b.Name+"] "+FirstLine(b.ResultText, 150), bi)
				continue
			}

		case BlockAssistant, BlockUser:
			for _, line := range NonEmptyLines(b.Text) {
				if !blockerRe.MatchString(line) || len(line) < 15 {
					continue
				}
				clipped := ClipSentence(line, 150)
				if b.Kind == BlockUser {
					clipped = "[user] " + clipped
				}
				push(clipped, bi)
				break
			}
		}
	}

	editPositions := buildEditPositions(tail)

	result := make([]string, 0, min(len(items), maxOutstandingItems))
	for i, it := range items {
		if i >= maxOutstandingItems {
			break
		}
		tagged := priorityTag(it.text)
		file := extractTscFile(it.text)
		if file != "" && it.tailIdx >= 0 && isTscResolved(file, it.tailIdx, editPositions) {
			tagged = strings.Replace(tagged, "[ERROR]", "[RESOLVED]", 1)
			tagged = strings.Replace(tagged, "[WARN]", "[RESOLVED]", 1)
		}
		result = append(result, tagged)
	}
	return result
}

func priorityTag(item string) string {
	if strings.HasPrefix(item, "[tsc]") {
		return "[ERROR] " + item
	}
	if strings.HasPrefix(item, "[bash:exit ") {
		return "[ERROR] " + item
	}
	if strings.HasPrefix(item, "[tests]") {
		return "[WARN] " + item
	}
	if strings.HasPrefix(item, "[no matches]") {
		return "[INFO] " + item
	}
	if strings.HasPrefix(item, "[user]") {
		return "[WARN] " + item
	}
	return "[WARN] " + item
}

func extractTscFile(item string) string {
	m := tscFileRe.FindStringSubmatch(item)
	if m != nil {
		return m[1]
	}
	return ""
}

func buildEditPositions(tail []NormalizedBlock) map[int]map[string]bool {
	positions := make(map[int]map[string]bool)
	for i, b := range tail {
		if b.Kind == BlockToolCall && fileWriteTools[b.Name] {
			path := extractPathFromArgs(b.Args)
			if path != "" {
				if positions[i] == nil {
					positions[i] = make(map[string]bool)
				}
				positions[i][path] = true
			}
		}
	}
	return positions
}

func isTscResolved(file string, tailIdx int, editPositions map[int]map[string]bool) bool {
	for pos, files := range editPositions {
		if pos > tailIdx && files[file] {
			return true
		}
	}
	return false
}

func firstNonEmptyLine(text string) string {
	for line := range strings.SplitSeq(text, "\n") {
		t := strings.TrimSpace(line)
		if t != "" {
			return t
		}
	}
	return ""
}
