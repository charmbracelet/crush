package chat

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"regexp"
	"slices"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/joestump-agent/a2tea"
	"github.com/joestump-agent/a2tea/event"
	"github.com/joestump-agent/a2tea/render"
	a2ui "github.com/tmc/a2ui"
	"github.com/tmc/a2ui/a2uistream"
)

// a2uiOpenTag and a2uiCloseTag delimit an inline A2UI block in assistant
// content — the wire format a2tea scans for.
const (
	a2uiOpenTag  = "<a2ui-json>"
	a2uiCloseTag = "</a2ui-json>"
)

var (
	fenceRe = regexp.MustCompile("(?s)```[^\n]*\n.*?```")
	// inlineCodeRe matches markdown inline code spans on a single line:
	// double-backtick spans first (their content may hold single backticks),
	// then single-backtick spans.
	inlineCodeRe = regexp.MustCompile("``[^\n]*?``|`[^`\n]+`")
)

// codeReplacement tracks a masked code span for later restoration.
type codeReplacement struct {
	placeholder string
	original    string
}

// maskMarkdownCode replaces markdown code with unique placeholders so a2tea's
// markdown-unaware scanner does not extract A2UI from code that is
// documentation, not live UI. Fenced code blocks are always masked (#6);
// inline code spans are masked only when they mention A2UI markers (#96) —
// masking every span would leak placeholders into legitimate surface JSON
// that happens to contain backticks. Use unmaskMarkdownCode to restore the
// originals after scanning.
func maskMarkdownCode(content string) (string, []codeReplacement) {
	var reps []codeReplacement
	mask := func(match string) string {
		p := fmt.Sprintf("\x00CODE%d\x00", len(reps))
		reps = append(reps, codeReplacement{p, match})
		return p
	}
	if strings.Contains(content, "```") {
		content = fenceRe.ReplaceAllStringFunc(content, mask)
	}
	if strings.Contains(content, "`") {
		content = inlineCodeRe.ReplaceAllStringFunc(content, func(match string) string {
			if !mentionsA2UI(match) {
				return match
			}
			return mask(match)
		})
	}
	return content, reps
}

// a2uiMarkers are the literals a2tea's scanner reacts to: the wire tag and
// the quoted A2UI message-type keys used for bare-JSON detection. Inline code
// naming any of these is documentation the scanner must not consume (#96).
var a2uiMarkers = []string{
	"a2ui-json",
	`"createSurface"`,
	`"updateComponents"`,
	`"updateDataModel"`,
	`"deleteSurface"`,
}

