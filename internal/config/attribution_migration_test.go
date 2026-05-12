package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAttributionMigration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		configJSON      string
		expectedTrailer TrailerStyle
	}{
		{
			name: "old setting co_authored_by=true migrates to co-authored-by",
			configJSON: `{
				"options": {
					"attribution": {
						"co_authored_by": true
					}
				}
			}`,
			expectedTrailer: TrailerStyleCoAuthoredBy,
		},
		{
			name: "old setting co_authored_by=false migrates to none",
			configJSON: `{
				"options": {
					"attribution": {
						"co_authored_by": false
					}
				}
			}`,
			expectedTrailer: TrailerStyleNone,
		},
		{
			name: "new setting takes precedence over old setting",
			configJSON: `{
				"options": {
					"attribution": {
						"trailer_style": "assisted-by",
						"co_authored_by": true
					}
				}
			}`,
			expectedTrailer: TrailerStyleAssistedBy,
		},
		{
			name: "default when neither setting present",
			configJSON: `{
				"options": {
					"attribution": {}
				}
			}`,
			expectedTrailer: TrailerStyleAssistedBy,
		},
		{
			name: "default when attribution is null",
			configJSON: `{
				"options": {}
			}`,
			expectedTrailer: TrailerStyleAssistedBy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := loadFromBytes([][]byte{[]byte(tt.configJSON)})
			require.NoError(t, err)

			cfg.setDefaults(t.TempDir(), "")

			require.Equal(t, tt.expectedTrailer, cfg.Options.Attribution.TrailerStyle)
		})
	}
}
