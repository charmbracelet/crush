// Package symbols extracts code symbols (functions, methods, types) from
// source files using tree-sitter grammars via gotreesitter. It provides
// symbol-aware chunking boundaries for semantic search indexing.
package symbols

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// Symbol represents a named code entity extracted from a source file.
//
// StartLine/EndLine span the whole definition — signature through closing
// brace — not just the line the name sits on, because the point of these
// is to be chunk boundaries.
type Symbol struct {
	Name      string // qualified name (e.g. "FuncName")
	Kind      string // tag kind (e.g. "definition.function")
	StartLine int    // 0-based start line
	EndLine   int    // 0-based end line (inclusive)
}

// ExtractResult holds the symbols extracted from a single file, or an
// error/fallback indicator if extraction could not proceed.
type ExtractResult struct {
	Path     string
	Symbols  []Symbol
	Fallback bool   // true if fixed-size chunking should be used instead
	Reason   string // why fallback was triggered (empty if Symbols populated)
}

// grammarCacheLimit bounds both the embedded grammar cache and the
// per-language tagger cache below. They are deliberately the same number:
// a cached tagger pins the *Language it was built from, so an unbounded
// tagger cache would defeat the grammar limit it is paired with.
const grammarCacheLimit = 32

// langTools holds the per-language objects worth reusing across files.
// Building a Tagger compiles its tree-sitter tags query, which is by far
// the most expensive step here — doing it per file made query compilation
// the dominant cost of a walk.
type langTools struct {
	parser *gotreesitter.Parser
	tagger *gotreesitter.Tagger
}

// Extractor extracts symbols from source files using gotreesitter.
//
// Safe for concurrent use, but tagging is serialized: a Tagger carries a
// reusable match buffer and its own parser, so it cannot be shared across
// goroutines. WalkAndExtract drives extraction from a single goroutine, so
// this costs nothing there.
type Extractor struct {
	mu    sync.Mutex
	tools map[string]*langTools
}

// NewExtractor creates a symbol extractor. It configures the grammar
// cache limit to avoid retaining every grammar touched during large
// indexing runs.
func NewExtractor() *Extractor {
	grammars.SetEmbeddedLanguageCacheLimit(grammarCacheLimit)
	return &Extractor{tools: make(map[string]*langTools)}
}

// toolsFor returns the cached parser/tagger for a language, building them
// on first use. The caller must hold e.mu.
func (e *Extractor) toolsFor(entry *grammars.LangEntry, lang *gotreesitter.Language, tagsQuery string) (*langTools, error) {
	if t, ok := e.tools[entry.Name]; ok {
		return t, nil
	}
	tagger, err := gotreesitter.NewTagger(lang, tagsQuery)
	if err != nil {
		return nil, err
	}
	// Crude but bounded: a repository realistically touches far fewer than
	// grammarCacheLimit languages, so this is a backstop rather than a
	// working eviction policy.
	if len(e.tools) >= grammarCacheLimit {
		clear(e.tools)
	}
	t := &langTools{parser: gotreesitter.NewParser(lang), tagger: tagger}
	e.tools[entry.Name] = t
	return t, nil
}

// skipDirs are directory names that should never be descended into
// during source walks. This covers common vendor, generated, and
// build artifact directories across ecosystems.
var skipDirs = map[string]bool{
	".git":          true,
	".hg":           true,
	".svn":          true,
	"node_modules":  true,
	"vendor":        true,
	"__pycache__":   true,
	".tox":          true,
	".eggs":         true,
	".mypy_cache":   true,
	".pytest_cache": true,
	"dist":          true,
	"build":         true,
	".next":         true,
	".nuxt":         true,
	"target":        true,
	".crush":        true,
	".claude":       true,
}

