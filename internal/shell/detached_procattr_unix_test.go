//go:build unix

package shell

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDetachedSysProcAttr(t *testing.T) {
	t.Parallel()

	attr := detachedSysProcAttr(true)
	require.NotNil(t, attr)
	require.True(t, attr.Setsid)
}
