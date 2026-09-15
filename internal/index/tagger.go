package index

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// maxIndexFileSize caps the bytes read per file for tagging. Files
// beyond it are recorded in the files table but not tagged — the map
// shows them, their contents stay untracked. It also bounds the
// scanner's per-line buffer. Same order of magnitude as the prompt
// context-file bound.
const maxIndexFileSize = 256 * 1024

// tag is one extracted top-level declaration.
type tag struct {
	name     string
	kind     string
	line     int
	exported bool
}

// declRule maps a line to a declaration. The regex must capture the
// declared name in group 1.
type declRule struct {
	re   *regexp.Regexp
	kind string
}

// langSpec is a declaration-only grammar: enough to route the model
// to the right file, deliberately not a parser.
type langSpec struct {
	rules   []declRule
	export  func(name, line string) bool
	imports []*regexp.Regexp
	// importsInBlock, when true, applies the import regexes only inside
	// an `import ( ... )` block or on an `import` line — prevents bare
	// string literals elsewhere from producing candidate refs (Go).
	importsInBlock bool
	// resolve maps an import specifier to a project-relative file or
	// directory path. It returns "" when the specifier can't be
	// resolved inside the project (external deps, stdlib).
	resolve func(imp, srcDir, modulePath string, exists func(string) bool) string
}

func goExport(name, _ string) bool {
	r, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(r)
}

func pyExport(name, _ string) bool {
	return !strings.HasPrefix(name, "_")
}

func lineHas(marker string) func(string, string) bool {
	// Word-boundary match — `publish`/`reexport` aren't `pub`/`export`.
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(marker) + `\b`)
	return func(_, line string) bool {
		return re.MatchString(line)
	}
}

func anyIdent(name, _ string) bool { return true }

var langByExt = map[string]langSpec{
	".go": {
		rules: []declRule{
			{regexp.MustCompile(`^func\s+(?:\([^)]*\)\s*)?(\w+)`), "func"},
			{regexp.MustCompile(`^type\s+(\w+)`), "type"},
			{regexp.MustCompile(`^(?:const|var)\s+(\w+)`), "var"},
		},
		export: goExport,
		imports: []*regexp.Regexp{
			regexp.MustCompile(`^\s*(?:\w+\s+)?"([^"]+)"`),
			regexp.MustCompile(`^import\s+\(?\s*"([^"]+)"`),
		},
		importsInBlock: true,
		resolve:        resolveGoImport,
	},
	".py": {
		rules: []declRule{
			{regexp.MustCompile(`^(?:async\s+)?def\s+(\w+)`), "func"},
			{regexp.MustCompile(`^class\s+(\w+)`), "class"},
			{regexp.MustCompile(`^\s+(?:async\s+)?def\s+(\w+)`), "method"},
		},
		export: pyExport,
		imports: []*regexp.Regexp{
			regexp.MustCompile(`^from\s+([\w.]+)\s+import`),
			regexp.MustCompile(`^import\s+([\w.]+)`),
		},
		resolve: resolvePyImport,
	},
	".rs": {
		rules: []declRule{
			{regexp.MustCompile(`^\s*(?:pub(?:\([^)]*\))?\s+)?(?:async\s+)?fn\s+(\w+)`), "func"},
			{regexp.MustCompile(`^\s*(?:pub(?:\([^)]*\))?\s+)?(?:struct|enum|trait|union)\s+(\w+)`), "type"},
			// `impl Trait for Type` must capture Type (the implementor),
			// not the trait — the `for` rule runs first.
			{regexp.MustCompile(`^\s*impl(?:<[^>]*>)?\s+(?:[\w:]+::)*\w+(?:<[^>]*>)?\s+for\s+(?:[\w:]+::)*(\w+)`), "impl"},
			{regexp.MustCompile(`^\s*impl(?:<[^>]*>)?\s+(?:[\w:]+::)*(\w+)`), "impl"},
			{regexp.MustCompile(`^\s*(?:pub\s+)?mod\s+(\w+)`), "mod"},
		},
		export: lineHas("pub"),
		imports: []*regexp.Regexp{
			regexp.MustCompile(`^\s*use\s+([\w:]+)`),
			regexp.MustCompile(`^\s*(?:pub\s+)?mod\s+(\w+)\s*;`),
		},
		resolve: resolveRsImport,
	},
	".ts": jsSpec, ".tsx": jsSpec, ".js": jsSpec, ".jsx": jsSpec,
	".mjs": jsSpec, ".cjs": jsSpec,
	".java": {
		rules: []declRule{
			{regexp.MustCompile(`^\s*(?:[\w@]+\s+)*?(?:class|interface|enum|record|@interface)\s+(\w+)`), "type"},
		},
		export: lineHas("public"),
		imports: []*regexp.Regexp{
			regexp.MustCompile(`^import\s+(?:static\s+)?([\w.]+)`),
		},
		resolve: resolveJavaImport,
	},
}

