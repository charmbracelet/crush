package env

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOsEnv_Get(t *testing.T) {
	env := New()

	// Test getting an existing environment variable
	t.Setenv("TEST_VAR", "test_value")

	value := env.Get("TEST_VAR")
	assert.Equal(t, "test_value", value)

	// Test getting a non-existent environment variable
	value = env.Get("NON_EXISTENT_VAR")
	assert.Equal(t, "", value)
}

func TestOsEnv_Env(t *testing.T) {
	env := New()

	envVars := env.Env()

	// Environment should not be empty in normal circumstances
	assert.NotNil(t, envVars)
	assert.Greater(t, len(envVars), 0)

	// Each environment variable should be in key=value format
	for _, envVar := range envVars {
		assert.Contains(t, envVar, "=")
	}
}

func TestNewFromMap(t *testing.T) {
	testMap := map[string]string{
		"KEY1": "value1",
		"KEY2": "value2",
	}

	env := NewFromMap(testMap)
	assert.NotNil(t, env)
	assert.IsType(t, &mapEnv{}, env)
}

func TestMapEnv_Get(t *testing.T) {
	testMap := map[string]string{
		"KEY1": "value1",
		"KEY2": "value2",
	}

	env := NewFromMap(testMap)

	// Test getting existing keys
	assert.Equal(t, "value1", env.Get("KEY1"))
	assert.Equal(t, "value2", env.Get("KEY2"))

	// Test getting non-existent key
	assert.Equal(t, "", env.Get("NON_EXISTENT"))
}

func TestMapEnv_Env(t *testing.T) {
	t.Run("with values", func(t *testing.T) {
		testMap := map[string]string{
			"KEY1": "value1",
			"KEY2": "value2",
		}

		env := NewFromMap(testMap)
		envVars := env.Env()

		assert.Len(t, envVars, 2)

		// Convert to map for easier testing (order is not guaranteed)
		envMap := make(map[string]string)
		for _, envVar := range envVars {
			parts := strings.SplitN(envVar, "=", 2)
			require.Len(t, parts, 2)
			envMap[parts[0]] = parts[1]
		}

		assert.Equal(t, "value1", envMap["KEY1"])
		assert.Equal(t, "value2", envMap["KEY2"])
	})

	t.Run("empty map", func(t *testing.T) {
		env := NewFromMap(map[string]string{})
		envVars := env.Env()
		assert.Nil(t, envVars)
	})

	t.Run("nil map", func(t *testing.T) {
		env := NewFromMap(nil)
		envVars := env.Env()
		assert.Nil(t, envVars)
	})
}

func TestMapEnv_GetEmptyValue(t *testing.T) {
	testMap := map[string]string{
		"EMPTY_KEY":  "",
		"NORMAL_KEY": "value",
	}

	env := NewFromMap(testMap)

	// Test that empty values are returned correctly
	assert.Equal(t, "", env.Get("EMPTY_KEY"))
	assert.Equal(t, "value", env.Get("NORMAL_KEY"))
}

func TestMapEnv_EnvFormat(t *testing.T) {
	testMap := map[string]string{
		"KEY_WITH_EQUALS": "value=with=equals",
		"KEY_WITH_SPACES": "value with spaces",
	}

	env := NewFromMap(testMap)
	envVars := env.Env()

	assert.Len(t, envVars, 2)

	// Check that the format is correct even with special characters
	found := make(map[string]bool)
	for _, envVar := range envVars {
		if envVar == "KEY_WITH_EQUALS=value=with=equals" {
			found["equals"] = true
		}
		if envVar == "KEY_WITH_SPACES=value with spaces" {
			found["spaces"] = true
		}
	}

	assert.True(t, found["equals"], "Should handle values with equals signs")
	assert.True(t, found["spaces"], "Should handle values with spaces")
}
