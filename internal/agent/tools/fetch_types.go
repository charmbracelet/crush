package tools

// AgenticFetchToolName is the name of the agentic fetch tool.
const AgenticFetchToolName = "agentic_fetch"

// WebFetchToolName is the name of the web_fetch tool.
const WebFetchToolName = "web_fetch"

// LargeContentThreshold is the size threshold for saving content to a file.
const LargeContentThreshold = 50000 // 50KB

// AgenticFetchParams defines the parameters for the agentic fetch tool.
type AgenticFetchParams struct {
	URL    string `json:"url" description:"The URL to fetch content from"`
	Prompt string `json:"prompt" description:"The prompt to run on the fetched content"`
}

// AgenticFetchPermissionsParams defines the permission parameters for the agentic fetch tool.
type AgenticFetchPermissionsParams struct {
	URL    string `json:"url"`
	Prompt string `json:"prompt"`
}

// WebFetchParams defines the parameters for the web_fetch tool.
type WebFetchParams struct {
	URL string `json:"url" description:"The URL to fetch content from"`
}

// FetchParams defines the parameters for the simple fetch tool.
type FetchParams struct {
	URL     string `json:"url" description:"The URL to fetch content from"`
	Format  string `json:"format" description:"The format to return the content in (text, markdown, or html)"`
	Timeout int    `json:"timeout,omitempty" description:"Optional timeout in seconds (max 120)"`
}

// FetchPermissionsParams defines the permission parameters for the simple fetch tool.
type FetchPermissionsParams struct {
	URL     string `json:"url"`
	Format  string `json:"format"`
	Timeout int    `json:"timeout,omitempty"`
}
