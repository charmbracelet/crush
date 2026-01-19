package tools

// MemorySearchToolName is the name of the memory_search tool.
const MemorySearchToolName = "memory_search"

// MemorySearchParams defines the parameters for the memory_search tool.
type MemorySearchParams struct {
	Query string `json:"query" description:"The query describing what information to search for in the session transcript"`
}
