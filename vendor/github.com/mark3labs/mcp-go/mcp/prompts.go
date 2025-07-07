package mcp

/* Prompts */

// ListPromptsRequest is sent from the client to request a list of prompts and
// prompt templates the server has.
type ListPromptsRequest struct {
	PaginatedRequest
}

// ListPromptsResult is the server's response to a prompts/list request from
// the client.
type ListPromptsResult struct {
	PaginatedResult
	Prompts []Prompt `json:"prompts"`
}

// GetPromptRequest is used by the client to get a prompt provided by the
// server.
type GetPromptRequest struct {
	Request
	Params GetPromptParams `json:"params"`
}

type GetPromptParams struct {
	// The name of the prompt or prompt template.
	Name string `json:"name"`
	// Arguments to use for templating the prompt.
	Arguments map[string]string `json:"arguments,omitempty"`
}

// GetPromptResult is the server's response to a prompts/get request from the
// client.
type GetPromptResult struct {
	Result
	// An optional description for the prompt.
	Description string          `json:"description,omitempty"`
	Messages    []PromptMessage `json:"messages"`
}

// Prompt represents a prompt or prompt template that the server offers.
// If Arguments is non-nil and non-empty, this indicates the prompt is a template
// that requires argument values to be provided when calling prompts/get.
// If Arguments is nil or empty, this is a static prompt that takes no arguments.
type Prompt struct {
	// The name of the prompt or prompt template.
	Name string `json:"name"`
	// An optional description of what this prompt provides
	Description string `json:"description,omitempty"`
	// A list of arguments to use for templating the prompt.
	// The presence of arguments indicates this is a template prompt.
	Arguments []PromptArgument `json:"arguments,omitempty"`
}

// GetName returns the name of the prompt.
func (p Prompt) GetName() string {
	return p.Name
}

// PromptArgument describes an argument that a prompt template can accept.
// When a prompt includes arguments, clients must provide values for all
// required arguments when making a prompts/get request.
type PromptArgument struct {
	// The name of the argument.
	Name string `json:"name"`
	// A human-readable description of the argument.
	Description string `json:"description,omitempty"`
	// Whether this argument must be provided.
	// If true, clients must include this argument when calling prompts/get.
	Required bool `json:"required,omitempty"`
}

// Role represents the sender or recipient of messages and data in a
// conversation.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// PromptMessage describes a message returned as part of a prompt.
//
// This is similar to `SamplingMessage`, but also supports the embedding of
// resources from the MCP server.
type PromptMessage struct {
	Role    Role    `json:"role"`
	Content Content `json:"content"` // Can be TextContent, ImageContent, AudioContent or EmbeddedResource
}

// PromptListChangedNotification is an optional notification from the server
// to the client, informing it that the list of prompts it offers has changed. This
// may be issued by servers without any previous subscription from the client.
type PromptListChangedNotification struct {
	Notification
}

// PromptOption is a function that configures a Prompt.
// It provides a flexible way to set various properties of a Prompt using the functional options pattern.
type PromptOption func(*Prompt)

// ArgumentOption is a function that configures a PromptArgument.
// It allows for flexible configuration of prompt arguments using the functional options pattern.
type ArgumentOption func(*PromptArgument)

//
// Core Prompt Functions
//

// NewPrompt creates a new Prompt with the given name and options.
// The prompt will be configured based on the provided options.
// Options are applied in order, allowing for flexible prompt configuration.
func NewPrompt(name string, opts ...PromptOption) Prompt {
	prompt := Prompt{
		Name: name,
	}

	for _, opt := range opts {
		opt(&prompt)
	}

	return prompt
}

// WithPromptDescription adds a description to the Prompt.
// The description should provide a clear, human-readable explanation of what the prompt does.
func WithPromptDescription(description string) PromptOption {
	return func(p *Prompt) {
		p.Description = description
	}
}

// WithArgument adds an argument to the prompt's argument list.
// The argument will be configured based on the provided options.
func WithArgument(name string, opts ...ArgumentOption) PromptOption {
	return func(p *Prompt) {
		arg := PromptArgument{
			Name: name,
		}

		for _, opt := range opts {
			opt(&arg)
		}

		if p.Arguments == nil {
			p.Arguments = make([]PromptArgument, 0)
		}
		p.Arguments = append(p.Arguments, arg)
	}
}

//
// Argument Options
//

// ArgumentDescription adds a description to a prompt argument.
// The description should explain the purpose and expected values of the argument.
func ArgumentDescription(desc string) ArgumentOption {
	return func(arg *PromptArgument) {
		arg.Description = desc
	}
}

// RequiredArgument marks an argument as required in the prompt.
// Required arguments must be provided when getting the prompt.
func RequiredArgument() ArgumentOption {
	return func(arg *PromptArgument) {
		arg.Required = true
	}
}
