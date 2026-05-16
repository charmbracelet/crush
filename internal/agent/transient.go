package agent

import (
	"errors"
	"io"
	"strings"
	"syscall"
)

// transientMaxAttempts caps how many times a stream may be re-issued after a
// transient network error. Total attempts = transientMaxAttempts (initial try
// plus retries). Keep this small: fantasy already retries inside the provider
// for the errors it understands, so this layer only catches the residual raw
// network resets that surface unwrapped.
const transientMaxAttempts = 3

// isTransientNetErr reports whether err is a transient network failure that
// is safe to retry at the agent layer. It covers the cases fantasy's own
// retry logic misses: raw syscall-level connection resets and broken pipes
// that surface unwrapped from the HTTP client when the provider tears down
// the TCP connection mid-handshake or before any stream bytes arrive.
//
// The check is conservative on purpose. We only return true for errors that:
//   - are not a context cancellation/deadline (those must propagate);
//   - clearly indicate the peer closed the socket (ECONNRESET, EPIPE), or
//     the stream ended before a complete response (io.ErrUnexpectedEOF);
//   - or contain the canonical "connection reset by peer" substring as a
//     defensive fallback for wrapped errors that lost their sentinel chain.
func isTransientNetErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.ECONNRESET) {
		return true
	}
	if errors.Is(err, syscall.EPIPE) {
		return true
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	// Fallback: some HTTP/TLS stacks wrap the syscall error in a way that
	// strips the sentinel by the time it reaches us. Match the canonical
	// text Go uses for ECONNRESET on every supported platform.
	return strings.Contains(err.Error(), "connection reset by peer")
}
