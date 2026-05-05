package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunAWSAuthRefresh(t *testing.T) {
	t.Run("empty command is a no-op", func(t *testing.T) {
		require.NoError(t, RunAWSAuthRefresh(""))
		require.NoError(t, RunAWSAuthRefresh("   "))
	})

	t.Run("successful command returns nil", func(t *testing.T) {
		require.NoError(t, RunAWSAuthRefresh("true"))
	})

	t.Run("non-zero exit surfaces error", func(t *testing.T) {
		err := RunAWSAuthRefresh("false")
		require.Error(t, err)
	})

	t.Run("shell expansion works", func(t *testing.T) {
		err := RunAWSAuthRefresh("test 1 = 1 && exit 0")
		assert.NoError(t, err)
	})
}
