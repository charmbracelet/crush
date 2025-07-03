package autolsp

// ServerName represents a language server name.
type ServerName string

const (
	ServerBashLanguageServer       ServerName = "bash-language-server"
	ServerClangd                   ServerName = "clangd"
	ServerDart                     ServerName = "dart"
	ServerDeno                     ServerName = "deno"
	ServerDockerLangserver         ServerName = "docker-langserver"
	ServerElixirLS                 ServerName = "elixir-ls"
	ServerGopls                    ServerName = "gopls"
	ServerIntelephense             ServerName = "intelephense"
	ServerJDTLS                    ServerName = "jdtls"
	ServerJediLanguageServer       ServerName = "jedi-language-server"
	ServerLuaLanguageServer        ServerName = "lua-language-server"
	ServerOmnisharp                ServerName = "omnisharp"
	ServerPylsp                    ServerName = "pylsp"
	ServerPyright                  ServerName = "pyright"
	ServerRustAnalyzer             ServerName = "rust-analyzer"
	ServerSolargraph               ServerName = "solargraph"
	ServerSvelteserver             ServerName = "svelteserver"
	ServerTypeScriptLanguageServer ServerName = "typescript-language-server"
	ServerVSCodeCSSLanguageServer  ServerName = "vscode-css-language-server"
	ServerVSCodeHTMLLanguageServer ServerName = "vscode-html-language-server"
	ServerVSCodeJSONLanguageServer ServerName = "vscode-json-language-server"
	ServerVLS                      ServerName = "vls"
	ServerYAMLLanguageServer       ServerName = "yaml-language-server"
)

// Server represents a language server with its configuration.
type Server struct {
	Name           ServerName
	Args           []string
	InstallCmd     string
	InstallWebsite string
	Langs          []LangName
	Priority       int // lower number means higher priority
}

// Servers contains all supported language servers with their configurations.
var Servers = []Server{
	{
		Name:           ServerBashLanguageServer,
		Args:           []string{"start"},
		InstallCmd:     "npm install -g bash-language-server",
		InstallWebsite: "",
		Langs:          []LangName{LangBash},
		Priority:       1,
	},
	{
		Name:           ServerClangd,
		InstallCmd:     "",
		InstallWebsite: "https://clangd.llvm.org/installation.html",
		Langs:          []LangName{LangC},
		Priority:       1,
	},
	{
		Name:           ServerDart,
		Args:           []string{"language-server"},
		InstallCmd:     "",
		InstallWebsite: "https://dart.dev/get-dart",
		Langs:          []LangName{LangDart},
		Priority:       1,
	},
	{
		Name:           ServerDeno,
		Args:           []string{"lsp"},
		InstallCmd:     "",
		InstallWebsite: "https://deno.com/#installation",
		Langs:          []LangName{LangJavaScript, LangTypeScript},
		Priority:       2,
	},
	{
		Name:           ServerDockerLangserver,
		Args:           []string{"--stdio"},
		InstallCmd:     "npm install -g dockerfile-language-server-nodejs",
		InstallWebsite: "",
		Langs:          []LangName{LangDocker},
		Priority:       1,
	},
	{
		Name:           ServerElixirLS,
		InstallCmd:     "",
		InstallWebsite: "https://github.com/elixir-lsp/elixir-ls#installation",
		Langs:          []LangName{LangElixir},
		Priority:       1,
	},
	{
		Name:           ServerGopls,
		InstallCmd:     "go install golang.org/x/tools/gopls@latest",
		InstallWebsite: "",
		Langs:          []LangName{LangGo},
		Priority:       1,
	},
	{
		Name:           ServerIntelephense,
		Args:           []string{"--stdio"},
		InstallCmd:     "npm install -g intelephense",
		InstallWebsite: "",
		Langs:          []LangName{LangPHP},
		Priority:       1,
	},
	{
		Name:           ServerJDTLS,
		InstallCmd:     "",
		InstallWebsite: "https://github.com/eclipse/eclipse.jdt.ls",
		Langs:          []LangName{LangJava},
		Priority:       1,
	},
	{
		Name:           ServerJediLanguageServer,
		InstallCmd:     "pip install jedi-language-server",
		InstallWebsite: "",
		Langs:          []LangName{LangPython},
		Priority:       1,
	},
	{
		Name:           ServerLuaLanguageServer,
		InstallCmd:     "",
		InstallWebsite: "https://github.com/LuaLS/lua-language-server/wiki/Getting-Started",
		Langs:          []LangName{LangLua},
		Priority:       1,
	},
	{
		Name:           ServerOmnisharp,
		Args:           []string{"--languageserver"},
		InstallCmd:     "npm install -g omnisharp-language-server",
		InstallWebsite: "",
		Langs:          []LangName{LangCSharp},
		Priority:       1,
	},
	{
		Name:           ServerPylsp,
		Args:           []string{},
		InstallCmd:     "pip install python-lsp-server",
		InstallWebsite: "",
		Langs:          []LangName{LangPython},
		Priority:       2,
	},
	{
		Name:           ServerPyright,
		Args:           []string{"--stdio"},
		InstallCmd:     "npm install -g pyright",
		InstallWebsite: "",
		Langs:          []LangName{LangPython},
		Priority:       3,
	},
	{
		Name:           ServerRustAnalyzer,
		Args:           []string{},
		InstallCmd:     "rustup component add rust-analyzer",
		InstallWebsite: "",
		Langs:          []LangName{LangRust},
		Priority:       1,
	},
	{
		Name:           ServerSolargraph,
		Args:           []string{"stdio"},
		InstallCmd:     "gem install solargraph",
		InstallWebsite: "",
		Langs:          []LangName{LangRuby},
		Priority:       1,
	},
	{
		Name:           ServerSvelteserver,
		Args:           []string{"--stdio"},
		InstallCmd:     "npm install -g svelte-language-server",
		InstallWebsite: "",
		Langs:          []LangName{LangSvelte},
		Priority:       1,
	},
	{
		Name:           ServerTypeScriptLanguageServer,
		Args:           []string{"--stdio"},
		InstallCmd:     "npm install -g typescript-language-server typescript",
		InstallWebsite: "",
		Langs:          []LangName{LangTypeScript},
		Priority:       1,
	},
	{
		Name:           ServerVSCodeCSSLanguageServer,
		Args:           []string{"--stdio"},
		InstallCmd:     "npm install -g vscode-langservers-extracted",
		InstallWebsite: "",
		Langs:          []LangName{LangCSS},
		Priority:       1,
	},
	{
		Name:           ServerVSCodeHTMLLanguageServer,
		Args:           []string{"--stdio"},
		InstallCmd:     "npm install -g vscode-langservers-extracted",
		InstallWebsite: "",
		Langs:          []LangName{LangHTML},
		Priority:       1,
	},
	{
		Name:           ServerVSCodeJSONLanguageServer,
		Args:           []string{"--stdio"},
		InstallCmd:     "npm install -g vscode-langservers-extracted",
		InstallWebsite: "",
		Langs:          []LangName{LangJSON},
		Priority:       1,
	},
	{
		Name:           ServerVLS,
		Args:           []string{},
		InstallCmd:     "npm install -g @volar/vue-language-server",
		InstallWebsite: "",
		Langs:          []LangName{LangVue},
		Priority:       1,
	},
	{
		Name:           ServerYAMLLanguageServer,
		Args:           []string{"--stdio"},
		InstallCmd:     "npm install -g yaml-language-server",
		InstallWebsite: "",
		Langs:          []LangName{LangYAML},
		Priority:       1,
	},
}
