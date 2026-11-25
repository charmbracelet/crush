package tools

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/fantasy"
)

type (
	sessionIDContextKey string
	messageIDContextKey string
	supportsImagesKey   string
	modelNameKey        string
	channelContextKey   string
)

const (
	// SessionIDContextKey is the key for the session ID in the context.
	SessionIDContextKey sessionIDContextKey = "session_id"
	// MessageIDContextKey is the key for the message ID in the context.
	MessageIDContextKey messageIDContextKey = "message_id"
	// SupportsImagesContextKey is the key for the model's image support capability.
	SupportsImagesContextKey supportsImagesKey = "supports_images"
	// ModelNameContextKey is the key for the model name in the context.
	ModelNameContextKey modelNameKey = "model_name"
	// ChannelContextKey is the key for the channel that originated the turn.
	ChannelContextKey channelContextKey = "channel"
)

// getContextValue is a generic helper that retrieves a typed value from context.
// If the value is not found or has the wrong type, it returns the default value.
func getContextValue[T any](ctx context.Context, key any, defaultValue T) T {
	value := ctx.Value(key)
	if value == nil {
		return defaultValue
	}
	if typedValue, ok := value.(T); ok {
		return typedValue
	}
	return defaultValue
}

// GetSessionFromContext retrieves the session ID from the context.
func GetSessionFromContext(ctx context.Context) string {
	return getContextValue(ctx, SessionIDContextKey, "")
}

// GetChannelFromContext retrieves the channel that originated the current turn.
func GetChannelFromContext(ctx context.Context) string {
	return getContextValue(ctx, ChannelContextKey, "")
}

// GetMessageFromContext retrieves the message ID from the context.
func GetMessageFromContext(ctx context.Context) string {
	return getContextValue(ctx, MessageIDContextKey, "")
}

// GetSupportsImagesFromContext retrieves whether the model supports images from the context.
func GetSupportsImagesFromContext(ctx context.Context) bool {
	return getContextValue(ctx, SupportsImagesContextKey, false)
}

// GetModelNameFromContext retrieves the model name from the context.
func GetModelNameFromContext(ctx context.Context) string {
	return getContextValue(ctx, ModelNameContextKey, "")
}

// UnsupportedMediaResponse tells the model it cannot receive a tool's media.
func UnsupportedMediaResponse(modelName string) fantasy.ToolResponse {
	return fantasy.NewTextErrorResponse(fmt.Sprintf("This model (%s) does not support image data.", modelName))
}

// NewPermissionDeniedResponse returns a tool response indicating the user
// denied permission, with StopTurn set so the agent loop does not retry.
func NewPermissionDeniedResponse() fantasy.ToolResponse {
	resp := fantasy.NewTextErrorResponse("User denied permission")
	resp.StopTurn = true
	return resp
}

// ghAvailable indicates whether the `gh` CLI is available on PATH.
var ghAvailable = func() bool {
	if testing.Testing() {
		return false
	}
	_, err := exec.LookPath("gh")
	return err == nil
}()

// toolDescriptionData is the common data structure for tool description templates.
type toolDescriptionData struct {
	GhAvailable bool
}

// renderToolDescription renders a tool description template with the given data.
func renderToolDescription(tmpl *template.Template) string {
	data := toolDescriptionData{
		GhAvailable: ghAvailable,
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		panic("failed to execute tool description template: " + err.Error())
	}
	return out.String()
}

// renderTemplate renders a Go template with the given data.
func renderTemplate(tmpl *template.Template, data any) string {
	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		panic("failed to execute tool description template: " + err.Error())
	}
	return out.String()
}

// rootsContextKey carries the session's effective root set.
type rootsContextKey string

// RootsContextKey is the key for the session's root set in the context.
const RootsContextKey rootsContextKey = "session_roots"

// systemPromptAppendContextKey carries ACP append-only system instructions.
type systemPromptAppendContextKey struct{}

// WithSystemPromptAppend adds trusted ACP system instructions to ctx.
func WithSystemPromptAppend(ctx context.Context, text string) context.Context {
	return context.WithValue(ctx, systemPromptAppendContextKey{}, text)
}

// SystemPromptAppend returns ACP system instructions carried by ctx.
func SystemPromptAppend(ctx context.Context) string {
	text, _ := ctx.Value(systemPromptAppendContextKey{}).(string)
	return text
}

// GetRootsFromContext retrieves the session's effective root set. An empty
// result means the caller is not root-scoped (a CLI run, a test), and the
// tools treat every path as in scope.
func GetRootsFromContext(ctx context.Context) []string {
	return getContextValue(ctx, RootsContextKey, []string(nil))
}

// OutsideRoots reports whether path escapes the session's root set.
//
// The check is by path component on the resolved path and fails closed, which
// is what the spec asks of an implementation that enforces the root set
// (session-setup.md:452). An empty root set means enforcement is off.
func OutsideRoots(roots []string, path string) bool {
	if len(roots) == 0 || path == "" {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return true
	}
	absPath = filepath.Clean(absPath)
	for _, root := range roots {
		if root == "" {
			continue
		}
		absRoot, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(filepath.Clean(absRoot), absPath)
		if err != nil {
			continue
		}
		// A relative result of "..", or one that starts with a parent
		// component, is outside; anything else is inside.
		if rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
			return false
		}
	}
	return true
}

// ClientFS is a client-side file system the tools read and write through,
// when the connected ACP client advertises one. Delegating to the client is
// what makes unsaved editor buffers visible to the Agent (file-system.md).
type ClientFS interface {
	ReadTextFile(ctx context.Context, path string) (string, error)
	WriteTextFile(ctx context.Context, path, content string) error
}

// clientFS is the process-wide client file system, installed by the ACP
// server when its client advertises fs support. Nil means the tools use the
// local disk, which is the CLI and TUI behavior.
var clientFS ClientFS

// SetClientFS installs the client file system. Passing nil restores local
// disk access.
func SetClientFS(fs ClientFS) { clientFS = fs }

// ReadFileText reads a text file, preferring the connected client's view of
// it so unsaved edits are visible.
func ReadFileText(ctx context.Context, path string) (string, error) {
	if clientFS != nil {
		return clientFS.ReadTextFile(ctx, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// WriteFileText writes a text file, preferring the connected client so its
// editor state and change tracking stay correct.
func WriteFileText(ctx context.Context, path, content string) error {
	if clientFS != nil {
		return clientFS.WriteTextFile(ctx, path, content)
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
