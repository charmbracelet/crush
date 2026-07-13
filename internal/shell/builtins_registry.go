package shell

import (
	"context"
	"io"
)

// BuiltinHandler is a function that handles a shell builtin command.
// It receives the full args slice (including the command name as args[0]),
// the context (which may carry a ConfigBuilder), and I/O streams.
type BuiltinHandler func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error

// extraBuiltins holds additional builtin handlers registered by other
// packages via RegisterBuiltin. This avoids an import cycle: shell cannot
// import shellconfig (because shellconfig imports shell), so shellconfig
// registers its handlers at init time and they are dispatched here.
var extraBuiltins map[string]BuiltinHandler

// RegisterBuiltin registers a builtin command handler. Must be called before
// any shell execution (typically in init()). If a handler is already
// registered for the same name, the new one replaces it.
func RegisterBuiltin(name string, handler BuiltinHandler) {
	if extraBuiltins == nil {
		extraBuiltins = make(map[string]BuiltinHandler)
	}
	extraBuiltins[name] = handler
}
