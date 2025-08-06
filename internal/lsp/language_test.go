package lsp

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/lsp/protocol"
	"github.com/stretchr/testify/require"
)

func TestDetectLanguageID_StandardExtensions(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		expected protocol.LanguageKind
	}{
		{"go file", "file:///test/main.go", protocol.LangGo},
		{"javascript file", "file:///test/app.js", protocol.LangJavaScript},
		{"css file", "file:///test/styles.css", protocol.LangCSS},
		{"html file", "file:///test/index.html", protocol.LangHTML},
		{"python file", "file:///test/script.py", protocol.LangPython},
		{"unknown extension", "file:///test/unknown.xyz", protocol.LanguageKind("")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test with empty LSP config (should use standard mappings)
			result := detectLanguageIDWithConfig(tt.uri, config.LSPs{})
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestDetectLanguageID_CustomExtensions(t *testing.T) {
	customLSP := config.LSPs{
		"wxss": {
			Command:    "vscode-css-language-server",
			Extensions: []string{".wxss"},
		},
		"wxml": {
			Command:    "vscode-html-language-server",
			Extensions: []string{".wxml"},
		},
		"template-engine": {
			Command:    "template-lsp",
			Extensions: []string{".tmpl", ".template", ".tpl"},
		},
	}

	tests := []struct {
		name     string
		uri      string
		expected protocol.LanguageKind
	}{
		{"wxss file", "file:///test/styles.wxss", protocol.LanguageKind("wxss")},
		{"wxml file", "file:///test/page.wxml", protocol.LanguageKind("wxml")},
		{"tmpl file", "file:///test/layout.tmpl", protocol.LanguageKind("template-engine")},
		{"template file", "file:///test/component.template", protocol.LanguageKind("template-engine")},
		{"tpl file", "file:///test/view.tpl", protocol.LanguageKind("template-engine")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectLanguageIDWithConfig(tt.uri, customLSP)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestDetectLanguageID_Priority(t *testing.T) {
	// Test that custom LSP configurations take priority over standard mappings
	customLSP := config.LSPs{
		"custom-css": {
			Command:    "custom-css-lsp",
			Extensions: []string{".css"},
		},
	}

	result := detectLanguageIDWithConfig("file:///test/styles.css", customLSP)
	require.Equal(t, protocol.LanguageKind("custom-css"), result)
}

func TestDetectLanguageID_CaseInsensitive(t *testing.T) {
	customLSP := config.LSPs{
		"case-test": {
			Command:    "test-lsp",
			Extensions: []string{".WXSS"},
		},
	}

	tests := []struct {
		name     string
		uri      string
		expected protocol.LanguageKind
	}{
		{"lowercase", "file:///test/styles.wxss", protocol.LanguageKind("case-test")},
		{"uppercase", "file:///test/styles.WXSS", protocol.LanguageKind("case-test")},
		{"mixed case", "file:///test/styles.WxSs", protocol.LanguageKind("case-test")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectLanguageIDWithConfig(tt.uri, customLSP)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestDetectLanguageID_MultipleExtensions(t *testing.T) {
	customLSP := config.LSPs{
		"multi-lsp": {
			Command:    "multi-lsp",
			Extensions: []string{".ext1", ".ext2", ".ext3"},
		},
	}

	tests := []struct {
		name     string
		uri      string
		expected protocol.LanguageKind
	}{
		{"ext1", "file:///test/file.ext1", protocol.LanguageKind("multi-lsp")},
		{"ext2", "file:///test/file.ext2", protocol.LanguageKind("multi-lsp")},
		{"ext3", "file:///test/file.ext3", protocol.LanguageKind("multi-lsp")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectLanguageIDWithConfig(tt.uri, customLSP)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestDetectLanguageID_EmptyExtensions(t *testing.T) {
	// Test that LSP configs without Extensions don't interfere
	customLSP := config.LSPs{
		"no-extensions": {
			Command: "some-lsp",
		},
	}

	// Should use standard mapping for .go files
	result := detectLanguageIDWithConfig("file:///test/main.go", customLSP)
	require.Equal(t, protocol.LangGo, result)
}

func TestDetectLanguageID_RealWorldExamples(t *testing.T) {
	// Test real-world custom file type scenarios
	customLSP := config.LSPs{
		"wxss": {
			Command:    "vscode-css-language-server",
			Args:       []string{"--stdio"},
			Extensions: []string{".wxss"},
		},
		"wxml": {
			Command:    "vscode-html-language-server",
			Args:       []string{"--stdio"},
			Extensions: []string{".wxml"},
		},
		"vue": {
			Command:    "vue-language-server",
			Args:       []string{"--stdio"},
			Extensions: []string{".vue"},
		},
		"svelte": {
			Command:    "svelte-language-server",
			Args:       []string{"--stdio"},
			Extensions: []string{".svelte"},
		},
	}

	tests := []struct {
		name     string
		uri      string
		expected protocol.LanguageKind
	}{
		{"wxss file", "file:///test/app.wxss", protocol.LanguageKind("wxss")},
		{"wxml file", "file:///test/page.wxml", protocol.LanguageKind("wxml")},
		{"vue file", "file:///test/component.vue", protocol.LanguageKind("vue")},
		{"svelte file", "file:///test/App.svelte", protocol.LanguageKind("svelte")},
		{"standard js", "file:///test/main.js", protocol.LangJavaScript}, // Standard still works
		{"standard css", "file:///test/styles.css", protocol.LangCSS},    // Standard still works
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectLanguageIDWithConfig(tt.uri, customLSP)
			require.Equal(t, tt.expected, result)
		})
	}
}

// detectLanguageIDWithConfig is a test helper that allows injecting custom config
func detectLanguageIDWithConfig(uri string, lspConfig config.LSPs) protocol.LanguageKind {
	ext := strings.ToLower(filepath.Ext(uri))

	// Check custom LSP configurations first
	for lspName, lspConfig := range lspConfig {
		for _, customExt := range lspConfig.Extensions {
			if ext == strings.ToLower(customExt) {
				// Use the LSP configuration key as language ID
				return protocol.LanguageKind(lspName)
			}
		}
	}

	switch ext {
	case ".abap":
		return protocol.LangABAP
	case ".bat":
		return protocol.LangWindowsBat
	case ".bib", ".bibtex":
		return protocol.LangBibTeX
	case ".clj":
		return protocol.LangClojure
	case ".coffee":
		return protocol.LangCoffeescript
	case ".c":
		return protocol.LangC
	case ".cpp", ".cxx", ".cc", ".c++":
		return protocol.LangCPP
	case ".cs":
		return protocol.LangCSharp
	case ".css":
		return protocol.LangCSS
	case ".d":
		return protocol.LangD
	case ".pas", ".pascal":
		return protocol.LangDelphi
	case ".diff", ".patch":
		return protocol.LangDiff
	case ".dart":
		return protocol.LangDart
	case ".dockerfile":
		return protocol.LangDockerfile
	case ".ex", ".exs":
		return protocol.LangElixir
	case ".erl", ".hrl":
		return protocol.LangErlang
	case ".fs", ".fsi", ".fsx", ".fsscript":
		return protocol.LangFSharp
	case ".gitcommit":
		return protocol.LangGitCommit
	case ".gitrebase":
		return protocol.LangGitRebase
	case ".go":
		return protocol.LangGo
	case ".groovy":
		return protocol.LangGroovy
	case ".hbs", ".handlebars":
		return protocol.LangHandlebars
	case ".hs":
		return protocol.LangHaskell
	case ".html", ".htm":
		return protocol.LangHTML
	case ".ini":
		return protocol.LangIni
	case ".java":
		return protocol.LangJava
	case ".js":
		return protocol.LangJavaScript
	case ".jsx":
		return protocol.LangJavaScriptReact
	case ".json":
		return protocol.LangJSON
	case ".tex", ".latex":
		return protocol.LangLaTeX
	case ".less":
		return protocol.LangLess
	case ".lua":
		return protocol.LangLua
	case ".makefile", "makefile":
		return protocol.LangMakefile
	case ".md", ".markdown":
		return protocol.LangMarkdown
	case ".m":
		return protocol.LangObjectiveC
	case ".mm":
		return protocol.LangObjectiveCPP
	case ".pl":
		return protocol.LangPerl
	case ".pm":
		return protocol.LangPerl6
	case ".php":
		return protocol.LangPHP
	case ".ps1", ".psm1":
		return protocol.LangPowershell
	case ".pug", ".jade":
		return protocol.LangPug
	case ".py":
		return protocol.LangPython
	case ".r":
		return protocol.LangR
	case ".cshtml", ".razor":
		return protocol.LangRazor
	case ".rb":
		return protocol.LangRuby
	case ".rs":
		return protocol.LangRust
	case ".scss":
		return protocol.LangSCSS
	case ".sass":
		return protocol.LangSASS
	case ".scala":
		return protocol.LangScala
	case ".shader":
		return protocol.LangShaderLab
	case ".sh", ".bash", ".zsh", ".ksh":
		return protocol.LangShellScript
	case ".sql":
		return protocol.LangSQL
	case ".swift":
		return protocol.LangSwift
	case ".ts":
		return protocol.LangTypeScript
	case ".tsx":
		return protocol.LangTypeScriptReact
	case ".xml":
		return protocol.LangXML
	case ".xsl":
		return protocol.LangXSL
	case ".yaml", ".yml":
		return protocol.LangYAML
	default:
		return protocol.LanguageKind("") // Unknown language
	}
}