// jsSpec is factored out because six extensions share it.
var jsSpec = langSpec{
	rules: []declRule{
		{regexp.MustCompile(`^\s*(?:export\s+)?(?:default\s+)?(?:async\s+)?function\*?\s+(\w+)`), "func"},
		{regexp.MustCompile(`^\s*(?:export\s+)?(?:abstract\s+)?class\s+(\w+)`), "class"},
		{regexp.MustCompile(`^\s*(?:export\s+)?(?:interface|type|enum)\s+(\w+)`), "type"},
		{regexp.MustCompile(`^\s*(?:export\s+)?(?:const|let|var)\s+(\w+)`), "var"},
		// Method rules must end in `{` (with optional TS return type) or
		// every indented call site gets tagged as a definition. The
		// return-type class admits space and comma for e.g.
		// `Map<string, number>`.
		{regexp.MustCompile(`^\s+(?:(?:public|private|protected|static|async|readonly|override|abstract)\s+)*(?:get\s+|set\s+)?(\w+)\s*\([^)]*\)\s*(?::\s*[\w<>\[\]|&,\s]+\s*)?\{`), "method"},
	},
	export: lineHas("export"),
	imports: []*regexp.Regexp{
		regexp.MustCompile(`(?:from|import)\s+['"]([^'"]+)['"]`),
		regexp.MustCompile(`require\(\s*['"]([^'"]+)['"]\s*\)`),
	},
	resolve: resolveJSImport,
}

// jsKeywords keeps the method rule from matching control-flow lines.
var jsKeywords = map[string]bool{
	"if": true, "for": true, "while": true, "return": true,
	"switch": true, "catch": true, "function": true, "new": true,
	"constructor": true, "typeof": true, "await": true, "else": true,
	"do": true, "case": true, "throw": true, "yield": true,
}

// tagFile extracts declarations and resolved references from one file.
// exists reports whether a project-relative path is a known file or a
// directory containing known files; modulePath is the go.mod module
// path when present.
func tagFile(root, relPath, modulePath string, exists func(string) bool) ([]tag, []string, error) {
	spec, ok := langByExt[filepath.Ext(relPath)]
	if !ok {
		return nil, nil, nil
	}
	f, err := os.Open(filepath.Join(root, relPath))
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	// Oversized files are recorded in the files table but not tagged —
	// the map still shows them.
	if st, err := f.Stat(); err != nil || st.Size() > maxIndexFileSize {
		return nil, nil, nil
	}

	// Sniff the first block for NUL bytes — binary files get recorded
	// in the files table but never tagged.
	br := bufio.NewReader(f)
	if head, _ := br.Peek(512); isProbablyBinary(head) {
		return nil, nil, nil
	}

	var tags []tag
	refSet := map[string]bool{}
	srcDir := filepath.Dir(relPath)

	sc := bufio.NewScanner(br)
	sc.Buffer(make([]byte, 64*1024), maxIndexFileSize)
	lineNo := 0
	inImportBlock := false
	for sc.Scan() {
		lineNo++
		line := sc.Text()
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") ||
			strings.HasPrefix(trimmed, "*") {
			continue
		}
		if spec.importsInBlock {
			switch {
			case strings.HasPrefix(trimmed, "import (") || trimmed == "import(":
				// A `)` or quote on the opener line means a
				// single-line form (`import ("fmt")`, `import ()`)
				// — fall through so it's scanned below instead of
				// sticking the gate open.
				if strings.ContainsAny(line, `")`) {
					break
				}
				inImportBlock = true
				continue
			case inImportBlock && strings.HasPrefix(trimmed, ")"):
				inImportBlock = false
				continue
			}
		}
		for _, rule := range spec.rules {
			m := rule.re.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			name := m[1]
			if rule.kind == "method" && jsKeywords[name] {
				continue
			}
			tags = append(tags, tag{
				name:     name,
				kind:     rule.kind,
				line:     lineNo,
				exported: spec.export(name, line),
			})
			break
		}
		if !spec.importsInBlock || inImportBlock || strings.HasPrefix(trimmed, "import") {
			for _, re := range spec.imports {
				if m := re.FindStringSubmatch(line); m != nil {
					if dst := spec.resolve(m[1], srcDir, modulePath, exists); dst != "" {
						refSet[dst] = true
					}
					break
				}
			}
		}
	}
	if err := sc.Err(); err != nil {
		// Truncated/binary tail: keep the tags collected so far but
		// drop refs — a partial ref set would poison in-degree
		// ranking until the file is next re-tagged.
		return tags, nil, nil
	}
	refs := make([]string, 0, len(refSet))
	for r := range refSet {
		refs = append(refs, r)
	}
	return tags, refs, nil
}

