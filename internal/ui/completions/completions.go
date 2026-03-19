package completions

import (
	"cmp"
	"slices"
	"strings"
	"sync"
	"unicode"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/ordered"
	"github.com/sahilm/fuzzy"
)

const (
	minHeight = 1
	maxHeight = 10
	minWidth  = 10
	maxWidth  = 100

	// Scoring weights for fuzzy matching.
	// Full path fuzzy match contributes most to the score.
	fullMatchWeight = 1_000
	// Basename fuzzy match contributes less than full path.
	baseMatchWeight = 300

	// Bonus points for exact matches (case-insensitive).
	// Path prefix match (e.g., "src/" matches "src/main.go") gets highest bonus.
	pathPrefixBonus = 5_000
	// Path contains match (e.g., "main" matches "src/main.go").
	pathContainsBonus = 2_000
	// Additional bonus when path hint is detected (user typed "/" or file extension).
	pathContainsHintBonus = 2_500
	// Basename prefix match (e.g., "main" matches "main.go").
	basePrefixBonus = 1_500
	// Basename contains match (e.g., "mai" matches "main.go").
	baseContainsBonus = 500
	// Smaller bonuses when path hint is detected.
	basePrefixHintBonus   = 300
	baseContainsHintBonus = 120

	// Penalties for deeply nested files to favor shallow matches.
	// Default penalty per directory level (e.g., "a/b/c" has 2 levels).
	depthPenaltyDefault = 20
	// Reduced penalty when user explicitly queries a path (typed "/" or extension).
	depthPenaltyPathHint = 5
)

// SelectionMsg is sent when a completion is selected.
type SelectionMsg[T any] struct {
	Value    T
	KeepOpen bool // If true, insert without closing.
}

// ClosedMsg is sent when the completions are closed.
type ClosedMsg struct{}

// CompletionItemsLoadedMsg is sent when files have been loaded for completions.
type CompletionItemsLoadedMsg struct {
	Files     []FileCompletionValue
	Resources []ResourceCompletionValue
}

// Completions represents the completions popup component.
type Completions struct {
	// Popup dimensions
	width  int
	height int

	// State
	open  bool
	query string

	// Key bindings
	keyMap KeyMap

	// List component
	list *list.FilterableList

	// Styling
	normalStyle  lipgloss.Style
	focusedStyle lipgloss.Style
	matchStyle   lipgloss.Style

	// Custom ranking state (replaces list.FilterableList's built-in filtering).
	items    []*CompletionItem // All completion items.
	filtered []*CompletionItem // Filtered and ranked items based on query.
	paths    []string          // Pre-computed full paths for matching.
	bases    []string          // Pre-computed basenames for matching.
}

// New creates a new completions component.
func New(normalStyle, focusedStyle, matchStyle lipgloss.Style) *Completions {
	l := list.NewFilterableList()
	l.SetGap(0)
	l.SetReverse(true)

	return &Completions{
		keyMap:       DefaultKeyMap(),
		list:         l,
		normalStyle:  normalStyle,
		focusedStyle: focusedStyle,
		matchStyle:   matchStyle,
	}
}

// IsOpen returns whether the completions popup is open.
func (c *Completions) IsOpen() bool {
	return c.open
}

// Query returns the current filter query.
func (c *Completions) Query() string {
	return c.query
}

// Size returns the visible size of the popup.
func (c *Completions) Size() (width, height int) {
	visible := len(c.filtered)
	return c.width, min(visible, c.height)
}

// KeyMap returns the key bindings.
func (c *Completions) KeyMap() KeyMap {
	return c.keyMap
}

// Open opens the completions with file items from the filesystem.
func (c *Completions) Open(depth, limit int) tea.Cmd {
	return func() tea.Msg {
		var msg CompletionItemsLoadedMsg
		var wg sync.WaitGroup
		wg.Go(func() {
			msg.Files = loadFiles(depth, limit)
		})
		wg.Go(func() {
			msg.Resources = loadMCPResources()
		})
		wg.Wait()
		return msg
	}
}

