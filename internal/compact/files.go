package compact

import (
	"regexp"
	"slices"
	"strings"
)

// Crush tool name sets for file operations.
var (
	fileWriteTools  = map[string]bool{"edit": true, "write": true, "multiedit": true}
	fileReadTools   = map[string]bool{"view": true}
	fileCreateTools = map[string]bool{"write": true}
)

// Declaration regexes for multiple languages.
var (
	goDeclRe     = regexp.MustCompile(`^\s*func\s+(?:\(\w+\s+\*?\w+\)\s+)?(\w+)`)
	goSigRe      = regexp.MustCompile(`^\s*func\s+(?:\(\w+\s+\*?\w+\)\s+)?\w+\s*(?:\([^)]*\))?\s*(?:\([^)]*\))?`)
	tsDeclRe     = regexp.MustCompile(`^\s*export\s+(?:default\s+)?(?:async\s+)?(?:function|class|type|interface|const|let|enum)\s+(\w+)`)
	tsSigRe      = regexp.MustCompile(`^\s*export\s+(?:default\s+)?(?:async\s+)?(?:function|class|type|interface|const|let|enum)\s+\w+[^;{]*[;{]?`)
	pyDeclRe     = regexp.MustCompile(`^\s*(?:async\s+)?def\s+(\w+)|^\s*class\s+(\w+)`)
	pySigRe      = regexp.MustCompile(`^\s*(?:async\s+)?(?:def|class)\s+\w+\s*(?:\([^)]*\))?`)
	rustDeclRe   = regexp.MustCompile(`^\s*(?:pub(?:\s*\([^)]*\))?\s+)?(?:fn|struct|enum|trait|type|const)\s+(\w+)`)
	rustSigRe    = regexp.MustCompile(`^\s*pub\s+(?:async\s+)?(?:fn|struct|enum|trait|type)\s+\w+`)
	declScreenRe = regexp.MustCompile(
		`^\s*(?:export|pub|func|def|class|type|interface|async|abstract|static|public|private|protected|struct|enum|trait|impl|module|const|fn|sealed|record|typedef|union|virtual|extern|inline)`,
	)
)

const (
	maxSigsPerFile  = 8
	maxCatalogFiles = 12
	scanLines       = 200
)

type fileSigEntry struct {
	sigs     []string
	modified bool
}

type symbolInfo struct {
	name      string
	kind      string
	signature string
}

// ExtractFileAndSymbols performs a single-pass extraction of file activity,
// symbol names, and type catalog from tool call blocks.
func ExtractFileAndSymbols(blocks []NormalizedBlock) UnifiedExtractResult {
	readSet := make(map[string]bool)
	modSet := make(map[string]bool)
	createSet := make(map[string]bool)
	symbols := make(map[string][]string)
	symbolsSeen := make(map[string]map[string]bool)
	var symbolRefs []SymbolRef
	refSeen := make(map[string]bool)

	fileSigs := make(map[string]*fileSigEntry)
	var fileOrder []string

	for i, b := range blocks {
		if b.Kind != BlockToolCall {
			continue
		}
		path := extractPathFromArgs(b.Args)
		if path == "" {
			continue
		}

		isRead := fileReadTools[b.Name]
		isWrite := fileWriteTools[b.Name]
		isCreate := fileCreateTools[b.Name]

		if isRead {
			readSet[path] = true
		}
		if isWrite {
			modSet[path] = true
		}
		if isCreate {
			createSet[path] = true
		}

		// Extract symbols from write args.
		if isWrite {
			newText := b.Args["new_string"]
			if newText == "" {
				newText = b.Args["content"]
			}
			if newText != "" {
				syms := extractSymbols(newText, 100, true)
				addSymbols(symbols, symbolsSeen, path, syms)
				addTypeCatalog(fileSigs, &fileOrder, path, syms, true)
				addSymbolRefs(&symbolRefs, refSeen, path, syms, "modified")
			}
		}

		// Extract from the next tool result (look-ahead).
		if isRead || isWrite {
			result := findToolResult(blocks, i)
			if result != nil && !result.IsError {
				syms := extractSymbols(result.ResultText, scanLines, true)
				addSymbols(symbols, symbolsSeen, path, syms)
				if isRead {
					addTypeCatalog(fileSigs, &fileOrder, path, syms, false)
				}
				access := "read"
				if isWrite {
					access = "modified"
				}
				addSymbolRefs(&symbolRefs, refSeen, path, syms, access)
			}
		}
	}

	// Dedup: if already modified, drop from created.
	for p := range modSet {
		delete(createSet, p)
	}

	act := FileActivity{
		Read:     setToSlice(readSet),
		Modified: setToSlice(modSet),
		Created:  setToSlice(createSet),
		Symbols:  symbols,
	}

	var modifiedSigs, readSigs []ExportSig
	for _, f := range fileOrder {
		entry := fileSigs[f]
		if len(entry.sigs) == 0 {
			continue
		}
		sigs := entry.sigs
		if len(sigs) > maxSigsPerFile {
			sigs = sigs[:maxSigsPerFile]
		}
		es := ExportSig{File: f, Signatures: sigs, Modified: entry.modified}
		if entry.modified {
			modifiedSigs = append(modifiedSigs, es)
		} else {
			readSigs = append(readSigs, es)
		}
	}
	catalog := append(modifiedSigs, readSigs...)
	if len(catalog) > maxCatalogFiles {
		catalog = catalog[:maxCatalogFiles]
	}

	return UnifiedExtractResult{
		FileActivity:  act,
		TypeCatalog:   catalog,
		SymbolChanges: symbolRefs,
	}
}