// mentionsA2UI reports whether s contains any marker the a2tea scanner
// could mistake for live A2UI content.
func mentionsA2UI(s string) bool {
	for _, m := range a2uiMarkers {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}

// unmaskMarkdownCode restores code spans replaced by maskMarkdownCode.
func unmaskMarkdownCode(text string, reps []codeReplacement) string {
	for _, r := range reps {
		text = strings.ReplaceAll(text, r.placeholder, r.original)
	}
	return text
}

// contentHasA2UI reports whether the assistant content carries any A2UI
// outside markdown code (fences and inline spans), so the renderer only takes
// the a2tea path when there is live UI to draw.
func contentHasA2UI(content string) bool {
	masked, _ := maskMarkdownCode(content)
	return a2tea.Contains(masked)
}

// hasUnclosedA2UITag reports whether content has more opening
// <a2ui-json> tags than closing </a2ui-json> tags — a truncated block
// (#5). This catches both a single unclosed block and the rarer case of
// a complete surface followed by a second truncated block. Only
// meaningful for finished messages; while streaming the close tag
// simply hasn't arrived yet.
func hasUnclosedA2UITag(content string) bool {
	return strings.Count(content, "<a2ui-json>") > strings.Count(content, "</a2ui-json>")
}

// contentHasUnclosedA2UI is the gate-level check (markdown code masked) for
// truncated A2UI blocks in finished messages.
func contentHasUnclosedA2UI(content string) bool {
	masked, _ := maskMarkdownCode(content)
	return hasUnclosedA2UITag(masked)
}

// a2uiBlockStats summarizes how the a2uistream parser treats the complete
// <a2ui-json> blocks in content (#7, #168).
type a2uiBlockStats struct {
	// dropped counts blocks the parser consumed but produced nothing from —
	// malformed JSON or objects it doesn't recognize. These lose content and
	// must alert.
	dropped int
	// protocolOnly counts blocks holding only protocol messages
	// (callFunction/actionResponse): recognized, just not drawable. These get
	// a quiet notice, not the malformed-content alert.
	protocolOnly int
	// unclosed reports an open tag the parser honors as a delimiter that
	// never got its closing partner — a truncated generation (#5).
	unclosed bool
}

// scanA2UIBlocks classifies the tagged blocks in content the way the
// a2uistream parser does. Both the segmentation and the per-block judgment
// derive from the parser's own rules, so this check agrees with rendering by
// construction.
//
// Segmentation used to pair tag literals with a blind string scan, which
// disagreed with the parser when a bare A2UI message's *string content*
// mentioned the tags — the parser consumes the whole object (tag literals and
// all) as one message, while the blind scan saw a "block" between the
// mentions and raised a false alert beside a working surface (#168). This
// mirrors the parser's pre-tag scan instead: a bare A2UI object opened before
// a tag swallows it, and only a tag the parser would honor as a delimiter
// starts a block.
//
// Judging each block by re-parsing it also fixes what a single-object
// unmarshal got wrong in both directions: valid-but-unrecognized JSON
// ({"foo":1}) is silently dropped by the parser but unmarshals fine (missed
// alert), while multiple newline-delimited messages render fine but fail a
// single-object unmarshal (false alert).
func scanA2UIBlocks(content string) a2uiBlockStats {
	var stats a2uiBlockStats
	s := content
scan:
	for {
		tagIdx := strings.Index(s, a2uiOpenTag)
		if tagIdx < 0 {
			return stats
		}
		// Mirror the parser's scan of the text ahead of the tag for bare
		// A2UI JSON objects.
		for from := 0; from < tagIdx; {
			rel := strings.IndexByte(s[from:tagIdx], '{')
			if rel < 0 {
				break
			}
			idx := from + rel
			if !atBareJSONBoundary(s, idx) || !possibleBareA2UIPrefix(s[idx:]) {
				from = idx + 1
				continue
			}
			end, complete := scanJSONObject(s[idx:])
			if !complete {
				// The parser buffers the incomplete object and everything
				// after it as plain text; the tag never becomes a delimiter.
				return stats
			}
			if acceptedBareA2UIObject(s[idx : idx+end]) {
				// The parser consumes the object and rescans from after it —
				// a tag literal inside the object's strings is message
				// content, not a delimiter.
				s = s[idx+end:]
				continue scan
			}
			from = idx + 1
		}
		rest := s[tagIdx+len(a2uiOpenTag):]
		j := strings.Index(rest, a2uiCloseTag)
		if j < 0 {
			stats.unclosed = true
			return stats
		}
		messages, payloads := classifyA2UIBlock(rest[:j])
		switch {
		case messages > 0: // renders — nothing to report
		case payloads > 0:
			stats.protocolOnly++
		default:
			stats.dropped++
		}
		s = rest[j+len(a2uiCloseTag):]
	}
}

// classifyA2UIBlock judges one complete tagged block's body with the same
// parser rendering uses, reporting how many typed (renderable) messages and
// version-neutral payload objects it yields. A block with payloads but no
// messages carries only protocol traffic (callFunction/actionResponse);
// one with neither was dropped entirely.
func classifyA2UIBlock(body string) (messages, payloads int) {
	parts, err := a2uistream.ParseAndValidate(a2uiOpenTag+body+a2uiCloseTag, nil)
	if err != nil {
		return 0, 0
	}
	for _, p := range parts {
		messages += len(p.Messages)
		payloads += len(p.Payload)
	}
	return messages, payloads
}

// The helpers below mirror a2uistream's unexported bare-JSON scanning rules
// so scanA2UIBlocks segments content exactly as the parser does.

// a2uiBareKeys are the JSON keys whose presence as a first key makes an
// object a bare A2UI message candidate for the parser.
var a2uiBareKeys = []string{
	"version",
	"functionCallId",
	"actionId",
	"wantResponse",
	"createSurface",
	"updateComponents",
	"updateDataModel",
	"deleteSurface",
	"callFunction",
	"actionResponse",
}

// a2uiPayloadKeys are the message-type keys that make the parser actually
// consume a candidate object (the version-neutral payload check).
var a2uiPayloadKeys = []string{
	"createSurface",
	"updateComponents",
	"updateDataModel",
	"deleteSurface",
	"callFunction",
	"actionResponse",
}

// atBareJSONBoundary reports whether an object starting at s[idx] sits at a
// position the parser treats as a bare-JSON boundary: the start of its
// buffer, or after whitespace.
func atBareJSONBoundary(s string, idx int) bool {
	if idx == 0 {
		return true
	}
	switch s[idx-1] {
	case ' ', '\t', '\n', '\r':
		return true
	default:
		return false
	}
}

// possibleBareA2UIPrefix reports whether s (starting at '{') could open a
// bare A2UI message: its first key must be one of the keys the parser
// recognizes.
func possibleBareA2UIPrefix(s string) bool {
	if s == "" || s[0] != '{' {
		return false
	}
	i := 1
	for i < len(s) && isJSONSpace(s[i]) {
		i++
	}
	if i == len(s) {
		return true
	}
	if s[i] != '"' {
		return false
	}
	i++
	start := i
	for i < len(s) && isJSONKeyChar(s[i]) {
		i++
	}
	fragment := s[start:i]
	if i == len(s) || s[i] != '"' {
		return hasBareA2UIKeyPrefix(fragment)
	}
	if !isBareA2UIKey(fragment) {
		return false
	}
	i++
	for i < len(s) && isJSONSpace(s[i]) {
		i++
	}
	return i == len(s) || s[i] == ':'
}

func hasBareA2UIKeyPrefix(fragment string) bool {
	if fragment == "" {
		return true
	}
	for _, key := range a2uiBareKeys {
		if strings.HasPrefix(key, fragment) {
			return true
		}
	}
	return false
}

func isBareA2UIKey(fragment string) bool {
	for _, key := range a2uiBareKeys {
		if key == fragment {
			return true
		}
	}
	return false
}

func isJSONKeyChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func isJSONSpace(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r':
		return true
	default:
		return false
	}
}