// SetItems sets the files and MCP resources and rebuilds the merged list.
func (c *Completions) SetItems(files []FileCompletionValue, resources []ResourceCompletionValue) {
	items := make([]*CompletionItem, 0, len(files)+len(resources))

	// Add files first.
	for _, file := range files {
		item := NewCompletionItem(
			file.Path,
			file,
			c.normalStyle,
			c.focusedStyle,
			c.matchStyle,
		)
		items = append(items, item)
	}

	// Add MCP resources.
	for _, resource := range resources {
		item := NewCompletionItem(
			resource.MCPName+"/"+cmp.Or(resource.Title, resource.URI),
			resource,
			c.normalStyle,
			c.focusedStyle,
			c.matchStyle,
		)
		items = append(items, item)
	}

	c.open = true
	c.query = ""
	c.items = items
	// Pre-compute paths and basenames for efficient fuzzy matching.
	c.paths = make([]string, len(items))
	c.bases = make([]string, len(items))
	for i, item := range items {
		path := item.Filter()
		c.paths[i] = path
		c.bases[i] = pathBase(path)
	}
	// Perform initial ranking with empty query (returns all items).
	c.filtered = c.rank(queryContext{
		query: c.query,
	})
	c.setVisibleItems(c.filtered)
	c.list.Focus()

	c.width = maxWidth
	c.height = ordered.Clamp(len(items), int(minHeight), int(maxHeight))
	c.list.SetSize(c.width, c.height)
	c.list.SelectFirst()
	c.list.ScrollToSelected()

	c.updateSize()
}

// Close closes the completions popup.
func (c *Completions) Close() {
	c.open = false
}

// Filter filters the completions with the given query.
func (c *Completions) Filter(query string) {
	if !c.open {
		return
	}

	if query == c.query {
		return
	}

	c.query = query
	// Apply custom ranking algorithm instead of list's built-in filtering.
	c.filtered = c.rank(queryContext{
		query: query,
	})
	c.setVisibleItems(c.filtered)

	c.updateSize()
}

func (c *Completions) updateSize() {
	items := c.filtered
	start, end := c.list.VisibleItemIndices()
	width := 0
	for i := start; i <= end; i++ {
		item := c.list.ItemAt(i)
		if item == nil {
			continue
		}
		s := item.(interface{ Text() string }).Text()
		width = max(width, ansi.StringWidth(s))
	}
	c.width = ordered.Clamp(width+2, int(minWidth), int(maxWidth))
	c.height = ordered.Clamp(len(items), int(minHeight), int(maxHeight))
	c.list.SetSize(c.width, c.height)
	c.list.SelectFirst()
	c.list.ScrollToSelected()
}

// HasItems returns whether there are visible items.
func (c *Completions) HasItems() bool {
	return len(c.filtered) > 0
}

// Update handles key events for the completions.
func (c *Completions) Update(msg tea.KeyPressMsg) (tea.Msg, bool) {
	if !c.open {
		return nil, false
	}

	switch {
	case key.Matches(msg, c.keyMap.Up):
		c.selectPrev()
		return nil, true

	case key.Matches(msg, c.keyMap.Down):
		c.selectNext()
		return nil, true

	case key.Matches(msg, c.keyMap.UpInsert):
		c.selectPrev()
		return c.selectCurrent(true), true

	case key.Matches(msg, c.keyMap.DownInsert):
		c.selectNext()
		return c.selectCurrent(true), true

	case key.Matches(msg, c.keyMap.Select):
		return c.selectCurrent(false), true

	case key.Matches(msg, c.keyMap.Cancel):
		c.Close()
		return ClosedMsg{}, true
	}

	return nil, false
}

// selectPrev selects the previous item with circular navigation.
func (c *Completions) selectPrev() {
	items := c.filtered
	if len(items) == 0 {
		return
	}
	if !c.list.SelectPrev() {
		c.list.WrapToEnd()
	}
	c.list.ScrollToSelected()
}

// selectNext selects the next item with circular navigation.
func (c *Completions) selectNext() {
	items := c.filtered
	if len(items) == 0 {
		return
	}
	if !c.list.SelectNext() {
		c.list.WrapToStart()
	}
	c.list.ScrollToSelected()
}

// selectCurrent returns a command with the currently selected item.
func (c *Completions) selectCurrent(keepOpen bool) tea.Msg {
	items := c.filtered
	if len(items) == 0 {
		return nil
	}

	selected := c.list.Selected()
	if selected < 0 || selected >= len(items) {
		return nil
	}

	item := items[selected]

	if !keepOpen {
		c.open = false
	}

	switch item := item.Value().(type) {
	case ResourceCompletionValue:
		return SelectionMsg[ResourceCompletionValue]{
			Value:    item,
			KeepOpen: keepOpen,
		}
	case FileCompletionValue:
		return SelectionMsg[FileCompletionValue]{
			Value:    item,
			KeepOpen: keepOpen,
		}
	default:
		return nil
	}
}

// Render renders the completions popup.
func (c *Completions) Render() string {
	if !c.open {
		return ""
	}

	items := c.filtered
	if len(items) == 0 {
		return ""
	}

	return c.list.Render()
}

