// Package nvim implements editor.Bridge by talking to a Neovim instance
// over msgpack-rpc.
//
// Detection: when Crush is launched from inside :terminal, Neovim sets
// $NVIM (Neovim 0.9+) or $NVIM_LISTEN_ADDRESS (older) to the path of the
// parent's RPC socket. Crush dials that exact socket, so multiple Neovim
// instances never confuse each other: the (nvim, crush) pair is fixed at
// process spawn time.
//
// Behavior: all Crush -> Neovim calls invoke functions on a single Lua
// module (`require('neocrush.bridge')`) that the user's plugin registers
// at startup. This keeps the wire protocol minimal and lets the plugin
// evolve the Lua-side behavior without a Crush release.
package nvim

import (
	"context"
	"fmt"
	"os"
	"sync"

	gonvim "github.com/neovim/go-client/nvim"

	"github.com/taigrr/crush/internal/editor"
)

// envVars are the environment variables Neovim sets in :terminal jobs.
// $NVIM is preferred; $NVIM_LISTEN_ADDRESS is the legacy name.
var envVars = []string{"NVIM", "NVIM_LISTEN_ADDRESS"}

// detectAddress returns the first non-empty value of the known env vars,
// or "" if none are set.
func detectAddress() string {
	for _, v := range envVars {
		if addr := os.Getenv(v); addr != "" {
			return addr
		}
	}
	return ""
}

// Bridge is a Neovim-backed editor.Bridge.
type Bridge struct {
	addr   string
	mu     sync.Mutex
	client *gonvim.Nvim
	closed bool
}

// New attempts to dial the parent Neovim instance. If no $NVIM env var is
// present it returns (nil, false) so the caller can substitute editor.Noop.
// Dial errors return (nil, false) as well: we never want a misbehaving
// editor connection to block Crush from starting.
func New() (*Bridge, bool) {
	addr := detectAddress()
	if addr == "" {
		return nil, false
	}
	b := &Bridge{addr: addr}
	if _, err := b.conn(); err != nil {
		return nil, false
	}
	return b, true
}

// conn returns a live msgpack-rpc client, lazily dialing on first use and
// reconnecting if the previous connection died.
func (b *Bridge) conn() (*gonvim.Nvim, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, editor.ErrUnavailable
	}
	if b.client != nil {
		return b.client, nil
	}
	c, err := gonvim.Dial(b.addr)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", b.addr, err)
	}
	b.client = c
	return c, nil
}

// reset drops the cached client; the next conn() call will redial. Used
// when an RPC call fails so a transient failure (e.g. nvim restart in
// dev) recovers automatically.
func (b *Bridge) reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.client != nil {
		_ = b.client.Close()
		b.client = nil
	}
}

// Available implements editor.Bridge.
func (b *Bridge) Available() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return !b.closed && b.addr != ""
}

// Close implements editor.Bridge.
func (b *Bridge) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	if b.client == nil {
		return nil
	}
	err := b.client.Close()
	b.client = nil
	return err
}

// callBridge invokes a function on the Lua bridge module loaded by the
// plugin. It runs inside Neovim's main loop via ExecLua, so all calls are
// safe with respect to buffer state.
//
// fn is the function name on the bridge module; out receives the
// JSON-decoded return value (pass nil to discard); args are forwarded as
// Lua positional arguments.
//
// On RPC error the connection is reset so the next call redials.
func (b *Bridge) callBridge(ctx context.Context, fn string, out any, args ...any) error {
	c, err := b.conn()
	if err != nil {
		return err
	}

	// Resolve the bridge module lazily inside Lua so we get a clear error
	// if the plugin is missing rather than a cryptic nil-index panic.
	code := fmt.Sprintf(`
local ok, mod = pcall(require, 'neocrush.bridge')
if not ok then
  error('neocrush.bridge plugin module not found: ' .. tostring(mod))
end
local fn = mod[%q]
if type(fn) ~= 'function' then
  error('neocrush.bridge.%s is not a function')
end
return fn(...)
`, fn, fn)

	// ExecLua takes a *single* result; pass nil to discard.
	if err := c.ExecLua(code, out, args...); err != nil {
		// Any RPC failure is a cue to redial next time. Closing a
		// healthy connection here would be wasteful, but in practice
		// ExecLua only errors on transport problems or genuine Lua
		// faults; either way a fresh dial costs ~1ms and resets state.
		b.reset()
		return fmt.Errorf("nvim bridge call %s: %w", fn, err)
	}
	_ = ctx // ExecLua is synchronous; ctx is reserved for future cancellation.
	return nil
}

// Context implements editor.Bridge.
func (b *Bridge) Context(ctx context.Context) (editor.EditorContext, error) {
	var raw struct {
		Path          string `msgpack:"path"`
		URI           string `msgpack:"uri"`
		Line          int    `msgpack:"line"`
		Column        int    `msgpack:"column"`
		ContextBefore string `msgpack:"context_before"`
		ContextLine   string `msgpack:"context_line"`
		ContextAfter  string `msgpack:"context_after"`
		TotalLines    int    `msgpack:"total_lines"`
		HasSelection  bool   `msgpack:"has_selection"`
		Selection     string `msgpack:"selection"`
	}
	if err := b.callBridge(ctx, "context", &raw); err != nil {
		return editor.EditorContext{}, err
	}
	return editor.EditorContext{
		Path:          raw.Path,
		URI:           raw.URI,
		Cursor:        editor.Position{Line: raw.Line, Column: raw.Column},
		ContextBefore: raw.ContextBefore,
		ContextLine:   raw.ContextLine,
		ContextAfter:  raw.ContextAfter,
		TotalLines:    raw.TotalLines,
		HasSelection:  raw.HasSelection,
		Selection:     raw.Selection,
	}, nil
}

// ShowLocations implements editor.Bridge.
func (b *Bridge) ShowLocations(ctx context.Context, title string, items []editor.Location) error {
	if len(items) == 0 {
		return nil
	}
	return b.callBridge(ctx, "show_locations", nil, title, items)
}

// FlashEdit implements editor.Bridge.
func (b *Bridge) FlashEdit(ctx context.Context, path string, startLine, endLine int) error {
	if path == "" {
		return nil
	}
	return b.callBridge(ctx, "flash_edit", nil, path, startLine, endLine)
}

// NotifyFileChanged implements editor.Bridge.
func (b *Bridge) NotifyFileChanged(ctx context.Context, path string) error {
	if path == "" {
		return nil
	}
	return b.callBridge(ctx, "file_changed", nil, path)
}
