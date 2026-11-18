//go:build !windows

package config

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/home"
	"github.com/stretchr/testify/require"
)

func TestGlobalPathsLiveUnderScratchHomeUnderTest(t *testing.T) {
	for name, path := range map[string]string{
		"config": GlobalConfig(),
		"data":   GlobalConfigData(),
		"cache":  GlobalCacheDir(),
	} {
		require.Truef(t, strings.HasPrefix(path, home.Dir()+string(filepath.Separator)), "%s path %q escapes the test home", name, path)
	}
}
