package configv2

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func resetEnvVars() {
	os.Setenv("ANTHROPIC_API_KEY", "")
	os.Setenv("OPENAI_API_KEY", "")
	os.Setenv("GEMINI_API_KEY", "")
	os.Setenv("XAI_API_KEY", "")
	os.Setenv("OPENROUTER_API_KEY", "")
}

func TestConfigWithEnv(t *testing.T) {
	resetEnvVars()
	testConfigDir = t.TempDir()

	cwdDir := t.TempDir()

	os.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")
	os.Setenv("OPENAI_API_KEY", "test-openai-key")
	os.Setenv("GEMINI_API_KEY", "test-gemini-key")
	os.Setenv("XAI_API_KEY", "test-xai-key")
	os.Setenv("OPENROUTER_API_KEY", "test-openrouter-key")
	cfg := InitConfig(cwdDir)
	data, _ := json.MarshalIndent(cfg, "", "  ")
	fmt.Println(string(data))
	assert.Len(t, cfg.Providers, 5)
}