func findToolResult(blocks []NormalizedBlock, callIdx int) *NormalizedBlock {
	for j := callIdx + 1; j < len(blocks) && j < callIdx+4; j++ {
		if blocks[j].Kind == BlockToolResult || blocks[j].Kind == BlockBash {
			return &blocks[j]
		}
	}
	return nil
}

func extractSymbols(text string, maxLines int, includeSigs bool) []symbolInfo {
	lines := strings.SplitN(text, "\n", maxLines+1)
	var syms []symbolInfo
	seen := make(map[string]bool)
	for _, line := range lines {
		decl := parseDeclName(line)
		if decl == nil || seen[decl.name] {
			continue
		}
		seen[decl.name] = true
		if includeSigs {
			decl.signature = parseSignature(line)
		}
		syms = append(syms, *decl)
	}
	return syms
}

func parseDeclName(line string) *symbolInfo {
	if !declScreenRe.MatchString(line) {
		return nil
	}
	if m := goDeclRe.FindStringSubmatch(line); m != nil {
		name := m[1]
		if len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' {
			return &symbolInfo{name: name, kind: "function"}
		}
	}
	if m := tsDeclRe.FindStringSubmatch(line); m != nil {
		kind := "variable"
		if strings.Contains(line, "function") {
			kind = "function"
		} else if strings.Contains(line, "class") {
			kind = "class"
		} else if strings.Contains(line, "type") || strings.Contains(line, "interface") {
			kind = "type"
		}
		return &symbolInfo{name: m[1], kind: kind}
	}
	if m := pyDeclRe.FindStringSubmatch(line); m != nil {
		if m[2] != "" {
			return &symbolInfo{name: m[2], kind: "class"}
		}
		return &symbolInfo{name: m[1], kind: "function"}
	}
	if m := rustDeclRe.FindStringSubmatch(line); m != nil {
		return &symbolInfo{name: m[1], kind: "function"}
	}
	return nil
}

func parseSignature(line string) string {
	trimmed := strings.TrimSpace(line)
	if goSigRe.MatchString(trimmed) {
		m := goDeclRe.FindStringSubmatch(trimmed)
		if m != nil && len(m[1]) > 0 && m[1][0] >= 'A' && m[1][0] <= 'Z' {
			return trimmed
		}
	}
	if tsSigRe.MatchString(trimmed) {
		return trimmed
	}
	if pySigRe.MatchString(trimmed) {
		t := strings.TrimSpace(trimmed)
		if !strings.HasPrefix(t, "def _") && !strings.HasPrefix(t, "class _") {
			return t
		}
	}
	if rustSigRe.MatchString(trimmed) {
		return trimmed
	}
	return ""
}

func addSymbols(symbols map[string][]string, seen map[string]map[string]bool, path string, syms []symbolInfo) {
	if seen[path] == nil {
		seen[path] = make(map[string]bool)
	}
	for _, s := range syms {
		if !seen[path][s.name] {
			seen[path][s.name] = true
			symbols[path] = append(symbols[path], s.name)
		}
	}
}

func addTypeCatalog(fileSigs map[string]*fileSigEntry, fileOrder *[]string, path string, syms []symbolInfo, modified bool) {
	entry, exists := fileSigs[path]
	if !exists {
		entry = &fileSigEntry{modified: modified}
		fileSigs[path] = entry
		*fileOrder = append(*fileOrder, path)
	} else if modified {
		entry.modified = true
	}
	for _, s := range syms {
		if s.signature != "" {
			found := slices.Contains(entry.sigs, s.signature)
			if !found {
				entry.sigs = append(entry.sigs, s.signature)
			}
		}
	}
}

func addSymbolRefs(refs *[]SymbolRef, seen map[string]bool, path string, syms []symbolInfo, access string) {
	for _, s := range syms {
		key := s.name + "@" + path
		if !seen[key] {
			seen[key] = true
			*refs = append(*refs, SymbolRef{
				Name: s.name, File: path, Kind: s.kind, Access: access,
			})
		}
	}
}

func setToSlice(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