// scanJSONObject reports the end of the JSON object starting at s[0] ('{'),
// tracking strings and escapes so braces (and tag literals) inside string
// values don't fool the scan. complete is false when the object runs past the
// end of s.
func scanJSONObject(s string) (end int, complete bool) {
	if s == "" || s[0] != '{' {
		return 0, false
	}
	depth := 0
	inString := false
	escaped := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch c {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i + 1, true
			}
		}
	}
	return 0, false
}

// acceptedBareA2UIObject reports whether the parser would consume obj as a
// bare A2UI message: valid JSON carrying at least one message-type key.
func acceptedBareA2UIObject(obj string) bool {
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(obj), &m); err != nil {
		return false
	}
	for _, key := range a2uiPayloadKeys {
		if _, ok := m[key]; ok {
			return true
		}
	}
	return false
}

// repairA2UIButtons rewrites the raw JSON inside <a2ui-json> blocks to fix a
// frequent model mistake (#47): a Button carrying its label in a `text` (or
// `label`) field instead of a child Text component. A2UI's Button has no such
// field in any schema version, so the label is silently dropped when the block
// is parsed into typed components — which is why this repair must run on the
// raw JSON, before a2tea.Scan.
//
// The repair is deliberately surgical: only a Button that has a non-empty
// text/label AND no child is rewritten — a Text component is synthesized from
// the stray label and the button's child pointed at it. Valid child-based
// buttons and every other component are left untouched, and a block whose JSON
// doesn't decode is returned verbatim so the existing error-alert path still
// applies.
func repairA2UIButtons(content string) string {
	if !strings.Contains(content, a2uiOpenTag) {
		return content
	}
	var b strings.Builder
	s := content
	for {
		i := strings.Index(s, a2uiOpenTag)
		if i < 0 {
			break
		}
		j := strings.Index(s[i+len(a2uiOpenTag):], a2uiCloseTag)
		if j < 0 {
			break
		}
		body := s[i+len(a2uiOpenTag) : i+len(a2uiOpenTag)+j]
		b.WriteString(s[:i+len(a2uiOpenTag)])
		b.WriteString(repairA2UIBody(body))
		b.WriteString(a2uiCloseTag)
		s = s[i+len(a2uiOpenTag)+j+len(a2uiCloseTag):]
	}
	b.WriteString(s)
	return b.String()
}

