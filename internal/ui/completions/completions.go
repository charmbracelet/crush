package completions

import (
	"cmp"
	"path/filepath"
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

	fullMatchWeight = 1_000
	baseMatchWeight = 300

	pathPrefixBonus       = 5_000
	pathContainsBonus     = 2_000
	pathContainsHintBonus = 2_500
	basePrefixBonus       = 1_500
	baseContainsBonus     = 500
	basePrefixHintBonus   = 300
	baseContainsHintBonus = 120

	depthPenaltyDefault  = 20
	depthPenaltyPathHint = 5
)

const (
	tierExactName = iota
	tierPrefixName
	tierPathSegment
	tierFallback
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

	// Items and ranking state
	items    []*CompletionItem
	filtered []*CompletionItem
	paths    []string
	bases    []string
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
	visible := len(c.list.FilteredItems())
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
	items := make([]list.FilterableItem, 0, len(files)+len(resources))

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
	c.items = make([]*CompletionItem, len(items))
	for i, item := range items {
		c.items[i] = item.(*CompletionItem)
	}
	c.paths = make([]string, len(c.items))
	c.bases = make([]string, len(c.items))
	for i, item := range c.items {
		path := item.Filter()
		c.paths[i] = path
		c.bases[i] = pathBase(path)
	}
	c.filtered = c.rank(queryContext{query: c.query})
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
	c.filtered = c.rank(queryContext{query: query})
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

	return c.list.List.Render()
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

func (c *Completions) rank(ctx queryContext) []*CompletionItem {
	query := strings.TrimSpace(ctx.query)
	if query == "" {
		for _, item := range c.items {
			item.SetMatch(fuzzy.Match{})
		}
		return c.items
	}

	fullMatches := matchIndex(query, c.paths)
	baseMatches := matchIndex(query, c.bases)
	allIndexes := make(map[int]struct{}, len(fullMatches)+len(baseMatches))
	for idx := range fullMatches {
		allIndexes[idx] = struct{}{}
	}
	for idx := range baseMatches {
		allIndexes[idx] = struct{}{}
	}

	queryLower := strings.ToLower(query)
	pathHint := hasPathHint(query)
	ranked := make([]rankedItem, 0, len(allIndexes))
	for idx := range allIndexes {
		path := c.paths[idx]
		pathLower := strings.ToLower(path)
		baseLower := strings.ToLower(c.bases[idx])

		fullMatch, hasFullMatch := fullMatches[idx]
		baseMatch, hasBaseMatch := baseMatches[idx]

		pathPrefix := strings.HasPrefix(pathLower, queryLower)
		pathContains := strings.Contains(pathLower, queryLower)
		basePrefix := strings.HasPrefix(baseLower, queryLower)
		baseContains := strings.Contains(baseLower, queryLower)

		score := 0
		if hasFullMatch {
			score += fullMatch.Score * fullMatchWeight
		}
		if hasBaseMatch {
			score += baseMatch.Score * baseMatchWeight
		}
		if pathPrefix {
			score += pathPrefixBonus
		}
		if pathContains {
			score += pathContainsBonus
		}
		if pathHint {
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
			if basePrefix {
				score += basePrefixBonus
			}
			if baseContains {
				score += baseContainsBonus
			}
		}

		depthPenalty := depthPenaltyDefault
		if pathHint {
			depthPenalty = depthPenaltyPathHint
		}
		score -= strings.Count(path, "/") * depthPenalty
		score -= ansi.StringWidth(path)

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
			return b.score - a.score
		}
		return strings.Compare(a.item.Text(), b.item.Text())
	})

	result := make([]*CompletionItem, 0, len(ranked))
	for _, item := range ranked {
		result = append(result, item.item)
	}
	return result
}

func matchIndex(query string, values []string) map[int]fuzzy.Match {
	source := stringSource(values)
	matches := fuzzy.FindFrom(query, source)
	result := make(map[int]fuzzy.Match, len(matches))
	for _, match := range matches {
		result[match.Index] = match
	}
	return result
}

type stringSource []string

func (s stringSource) Len() int {
	return len(s)
}

func (s stringSource) String(i int) string {
	return s[i]
}

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

func hasPathHint(query string) bool {
	if strings.Contains(query, "/") || strings.Contains(query, "\\") {
		return true
	}

	lastDot := strings.LastIndex(query, ".")
	if lastDot < 0 || lastDot == len(query)-1 {
		return false
	}

	suffix := query[lastDot+1:]
	if len(suffix) > 12 {
		return false
	}

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

func namePriorityTier(path, query string) int {
	queryLower := strings.ToLower(strings.TrimSpace(query))
	if queryLower == "" {
		return tierFallback
	}

	normalized := strings.ToLower(strings.ReplaceAll(path, `\`, `/`))
	base := strings.ToLower(pathBase(normalized))
	stem := strings.TrimSuffix(base, filepath.Ext(base))

	if base == queryLower || stem == queryLower {
		return tierExactName
	}
	if strings.HasPrefix(base, queryLower) {
		return tierPrefixName
	}
	if hasPathSegment(normalized, queryLower) {
		return tierPathSegment
	}
	return tierFallback
}

func hasPathSegment(pathLower, queryLower string) bool {
	if queryLower == "" {
		return false
	}
	for _, part := range strings.Split(pathLower, "/") {
		if part == queryLower {
			return true
		}
	}
	return false
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