// ExtractFile extracts symbols from a single source file. If the
// language is unsupported, the grammar quality is insufficient, or
// parsing produces errors, it returns a result with Fallback=true.
func (e *Extractor) ExtractFile(path string) ExtractResult {
	result := ExtractResult{Path: path}

	src, err := os.ReadFile(path)
	if err != nil {
		result.Fallback = true
		result.Reason = fmt.Sprintf("read error: %v", err)
		return result
	}

	entry := grammars.DetectLanguage(filepath.Base(path))
	if entry == nil {
		result.Fallback = true
		result.Reason = "unsupported language"
		return result
	}

	// Quality is only populated by AuditParseSupport(); an empty value
	// means the grammar loaded without audit and should be treated as
	// usable. Only reject explicitly degraded grammars.
	if entry.Quality == grammars.ParseQualityPartial || entry.Quality == grammars.ParseQualityNone {
		result.Fallback = true
		result.Reason = fmt.Sprintf("grammar quality %q", entry.Quality)
		return result
	}

	tagsQuery := grammars.ResolveTagsQuery(*entry)
	if tagsQuery == "" {
		result.Fallback = true
		result.Reason = "no tags query available"
		return result
	}

	lang := entry.Language()
	if lang == nil {
		result.Fallback = true
		result.Reason = "failed to load grammar"
		return result
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	tools, err := e.toolsFor(entry, lang, tagsQuery)
	if err != nil {
		result.Fallback = true
		result.Reason = fmt.Sprintf("tagger init: %v", err)
		return result
	}

	// Parse once. Tagger.Tag would parse the source a second time, so tag
	// the tree we already have instead.
	tree, err := tools.parser.Parse(src)
	if err != nil || tree == nil || tree.RootNode() == nil {
		result.Fallback = true
		if err != nil {
			result.Reason = fmt.Sprintf("parse error: %v", err)
		} else {
			result.Reason = "parse returned nil tree"
		}
		return result
	}
	defer tree.Release()

	if tree.RootNode().HasError() {
		result.Fallback = true
		result.Reason = "parse tree contains errors"
		return result
	}

	for _, tag := range tools.tagger.TagTree(tree) {
		if !isDefinition(tag.Kind) {
			continue
		}
		// Range is the whole definition; NameRange is only the identifier,
		// which would collapse every symbol to a single line and make these
		// useless as chunk boundaries.
		result.Symbols = append(result.Symbols, Symbol{
			Name:      tag.Name,
			Kind:      tag.Kind,
			StartLine: int(tag.Range.StartPoint.Row),
			EndLine:   int(tag.Range.EndPoint.Row),
		})
	}

	return result
}

// WalkAndExtract walks a directory tree, extracting symbols from every
// recognized source file. It skips vendor/build directories and hidden
// directories. Results are sent on the returned channel; the channel is
// closed when the walk completes or ctx is cancelled.
func (e *Extractor) WalkAndExtract(ctx context.Context, root string) <-chan ExtractResult {
	ch := make(chan ExtractResult, 64)
	go func() {
		defer close(ch)
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}

			if ctx.Err() != nil {
				return ctx.Err()
			}

			name := d.Name()

			if d.IsDir() {
				// Never skip the root itself. Indexing a directory that
				// happens to be hidden or named "build" is a legitimate
				// request, and skipping it would silently walk nothing.
				if path == root {
					return nil
				}
				if skipDirs[name] {
					return filepath.SkipDir
				}
				if strings.HasPrefix(name, ".") {
					return filepath.SkipDir
				}
				return nil
			}

			entry := grammars.DetectLanguage(name)
			if entry == nil {
				return nil
			}

			select {
			case ch <- e.ExtractFile(path):
			case <-ctx.Done():
				return ctx.Err()
			}

			return nil
		})
		// Cancellation is how callers stop a walk, not a failure.
		if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			slog.Error("Symbol walk failed", "root", root, "error", err)
		}
	}()
	return ch
}

// isDefinition reports whether a tag kind represents a definition
// rather than a reference.
func isDefinition(kind string) bool {
	return strings.HasPrefix(kind, "definition.")
}
