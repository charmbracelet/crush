package critic

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadSkillConfig_NoFile(t *testing.T) {
	t.Parallel()
	base := CriticSkillConfig{Enabled: true, Threshold: 0.9}
	cfg, err := LoadSkillConfig(base, t.TempDir())
	require.NoError(t, err)
	require.Equal(t, base, cfg)
}

func TestLoadSkillConfig_MergesValues(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, ".crush", "skills", "critic")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	content := `{"enabled":false,"threshold":0.5,"max_iterations":7,"timeout":"5s","retention_days":14}`
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "config.json"), []byte(content), 0o644))

	base := CriticSkillConfig{Enabled: true, Threshold: 0.9, MaxIterations: 3, Timeout: 10 * time.Second}
	cfg, err := LoadSkillConfig(base, tmp)
	require.NoError(t, err)
	require.False(t, cfg.Enabled)
	require.InDelta(t, 0.5, cfg.Threshold, 0.001)
	require.Equal(t, 7, cfg.MaxIterations)
	require.Equal(t, 5*time.Second, cfg.Timeout)
	require.Equal(t, 14, cfg.RetentionDays)
}

func TestLoadSkillConfig_InvalidJSON(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, ".crush", "skills", "critic")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "config.json"), []byte("not json"), 0o644))

	base := CriticSkillConfig{Enabled: true}
	cfg, err := LoadSkillConfig(base, tmp)
	require.Error(t, err)
	require.Equal(t, base, cfg)
}
