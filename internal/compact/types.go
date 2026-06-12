// Package compact provides algorithmic, LLM-free session compaction for
// Crush. It extracts structured sections from conversation history and
// produces deterministic, cache-friendly summaries that preserve the
// information a coding agent needs to continue working.
//
// The design is based on pi-vcc (github.com/monotykamary/pi-vcc), adapted
// for Crush's message model and tool set.
package compact

// BlockKind identifies the type of a normalized block.
type BlockKind int

const (
	BlockUser BlockKind = iota
	BlockAssistant
	BlockToolCall
	BlockToolResult
	BlockBash
)

// NormalizedBlock is a uniform representation of a conversation fragment.
// All extractors operate on this type instead of raw messages.
type NormalizedBlock struct {
	Kind        BlockKind
	Text        string            // user text, assistant text, or bash output
	Name        string            // tool name (for ToolCall and ToolResult)
	Args        map[string]string // parsed tool arguments (for ToolCall)
	RawInput    string            // raw JSON input (for ToolCall)
	ResultText  string            // tool result content (for ToolResult)
	IsError     bool              // whether tool result is an error
	Command     string            // bash command (for Bash)
	Output      string            // bash output (for Bash)
	ExitCode    int               // bash exit code (-1 if unknown)
	SourceIndex int               // index in original message slice
}

// SectionData holds all extracted sections from a conversation.
type SectionData struct {
	SessionGoal        []string
	OutstandingContext []string
	FilesAndChanges    []string
	Commits            []string
	UserPreferences    []string
	TypeCatalog        []string
	TurnSummaries      []string
	BriefTranscript    string
}

// SymbolRef links a symbol name to a file and access type.
type SymbolRef struct {
	Name   string
	File   string
	Kind   string // "function", "type", "class", "variable"
	Access string // "modified" or "read"
}

// FileActivity tracks which files were read, modified, or created.
type FileActivity struct {
	Read     []string
	Modified []string
	Created  []string
	Symbols  map[string][]string // file → symbol names
}

// ExportSig holds exported signatures for a file.
type ExportSig struct {
	File       string
	Signatures []string
	Modified   bool
}

// UnifiedExtractResult combines all file/symbol extraction results.
type UnifiedExtractResult struct {
	FileActivity  FileActivity
	TypeCatalog   []ExportSig
	SymbolChanges []SymbolRef
}
