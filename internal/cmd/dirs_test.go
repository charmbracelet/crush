package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func init() {
	os.Setenv("XDG_CONFIG_HOME", "/tmp/fakeconfig")
	os.Setenv("XDG_DATA_HOME", "/tmp/fakedata")
	os.Unsetenv("CRUSH_GLOBAL_CONFIG")
	os.Unsetenv("CRUSH_GLOBAL_DATA")
}

func TestDirs(t *testing.T) {
	var b bytes.Buffer
	dirsCmd.SetOut(&b)
	dirsCmd.SetErr(&b)
	dirsCmd.SetIn(bytes.NewReader(nil))
	dirsCmd.Run(dirsCmd, nil)
	expected := filepath.FromSlash("/tmp/fakeconfig/crush") + "\n" +
		filepath.FromSlash("/tmp/fakedata/crush") + "\n"
	require.Equal(t, expected, b.String())
}

func TestConfigDir(t *testing.T) {
	var b bytes.Buffer
	configDirCmd.SetOut(&b)
	configDirCmd.SetErr(&b)
	configDirCmd.SetIn(bytes.NewReader(nil))
	configDirCmd.Run(configDirCmd, nil)
	expected := filepath.FromSlash("/tmp/fakeconfig/crush") + "\n"
	require.Equal(t, expected, b.String())
}

func TestDataDir(t *testing.T) {
	var b bytes.Buffer
	dataDirCmd.SetOut(&b)
	dataDirCmd.SetErr(&b)
	dataDirCmd.SetIn(bytes.NewReader(nil))
	dataDirCmd.Run(dataDirCmd, nil)
	expected := filepath.FromSlash("/tmp/fakedata/crush") + "\n"
	require.Equal(t, expected, b.String())
}