// repairA2UIBody repairs the JSON body of a single <a2ui-json> block. The body
// may hold one message object, an array of messages, or several
// newline-delimited messages — the same forms the a2tea parser accepts. If the
// body doesn't decode, or no button needs fixing, it is returned unchanged
// (preserving the author's formatting and the malformed-block alert path).
func repairA2UIBody(body string) string {
	dec := json.NewDecoder(strings.NewReader(body))
	var vals []any
	for {
		var v any
		if err := dec.Decode(&v); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return body
		}
		vals = append(vals, v)
	}

	changed := false
	for _, v := range vals {
		switch m := v.(type) {
		case map[string]any:
			changed = repairA2UIMessage(m) || changed
		case []any:
			for _, e := range m {
				if mm, ok := e.(map[string]any); ok {
					changed = repairA2UIMessage(mm) || changed
				}
			}
		}
	}
	if !changed {
		return body
	}

	out := make([]string, 0, len(vals))
	for _, v := range vals {
		enc, err := json.Marshal(v)
		if err != nil {
			return body
		}
		out = append(out, string(enc))
	}
	return strings.Join(out, "\n")
}

// repairA2UIMessage fixes childless-but-labeled buttons inside one raw A2UI
// message map, reporting whether it changed anything. For each such button it
// moves the stray text/label into a new Text component (with a unique id
// derived from the button's) and points the button's child at it.
func repairA2UIMessage(msg map[string]any) bool {
	uc, ok := msg["updateComponents"].(map[string]any)
	if !ok {
		return false
	}
	comps, ok := uc["components"].([]any)
	if !ok {
		return false
	}

	ids := make(map[string]bool, len(comps))
	for _, c := range comps {
		if m, ok := c.(map[string]any); ok {
			if id, ok := m["id"].(string); ok {
				ids[id] = true
			}
		}
	}

	var added []any
	for _, c := range comps {
		m, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if typ, _ := m["component"].(string); typ != "Button" {
			continue
		}
		switch child := m["child"].(type) {
		case nil: // absent (or JSON null): childless, repairable
		case string:
			if child != "" {
				continue // valid child-based button: leave untouched
			}
		default:
			continue // e.g. inline-nested object: not ours to rewrite
		}
		label, _ := m["text"].(string)
		if label == "" {
			label, _ = m["label"].(string)
		}
		if label == "" {
			continue
		}

		base := "label"
		if id, _ := m["id"].(string); id != "" {
			base = id + "-label"
		}
		labelID := base
		for n := 2; ids[labelID]; n++ {
			labelID = fmt.Sprintf("%s-%d", base, n)
		}
		ids[labelID] = true

		m["child"] = labelID
		delete(m, "text")
		delete(m, "label")
		added = append(added, map[string]any{
			"component": "Text",
			"id":        labelID,
			"text":      label,
		})
	}
	if len(added) == 0 {
		return false
	}
	uc["components"] = append(comps, added...)
	return true
}

