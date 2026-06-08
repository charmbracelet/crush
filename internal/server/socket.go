//go:build !windows

package server

// isStaleSocketErr is the internal, non-Windows alias for the
// cross-platform IsStaleSocketErr. It is kept for the existing
// callers in net_other.go.
func isStaleSocketErr(err error) bool {
	return IsStaleSocketErr(err)
}