func (c *Completions) setVisibleItems(items []*CompletionItem) {
	filterables := make([]list.FilterableItem, 0, len(items))
	for _, item := range items {
		filterables = append(filterables, item)
	}
	c.list.SetItems(filterables...)
}

type queryContext struct {
	query string
}

type rankedItem struct {
	item  *CompletionItem
	score int
}

// rank uses path-first fuzzy ordering with basename as a secondary boost.
//
// Ranking strategy:
// 1. Perform fuzzy matching on both full paths and basenames.
// 2. Apply bonus points for exact prefix/contains matches.
// 3. Adjust bonuses based on whether query contains path hints (/, \, or file extension).
// 4. Apply depth penalty to favor shallower files.
// 5. Sort by score (descending), then alphabetically.
//
// Example scoring for query "main.go":
//   - "main.go" (root): high fuzzy + pathPrefix + basePrefix + low depth penalty
//   - "src/main.go": high fuzzy + pathContains + basePrefix + moderate depth penalty
//   - "test/helper/main.go": high fuzzy + pathContains + basePrefix + high depth penalty
func (c *Completions) rank(ctx queryContext) []*CompletionItem {
	query := strings.TrimSpace(ctx.query)
	if query == "" {
		// Empty query: return all items with no highlights.
		for _, item := range c.items {
			item.SetMatch(fuzzy.Match{})
		}
		return c.items
	}

	// Perform fuzzy matching on both full paths and basenames.
	fullMatches := matchIndex(query, c.paths)
	baseMatches := matchIndex(query, c.bases)
	// Collect unique item indices that matched either full path or basename.
	allIndexes := make(map[int]struct{}, len(fullMatches)+len(baseMatches))
	for idx := range fullMatches {
		allIndexes[idx] = struct{}{}
	}
	for idx := range baseMatches {
		allIndexes[idx] = struct{}{}
	}

	queryLower := strings.ToLower(query)
	// Detect if query looks like a path (contains / or \ or file extension).
	pathHint := hasPathHint(query)
	ranked := make([]rankedItem, 0, len(allIndexes))
	for idx := range allIndexes {
		path := c.paths[idx]
		pathLower := strings.ToLower(path)
		baseLower := strings.ToLower(c.bases[idx])

		fullMatch, hasFullMatch := fullMatches[idx]
		baseMatch, hasBaseMatch := baseMatches[idx]

		// Check for exact (case-insensitive) prefix/contains matches.
		pathPrefix := strings.HasPrefix(pathLower, queryLower)
		pathContains := strings.Contains(pathLower, queryLower)
		basePrefix := strings.HasPrefix(baseLower, queryLower)
		baseContains := strings.Contains(baseLower, queryLower)

		// Calculate score by accumulating weighted components.
		score := 0
		// Fuzzy match scores (primary signals).
		if hasFullMatch {
			score += fullMatch.Score * fullMatchWeight
		}
		if hasBaseMatch {
			score += baseMatch.Score * baseMatchWeight
		}
		// Path-level exact match bonuses.
		if pathPrefix {
			score += pathPrefixBonus
		}
		if pathContains {
			score += pathContainsBonus
		}
		// Apply different bonuses based on whether query contains path hints.
		if pathHint {
			// User typed a path-like query (e.g., "src/main.go" or "main.go").
			// Prioritize full path matches.
			if pathContains {
				score += pathContainsHintBonus
			}
			if basePrefix {
				score += basePrefixHintBonus
			}
			if baseContains {
				score += baseContainsHintBonus
			}
		} else {
			// User typed a simple query (e.g., "main").
			// Prioritize basename matches.
			if basePrefix {
				score += basePrefixBonus
			}
			if baseContains {
				score += baseContainsBonus
			}
		}

		// Apply penalties to discourage deeply nested files.
		depthPenalty := depthPenaltyDefault
		if pathHint {
			// Reduce penalty when user explicitly queries a path.
			depthPenalty = depthPenaltyPathHint
		}
		score -= strings.Count(path, "/") * depthPenalty
		// Minor penalty based on path length (favor shorter paths).
		score -= ansi.StringWidth(path)

		// Choose which match to highlight based on weighted contribution.
		if hasFullMatch && (!hasBaseMatch || fullMatch.Score*fullMatchWeight >= baseMatch.Score*baseMatchWeight) {
			c.items[idx].SetMatch(fullMatch)
		} else if hasBaseMatch {
			c.items[idx].SetMatch(remapMatchToPath(baseMatch, path))
		} else {
			c.items[idx].SetMatch(fuzzy.Match{})
		}

		ranked = append(ranked, rankedItem{
			item:  c.items[idx],
			score: score,
		})
	}

	slices.SortStableFunc(ranked, func(a, b rankedItem) int {
		if a.score != b.score {
			// Higher score first.
			return b.score - a.score
		}
		// Tie-breaker: sort alphabetically.
		return strings.Compare(a.item.Text(), b.item.Text())
	})

	result := make([]*CompletionItem, 0, len(ranked))
	for _, item := range ranked {
		result = append(result, item.item)
	}
	return result
}

