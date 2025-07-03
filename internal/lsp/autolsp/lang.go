package autolsp

// LangName represents a programming language name.
type LangName string

const (
	LangBash       LangName = "bash"
	LangC          LangName = "c"
	LangCSS        LangName = "css"
	LangCSharp     LangName = "csharp"
	LangDart       LangName = "dart"
	LangDocker     LangName = "docker"
	LangElixir     LangName = "elixir"
	LangGo         LangName = "go"
	LangHTML       LangName = "html"
	LangJSON       LangName = "json"
	LangJava       LangName = "java"
	LangJavaScript LangName = "javascript"
	LangLua        LangName = "lua"
	LangPHP        LangName = "php"
	LangPython     LangName = "python"
	LangRuby       LangName = "ruby"
	LangRust       LangName = "rust"
	LangSvelte     LangName = "svelte"
	LangTypeScript LangName = "typescript"
	LangVue        LangName = "vue"
	LangYAML       LangName = "yaml"
)

// Lang represents a programming language with its detection patterns.
type Lang struct {
	Name              LangName
	WorkspacePatterns []string
	FilePatterns      []string
}

// Langs contains all supported languages with their detection patterns.
var Langs = []Lang{
	{
		Name:              LangBash,
		WorkspacePatterns: nil,
		FilePatterns:      []string{"*.sh", "*.bash"},
	},
	{
		Name:              LangC,
		WorkspacePatterns: nil,
		FilePatterns:      []string{"*.c"},
	},
	{
		Name:              LangCSS,
		WorkspacePatterns: nil,
		FilePatterns:      []string{"*.css"},
	},
	{
		Name:              LangCSharp,
		WorkspacePatterns: nil,
		FilePatterns:      []string{"*.cs"},
	},
	{
		Name:              LangDart,
		WorkspacePatterns: []string{"pubspec.yaml"},
		FilePatterns:      []string{"*.dart"},
	},
	{
		Name:              LangDocker,
		WorkspacePatterns: nil,
		FilePatterns:      []string{"Dockerfile"},
	},
	{
		Name:              LangElixir,
		WorkspacePatterns: []string{"mix.exs"},
		FilePatterns:      []string{"*.ex", "*.exs"},
	},
	{
		Name:              LangGo,
		WorkspacePatterns: []string{"go.mod"},
		FilePatterns:      []string{"*.go"},
	},
	{
		Name:              LangHTML,
		WorkspacePatterns: nil,
		FilePatterns:      []string{"*.html", "*.htm"},
	},
	{
		Name:              LangJSON,
		WorkspacePatterns: nil,
		FilePatterns:      []string{"*.json"},
	},
	{
		Name:              LangJava,
		WorkspacePatterns: []string{"build.gradle", "pom.xml"},
		FilePatterns:      []string{"*.java"},
	},
	{
		Name:              LangJavaScript,
		WorkspacePatterns: []string{"package.json"},
		FilePatterns:      []string{"*.js", "*.jsx", "*.mjs", "*.cjs"},
	},
	{
		Name:              LangLua,
		WorkspacePatterns: nil,
		FilePatterns:      []string{"*.lua"},
	},
	{
		Name:              LangPHP,
		WorkspacePatterns: []string{"composer.json"},
		FilePatterns:      []string{"*.php"},
	},
	{
		Name:              LangPython,
		WorkspacePatterns: []string{"pyproject.toml", "requirements.txt", "setup.py"},
		FilePatterns:      []string{"*.py"},
	},
	{
		Name:              LangRuby,
		WorkspacePatterns: []string{"Gemfile", "*.gemspec"},
		FilePatterns:      []string{"*.rb", "*.rake", "*.gemspec"},
	},
	{
		Name:              LangRust,
		WorkspacePatterns: []string{"Cargo.toml"},
		FilePatterns:      []string{"*.rs"},
	},
	{
		Name:              LangSvelte,
		WorkspacePatterns: nil,
		FilePatterns:      []string{"*.svelte"},
	},
	{
		Name:              LangTypeScript,
		WorkspacePatterns: nil,
		FilePatterns:      []string{"*.ts", "*.tsx"},
	},
	{
		Name:              LangVue,
		WorkspacePatterns: nil,
		FilePatterns:      []string{"*.vue"},
	},
	{
		Name:              LangYAML,
		WorkspacePatterns: nil,
		FilePatterns:      []string{"*.yaml", "*.yml"},
	},
}
