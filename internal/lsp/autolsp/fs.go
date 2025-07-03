package autolsp

// dirsToIgnore contains directory names that should be ignored during language detection.
var dirsToIgnore = map[string]struct{}{
	".git":         {},
	".github":      {},
	".gitlab":      {},
	".hg":          {},
	".idea":        {},
	".svn":         {},
	".task":        {},
	".vscode":      {},
	"build":        {},
	"dist":         {},
	"node_modules": {},
	"vendor":       {},
}
