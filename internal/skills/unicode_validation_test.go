package skills

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSkillValidateUnicodeLimits(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name          string
		description   string
		compatibility string
		wantError     bool
	}{
		{name: "description at limit", description: strings.Repeat("文", MaxDescriptionLength)},
		{name: "description over limit", description: strings.Repeat("文", MaxDescriptionLength+1), wantError: true},
		{name: "emoji description at limit", description: strings.Repeat("🌍", MaxDescriptionLength)},
		{name: "compatibility at limit", description: "Description", compatibility: strings.Repeat("文", MaxCompatibilityLength)},
		{name: "compatibility over limit", description: "Description", compatibility: strings.Repeat("文", MaxCompatibilityLength+1), wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			skill := Skill{Name: "unicode-skill", Description: tc.description, Compatibility: tc.compatibility}
			if tc.wantError {
				require.Error(t, skill.Validate())
			} else {
				require.NoError(t, skill.Validate())
			}
		})
	}
}