// matchIndex performs fuzzy matching and returns a map of item index to match result.
func matchIndex(query string, values []string) map[int]fuzzy.Match {
	source := stringSource(values)
	matches := fuzzy.FindFrom(query, source)
	result := make(map[int]fuzzy.Match, len(matches))
	for _, match := range matches {
		result[match.Index] = match
	}
	return result
}

// stringSource adapts []string to fuzzy.Source interface.
type stringSource []string

func (s stringSource) Len() int {
	return len(s)
}

func (s stringSource) String(i int) string {
	return s[i]
}

// pathBase extracts the basename from a file path (handles both / and \ separators).
// Examples:
//   - "src/main.go" → "main.go"
//   - "file.txt" → "file.txt"
//   - "dir/" → "dir/"
func pathBase(value string) string {
	trimmed := strings.TrimRight(value, `/\`)
	if trimmed == "" {
		return value
	}
	idx := strings.LastIndexAny(trimmed, `/\`)
	if idx < 0 {
		return trimmed
	}
	return trimmed[idx+1:]
}

// remapMatchToPath remaps a basename match's character indices to full path indices.
// Example:
//   - baseMatch for "main" in "main.go" with indices [0,1,2,3]
//   - fullPath is "src/main.go" (offset 4)
//   - Result: [4,5,6,7] (highlights "main" in full path)
func remapMatchToPath(match fuzzy.Match, fullPath string) fuzzy.Match {
	base := pathBase(fullPath)
	if base == "" {
		return match
	}
	offset := len(fullPath) - len(base)
	remapped := make([]int, 0, len(match.MatchedIndexes))
	for _, idx := range match.MatchedIndexes {
		remapped = append(remapped, offset+idx)
	}
	match.MatchedIndexes = remapped
	return match
}

// hasPathHint detects if the query looks like a file path query.
// Returns true if:
//   - Query contains "/" or "\" (explicit path separator)
//   - Query ends with a file extension pattern (e.g., ".go", ".ts")
//
// File extension heuristics:
//   - Must have a dot not at start/end (e.g., "main.go" ✓, "v0.1" ✗, ".gitignore" ✓)
//   - Extension must be ≤12 chars (e.g., ".go" ✓, ".verylongextension" ✗)
//   - Extension must contain at least one letter and only alphanumeric/_/- chars
//
// Examples:
//   - "src/main" → true (contains /)
//   - "main.go" → true (file extension)
//   - ".gitignore" → true (file extension)
//   - "v0.1" → false (no letter in suffix)
//   - "main" → false (no path hint)
func hasPathHint(query string) bool {
	if strings.Contains(query, "/") || strings.Contains(query, "\\") {
		return true
	}

	// Check for file extension pattern.
	lastDot := strings.LastIndex(query, ".")
	if lastDot < 0 || lastDot == len(query)-1 {
		// No dot or dot at end (e.g., "main" or "foo.").
		return false
	}

	suffix := query[lastDot+1:]
	if len(suffix) > 12 {
		// Extension too long (unlikely to be a real extension).
		return false
	}

	// Validate that suffix looks like a file extension:
	// - Contains only alphanumeric, underscore, or hyphen.
	// - Contains at least one letter (to exclude version numbers like "v0.1").
	hasLetter := false
	for _, r := range suffix {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' {
			return false
		}
		if unicode.IsLetter(r) {
			hasLetter = true
		}
	}

	return hasLetter
}

func loadFiles(depth, limit int) []FileCompletionValue {
	files, _, _ := fsext.ListDirectory(".", nil, depth, limit)
	slices.Sort(files)
	result := make([]FileCompletionValue, 0, len(files))
	for _, file := range files {
		result = append(result, FileCompletionValue{
			Path: strings.TrimPrefix(file, "./"),
		})
	}
	return result
}

func loadMCPResources() []ResourceCompletionValue {
	var resources []ResourceCompletionValue
	for mcpName, mcpResources := range mcp.Resources() {
		for _, r := range mcpResources {
			resources = append(resources, ResourceCompletionValue{
				MCPName:  mcpName,
				URI:      r.URI,
				Title:    r.Name,
				MIMEType: r.MIMEType,
			})
		}
	}
	return resources
}
