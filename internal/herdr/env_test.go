package herdr

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStripEnvDropsOnlyHerdrVars(t *testing.T) {
	t.Parallel()
	in := []string{"PATH=/bin", "HERDR_ENV=1", "HERDR_SOCKET_PATH=/s", "HERDR_PANE_ID=p1", "HERDR_BIN_PATH=/h", "HERDRX=keep"}
	require.Equal(t, []string{"PATH=/bin", "HERDRX=keep"}, StripEnv(in))
	require.Len(t, in, 6, "input untouched")
}