// isProbablyBinary sniffs the first block for NUL bytes.
func isProbablyBinary(head []byte) bool {
	return bytes.IndexByte(head, 0) >= 0
}

// resolveGoImport maps a Go import path to the project-relative
// package directory when it lives under the module path.
func resolveGoImport(imp, _, modulePath string, exists func(string) bool) string {
	// The "/" boundary matters: `example.com/proj2/...` must not
	// strip to `2/...` under module `example.com/proj`.
	if modulePath == "" || !strings.HasPrefix(imp, modulePath+"/") {
		return ""
	}
	dir := strings.TrimPrefix(imp, modulePath+"/")
	// A package dir counts when indexed files sit under it — exists
	// covers directories as well as files.
	if exists(dir) {
		return dir
	}
	return ""
}

// probeExt returns the first candidate that exists, or "".
func probeExt(base string, exists func(string) bool, exts ...string) string {
	for _, ext := range exts {
		if exists(base + ext) {
			return base + ext
		}
	}
	return ""
}

func resolveJSImport(imp, srcDir, _ string, exists func(string) bool) string {
	if !strings.HasPrefix(imp, ".") {
		return "" // Package imports aren't project files.
	}
	base := filepath.Clean(filepath.Join(srcDir, imp))
	base = filepath.ToSlash(base)
	if exists(base) {
		return base
	}
	if p := probeExt(base, exists, ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"); p != "" {
		return p
	}
	return probeExt(base+"/index", exists, ".ts", ".tsx", ".js", ".jsx")
}

func resolvePyImport(imp, srcDir, _ string, exists func(string) bool) string {
	var bases []string
	if strings.HasPrefix(imp, ".") {
		// Relative import: leading dots walk up from the file's dir —
		// `from .sibling import x` inside pkg/a.py means pkg/sibling.py.
		dots := len(imp) - len(strings.TrimLeft(imp, "."))
		rest := strings.TrimLeft(imp, ".")
		dir := srcDir
		for range dots - 1 {
			dir = filepath.Dir(dir)
		}
		base := filepath.ToSlash(filepath.Join(dir, strings.ReplaceAll(rest, ".", "/")))
		bases = append(bases, base)
	} else {
		base := strings.ReplaceAll(imp, ".", "/")
		// Absolute imports resolve from root, or from src/ under the
		// common src-layout.
		bases = append(bases, base, "src/"+base)
	}
	for _, base := range bases {
		if p := probeExt(base, exists, ".py"); p != "" {
			return p
		}
		if exists(base + "/__init__.py") {
			return base + "/__init__.py"
		}
	}
	return ""
}

func resolveRsImport(imp, srcDir, _ string, exists func(string) bool) string {
	imp = strings.TrimPrefix(imp, "crate::")
	imp = strings.TrimPrefix(imp, "self::")
	parts := strings.Split(imp, "::")
	if len(parts) == 0 {
		return ""
	}
	last := parts[len(parts)-1]
	// The last `use` segment is usually an ITEM (a symbol), not a
	// file: `use crate::a::b::Item` lives in src/a/b.rs or
	// src/a/b/mod.rs. `mod foo;` declarations instead name the file
	// directly. Probe item-first, then the file shape.
	parent := filepath.ToSlash(filepath.Join(parts[:len(parts)-1]...))
	bases := []string{}
	if parent != "" {
		bases = append(bases,
			"src/"+parent, // src/a/b.rs or src/a/b/mod.rs
			"src/"+parent+"/"+last,
		)
	}
	bases = append(bases, filepath.ToSlash(filepath.Join(srcDir, last)))
	for _, base := range bases {
		if p := probeExt(base, exists, ".rs"); p != "" {
			return p
		}
		if exists(base + "/mod.rs") {
			return base + "/mod.rs"
		}
	}
	return ""
}

func resolveJavaImport(imp, _, _ string, exists func(string) bool) string {
	p := strings.ReplaceAll(imp, ".", "/") + ".java"
	if exists(p) {
		return p
	}
	// Maven/Gradle layout.
	if idx := strings.Index(p, "/"); idx >= 0 {
		for _, root := range []string{"src/main/java/", "src/test/java/"} {
			if exists(root + p) {
				return root + p
			}
		}
	}
	return ""
}