// syncA2UISurfaces builds — or reuses — the live a2tea models for the
// scanned parts (#44). Surfaces are keyed by a hash of the scanned source:
// streaming deltas rebuild them, while pure re-renders (width changes,
// focus flips, key events) reuse the same models and preserve their
// interaction state (focus ring position, edited field values). The slice
// is part-indexed; parts without a renderable surface hold nil.
func (a *AssistantMessageItem) syncA2UISurfaces(src string, parts []a2tea.Part) {
	h := fnv64(src)
	if a.a2uiScanned && a.a2uiSrcHash == h && len(a.a2uiSurfaces) == len(parts) {
		return
	}
	surfaces := make([]render.Model, len(parts))
	ids := make([]string, len(parts))
	for i, p := range parts {
		if len(p.Messages) == 0 {
			continue
		}
		model, err := a2tea.Render(p.Messages)
		if err != nil {
			// Valid A2UI with nothing to draw (e.g. a bare data-model
			// update). The render loop skips nil entries; the block-stats
			// pass decides whether anything needs alerting.
			continue
		}
		if rm, ok := model.(render.Model); ok {
			surfaces[i] = rm
			ids[i] = a2uiPartSurfaceID(p.Messages)
		}
	}
	a.a2uiSurfaces = surfaces
	a.a2uiSurfaceIDs = ids
	a.a2uiSrcHash = h
	a.a2uiScanned = true
	// A rebuild replaces focused models with fresh blurred ones; re-grant
	// focus when the item is selected so the ring survives streaming.
	if a.focusableMessageItem.isFocused() {
		a.focusA2UISurfaces()
	}
}

// dropA2UISurfaces discards the live surface models, e.g. when the content
// no longer scans as A2UI. Retirement marks are kept: they are keyed by
// surface ID, and a surface that reappears after a rescan was still already
// submitted (#45).
func (a *AssistantMessageItem) dropA2UISurfaces() {
	a.a2uiSurfaces = nil
	a.a2uiSurfaceIDs = nil
	a.a2uiSrcHash = 0
	a.a2uiScanned = false
}

// a2uiPartSurfaceID extracts the surface ID a scan-part's messages establish:
// the first updateComponents' surfaceId — the same rule a2tea.Render uses to
// pick the surface it draws, so the ID here always names the rendered model.
func a2uiPartSurfaceID(msgs []a2ui.ServerMessage) string {
	for _, m := range msgs {
		if m.UpdateComponents != nil {
			return m.UpdateComponents.SurfaceID
		}
	}
	return ""
}

// a2uiSurfaceRetired reports whether the surface at part-index i has been
// retired after a submission (#45). Retired surfaces still render (showing
// their final state) but no longer receive focus or keys.
func (a *AssistantMessageItem) a2uiSurfaceRetired(i int) bool {
	if i < 0 || i >= len(a.a2uiSurfaceIDs) {
		return false
	}
	return a.a2uiRetired[a.a2uiSurfaceIDs[i]]
}

// RetireA2UISurface retires the live surface with the given A2UI surface ID
// after a button press (#45): it reads the surface's current field values,
// revokes its focus, and marks it retired so the form cannot be re-submitted
// — the mark is keyed by surface ID, so it survives streaming rebuilds of
// the model. It returns the gathered values and whether the surface was
// found on this item.
func (a *AssistantMessageItem) RetireA2UISurface(surfaceID string) (map[string]any, bool) {
	if surfaceID == "" {
		return nil, false
	}
	for i, s := range a.a2uiSurfaces {
		if s == nil || i >= len(a.a2uiSurfaceIDs) || a.a2uiSurfaceIDs[i] != surfaceID {
			continue
		}
		if a.a2uiRetired[surfaceID] {
			// Already submitted or dismissed: report not-found so a
			// duplicate event cannot re-read values from a dead form.
			return nil, false
		}
		var values map[string]any
		if fv, ok := s.(interface{ FieldValues() map[string]any }); ok {
			values = fv.FieldValues()
		}
		if a.a2uiRetired == nil {
			a.a2uiRetired = make(map[string]bool)
		}
		a.a2uiRetired[surfaceID] = true
		s.Blur()
		a.Bump()
		return values, true
	}
	return nil, false
}

// a2uiCancelWords are the button-name tokens read as "dismiss this form":
// pressing such a button retires the surface without starting an agent turn
// (#45). Matched against whole tokens of the button's component ID and its
// action name, never as substrings, so ids like "notify" or "disclose" do
// not false-positive.
var a2uiCancelWords = map[string]bool{
	"cancel":  true,
	"dismiss": true,
	"close":   true,
	"no":      true,
}

