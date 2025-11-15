package config

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAttributionMigration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		configJSON       string
		expectedTrailer  TrailerStyle
		expectedGenerate bool
	}{
		{
			name: "old setting co_authored_by=true migrates to co-authored-by",
			configJSON: `{
				"options": {
					"attribution": {
						"co_authored_by": true,
						"generated_with": false
					}
				}
			}`,
			expectedTrailer:  TrailerStyleCoAuthoredBy,
			expectedGenerate: false,
		},
		{
			name: "old setting co_authored_by=false migrates to none",
			configJSON: `{
				"options": {
					"attribution": {
						"co_authored_by": false,
						"generated_with": true
					}
				}
			}`,
			expectedTrailer:  TrailerStyleNone,
			expectedGenerate: true,
		},
		{
			name: "new setting takes precedence over old setting",
			configJSON: `{
				"options": {
					"attribution": {
						"trailer_style": "assisted-by",
						"co_authored_by": true,
						"generated_with": false
					}
				}
			}`,
			expectedTrailer:  TrailerStyleAssistedBy,
			expectedGenerate: false,
		},
		{
			name: "default when neither setting present",
			configJSON: `{
				"options": {
					"attribution": {
						"generated_with": true
					}
				}
			}`,
			expectedTrailer:  TrailerStyleAssistedBy,
			expectedGenerate: true,
		},
		{
			name: "default when attribution is null",
			configJSON: `{
				"options": {}
			}`,
			expectedTrailer:  TrailerStyleAssistedBy,
			expectedGenerate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := loadFromReaders([]io.Reader{strings.NewReader(tt.configJSON)})
			require.NoError(t, err)

			cfg.setDefaults(t.TempDir(), "")

			require.Equal(t, tt.expectedTrailer, cfg.Options.Attribution.TrailerStyle)
			require.Equal(t, tt.expectedGenerate, cfg.Options.Attribution.GeneratedWith)
		})
	}
}
