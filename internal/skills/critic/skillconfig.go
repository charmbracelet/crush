package critic

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// duration is a time.Duration that unmarshals from a JSON string such as "10s".
type duration time.Duration

func (d *duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = duration(parsed)
	return nil
}

// skillConfigFile is the JSON structure expected in .crush/skills/critic/config.json.
type skillConfigFile struct {
	Enabled       *bool     `json:"enabled,omitempty"`
	Model         *string   `json:"model,omitempty"`
	MaxIterations *int      `json:"max_iterations,omitempty"`
	AutoApprove   *bool     `json:"auto_approve,omitempty"`
	Threshold     *float64  `json:"threshold,omitempty"`
	CacheSize     *int      `json:"cache_size,omitempty"`
	MaxDiffSize   *int      `json:"max_diff_size,omitempty"`
	MaxFileSize   *int      `json:"max_file_size,omitempty"`
	Timeout       *duration `json:"timeout,omitempty"`
	RetentionDays *int      `json:"retention_days,omitempty"`
}

// LoadSkillConfig searches for .crush/skills/critic/config.json (and .kimi/,
// crush/ fallbacks) relative to workDir and merges any values found over the
// provided base config. If no file exists, base is returned unchanged.
func LoadSkillConfig(base CriticSkillConfig, workDir string) (CriticSkillConfig, error) {
	var raw []byte
	var found bool
	for _, dir := range []string{".crush", ".kimi", "crush"} {
		path := filepath.Join(workDir, dir, "skills", "critic", "config.json")
		if b, err := os.ReadFile(path); err == nil {
			raw = b
			found = true
			break
		}
	}
	if !found {
		return base, nil
	}

	var file skillConfigFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return base, fmt.Errorf("parse critic skill config: %w", err)
	}

	if file.Enabled != nil {
		base.Enabled = *file.Enabled
	}
	if file.Model != nil {
		base.Model = *file.Model
	}
	if file.MaxIterations != nil {
		base.MaxIterations = *file.MaxIterations
	}
	if file.AutoApprove != nil {
		base.AutoApprove = *file.AutoApprove
	}
	if file.Threshold != nil {
		base.Threshold = *file.Threshold
	}
	if file.CacheSize != nil {
		base.CacheSize = *file.CacheSize
	}
	if file.MaxDiffSize != nil {
		base.MaxDiffSize = *file.MaxDiffSize
	}
	if file.MaxFileSize != nil {
		base.MaxFileSize = *file.MaxFileSize
	}
	if file.Timeout != nil {
		base.Timeout = time.Duration(*file.Timeout)
	}
	if file.RetentionDays != nil {
		base.RetentionDays = *file.RetentionDays
	}

	return base, nil
}