// A2UIButtonIsCancel reports whether an activated button reads as a cancel /
// dismiss control. A2UI has no cancel semantics of its own — model-authored
// buttons typically carry no action at all — so the judgment is by naming
// convention on the component ID (e.g. "btn-cancel", "dismissBtn") and, when
// present, the server action's name.
func A2UIButtonIsCancel(clicked event.ButtonClicked) bool {
	if a2uiHasCancelToken(clicked.ID) {
		return true
	}
	return clicked.Action != nil && a2uiHasCancelToken(clicked.Action.Name)
}

// a2uiHasCancelToken tokenizes s on non-alphanumeric and camelCase
// boundaries and reports whether any token is a cancel word.
func a2uiHasCancelToken(s string) bool {
	for _, tok := range a2uiNameTokens(s) {
		if a2uiCancelWords[tok] {
			return true
		}
	}
	return false
}

// a2uiNameTokens splits an identifier-ish name ("btn-cancel", "dismissBtn",
// "close_form") into lowercase word tokens.
func a2uiNameTokens(s string) []string {
	var tokens []string
	var b strings.Builder
	flush := func() {
		if b.Len() > 0 {
			tokens = append(tokens, strings.ToLower(b.String()))
			b.Reset()
		}
	}
	var prev rune
	for _, r := range s {
		switch {
		case !unicode.IsLetter(r) && !unicode.IsDigit(r):
			flush()
		case unicode.IsUpper(r) && unicode.IsLower(prev):
			flush()
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
		prev = r
	}
	flush()
	return tokens
}

// A2UISubmissionPrompt builds the agent-facing message for an A2UI form
// submission (#45): which button was pressed on which surface, plus the
// surface's field values at the moment of submission. Values are keyed by
// component ID, sorted for determinism, and JSON-encoded so their types
// (string, bool, number, string list) survive the trip through prose.
func A2UISubmissionPrompt(clicked event.ButtonClicked, values map[string]any) string {
	var b strings.Builder
	b.WriteString("[A2UI form submission] The user pressed the ")
	fmt.Fprintf(&b, "%q button", clicked.ID)
	if clicked.Action != nil && clicked.Action.Name != "" {
		fmt.Fprintf(&b, " (action %q)", clicked.Action.Name)
	}
	if clicked.SurfaceID != "" {
		fmt.Fprintf(&b, " on surface %q", clicked.SurfaceID)
	}
	b.WriteString(".")
	if len(values) > 0 {
		b.WriteString("\nField values:")
		for _, k := range slices.Sorted(maps.Keys(values)) {
			enc, err := json.Marshal(values[k])
			if err != nil {
				enc = fmt.Appendf(nil, "%q", fmt.Sprintf("%v", values[k]))
			}
			fmt.Fprintf(&b, "\n- %s: %s", k, enc)
		}
	}
	return b.String()
}

// a2uiSurfaceAt returns the live surface for scan-part i, or nil when the
// part has none (or the index is stale).
func (a *AssistantMessageItem) a2uiSurfaceAt(i int) render.Model {
	if i < 0 || i >= len(a.a2uiSurfaces) {
		return nil
	}
	return a.a2uiSurfaces[i]
}

// hasLiveA2UISurfaces reports whether the item holds at least one live
// a2tea surface model. While true, the content-hash render caches are
// bypassed so surface interaction is never served a frozen frame.
func (a *AssistantMessageItem) hasLiveA2UISurfaces() bool {
	for _, s := range a.a2uiSurfaces {
		if s != nil {
			return true
		}
	}
	return false
}

// focusA2UISurfaces grants keyboard focus to the first live, non-retired
// surface and blurs the rest, honoring the a2tea composition contract of at
// most one focused child at a time. Retired surfaces (#45) never regain
// focus — a submitted form must not be re-submittable. render.Surface's
// Focus returns a nil cmd, so dropping it here loses nothing.
func (a *AssistantMessageItem) focusA2UISurfaces() {
	granted := false
	for i, s := range a.a2uiSurfaces {
		if s == nil {
			continue
		}
		if !granted && !a.a2uiSurfaceRetired(i) {
			_ = s.Focus()
			granted = true
			continue
		}
		s.Blur()
	}
}

// blurA2UISurfaces revokes keyboard focus from every live surface.
func (a *AssistantMessageItem) blurA2UISurfaces() {
	for _, s := range a.a2uiSurfaces {
		if s != nil {
			s.Blur()
		}
	}
}

// focusedA2UISurfaceIndex returns the index of the surface currently
// holding keyboard focus, or -1 when none does.
func (a *AssistantMessageItem) focusedA2UISurfaceIndex() int {
	for i, s := range a.a2uiSurfaces {
		if s != nil && s.Focused() {
			return i
		}
	}
	return -1
}

// updateA2UISurface forwards msg into the surface's Update, stores the
// returned model back, and bumps the item version so the list re-renders
// the surface's new state on the next draw.
func (a *AssistantMessageItem) updateA2UISurface(idx int, msg tea.Msg) tea.Cmd {
	model, cmd := a.a2uiSurfaces[idx].Update(msg)
	if rm, ok := model.(render.Model); ok {
		a.a2uiSurfaces[idx] = rm
	}
	a.Bump()
	return cmd
}

// a2uiSurfaceWantsKey reports whether a focused surface should consume the
// key. The whitelist mirrors the keys render.Surface.Update reacts to —
// forwarding everything would swallow host shortcuts (copy, help, quit)
// whenever a surface item is selected. While a text-editable component is
// being edited every key is potential input; Esc stays with the host
// unless a modal is open to close.
func a2uiSurfaceWantsKey(s render.Model, key tea.KeyMsg) bool {
	k := key.String()
	if k == "esc" {
		m, ok := s.(interface{ HasOpenModal() bool })
		return ok && m.HasOpenModal()
	}
	if e, ok := s.(interface{ EditingText() bool }); ok && e.EditingText() {
		return true
	}
	switch k {
	case "tab", "shift+tab", "enter", "space", "up", "down", "left", "right", "h", "l", "backspace":
		return true
	}
	return false
}

// renderContentWithA2UI renders assistant content that contains A2UI. a2tea
// scans the content into ordered parts of prose text and typed A2UI messages;
// crush renders the prose as markdown and hands each part's messages to
// a2tea.Render, stitching the rendered surface in place.
//
// Markdown code (fences, and inline spans naming A2UI markers) is masked
// before scanning so that A2UI examples in documentation prose are not
// extracted as live surfaces (#6, #96). If any complete tag
// pair contains malformed JSON — a block a2tea drops silently — an alert
// element is appended (#7). An unclosed <a2ui-json> tag (truncated
// generation) also triggers the alert (#5).
//
// This deliberately bypasses the streaming-markdown prefix cache (which assumes
// a single glamour render per item) and renders each segment directly. The
// renderer is shared, so the whole multi-render sequence holds its lock.
func (a *AssistantMessageItem) renderContentWithA2UI(content string, width int, finished bool) string {
	masked, codeReps := maskMarkdownCode(content)
	// Repair childless-but-labeled buttons on the raw JSON before parsing —
	// the stray label field is dropped by the typed parser (#47). Runs after
	// masking so button examples inside fenced code stay verbatim.
	masked = repairA2UIButtons(masked)

	parts, err := a2tea.Scan(masked)
	if err != nil {
		// Not parseable as A2UI — render everything as markdown so nothing is
		// lost.
		a.dropA2UISurfaces()
		return a.renderMarkdown(content, width)
	}

	// Keep the a2tea models alive on the item (rebuilt only when the source
	// changed) so the surfaces can receive focus and key input instead of
	// being frozen to a string (#44).
	a.syncA2UISurfaces(masked, parts)

	renderer := common.MarkdownRenderer(a.sty, width)
	mu := common.LockMarkdownRenderer(renderer)
	mu.Lock()
	defer mu.Unlock()

	renderMarkdown := func(text string) string {
		if strings.TrimSpace(text) == "" {
			return ""
		}
		// Restore masked code before markdown rendering.
		text = unmaskMarkdownCode(text, codeReps)
		out, err := renderer.Render(text)
		if err != nil {
			return strings.TrimSpace(text)
		}
		return trimGlamourMargins(out)
	}

	var b strings.Builder
	writeChunk := func(s string) {
		if s == "" {
			return
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(s)
	}

	// a2tea's chrome renders with terminal defaults (monochrome by design),
	// so each surface is wrapped in a themed container to match the rest of
	// the chat. The a2tea model is sized to the container's inner width.
	surface := a.sty.Messages.A2UISurface
	innerWidth := max(width-surface.GetHorizontalFrameSize(), 1)
	for i, p := range parts {
		writeChunk(renderMarkdown(p.Text))
		if len(p.Messages) == 0 {
			continue
		}
		model := a.a2uiSurfaceAt(i)
		if model == nil {
			// Valid A2UI messages with nothing to draw (e.g. a data-model
			// update). Not an error worth alarming the user about.
			continue
		}
		// Render from the live model each call: its View reflects the
		// current focus ring and edited values, not a frozen snapshot.
		model.SetSize(innerWidth, 0)
		rendered := strings.TrimRight(model.View().Content, "\n")
		writeChunk(surface.Width(max(width-surface.GetHorizontalBorderSize(), 1)).Render(rendered))
	}

	// Alert when the parser dropped a complete tagged block (#7) — checked
	// directly so bare-JSON parts cannot mask the count — or when generation
	// was truncated mid-block (#5). Blocks carrying only protocol messages
	// were understood, just not drawable, so they get a quiet notice instead
	// (#168). Only for finished messages: while streaming, an "unclosed" tag
	// usually just means the close tag hasn't arrived yet, and flashing a red
	// alert between flushes that then vanishes reads as a glitch.
	if finished {
		stats := scanA2UIBlocks(masked)
		switch {
		case stats.dropped > 0 || stats.unclosed:
			writeChunk(a.renderA2UIAlert(width))
		case stats.protocolOnly > 0:
			writeChunk(a.renderA2UIProtocolNotice(width))
		}
	}

	return b.String()
}

// renderTruncatedA2UI handles a finished message whose <a2ui-json> block was
// never closed — generation was truncated mid-block (#5). The prose before
// the unclosed tag is rendered as markdown, and the truncated block is
// surfaced through the standard A2UI alert instead of leaving a wall of raw
// JSON.
func (a *AssistantMessageItem) renderTruncatedA2UI(content string, width int) string {
	// Mask (not strip) markdown code: a fence or inline span containing an
	// <a2ui-json> example must not be mistaken for the truncation point, but
	// the code must stay in the rendered prose — stripping deleted it from
	// the message.
	masked, codeReps := maskMarkdownCode(content)
	idx := strings.Index(masked, "<a2ui-json>")

	var b strings.Builder
	if idx > 0 {
		prose := unmaskMarkdownCode(masked[:idx], codeReps)
		if strings.TrimSpace(prose) != "" {
			b.WriteString(a.renderMarkdown(prose, width))
		}
	}
	if b.Len() > 0 {
		b.WriteString("\n\n")
	}
	b.WriteString(a.renderA2UIAlert(width))
	return b.String()
}

// renderA2UIAlert builds an alert element shown when content advertised A2UI but
// a2tea could not turn it into a surface. Styled in crush's existing
// error-message language.
func (a *AssistantMessageItem) renderA2UIAlert(width int) string {
	inner := max(width-2, 1)
	tag := a.sty.Messages.ErrorTag.Render("A2UI")
	title := a.sty.Messages.ErrorTitle.Render("couldn't render a UI block in this message")
	reason := a.sty.Messages.ErrorDetails.Width(inner).Render(
		"The A2UI content was malformed or used unsupported components.")
	return tag + " " + title + "\n\n" + reason
}

// renderA2UIProtocolNotice is the quiet counterpart to renderA2UIAlert for
// tagged blocks holding only protocol messages (callFunction/actionResponse):
// the content was understood, there is simply nothing to draw, so a muted
// one-liner replaces the misleading malformed-content alert (#168).
func (a *AssistantMessageItem) renderA2UIProtocolNotice(width int) string {
	inner := max(width-2, 1)
	return a.sty.Messages.ErrorDetails.Width(inner).Render(
		"This message includes A2UI protocol data with nothing to display.")
}
