package model

import (
	"reflect"
	"testing"
	"unsafe"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/stretchr/testify/require"
)

func TestCurrentModelSupportsImages(t *testing.T) {
	t.Parallel()

	t.Run("returns false when config is nil", func(t *testing.T) {
		t.Parallel()

		ui := newTestUIWithConfig(t, nil)
		require.False(t, ui.currentModelSupportsImages())
	})

	t.Run("returns false when coder agent is missing", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{
			Providers: csync.NewMap[string, config.ProviderConfig](),
			Agents:    map[string]config.Agent{},
		}
		ui := newTestUIWithConfig(t, cfg)
		require.False(t, ui.currentModelSupportsImages())
	})

	t.Run("returns false when model is not found", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{
			Providers: csync.NewMap[string, config.ProviderConfig](),
			Agents: map[string]config.Agent{
				config.AgentCoder: {Model: config.SelectedModelTypeLarge},
			},
		}
		ui := newTestUIWithConfig(t, cfg)
		require.False(t, ui.currentModelSupportsImages())
	})

	t.Run("returns true when current model supports images", func(t *testing.T) {
		t.Parallel()

		providers := csync.NewMap[string, config.ProviderConfig]()
		providers.Set("test-provider", config.ProviderConfig{
			ID: "test-provider",
			Models: []catwalk.Model{
				{ID: "test-model", SupportsImages: true},
			},
		})

		cfg := &config.Config{
			Models: map[config.SelectedModelType]config.SelectedModel{
				config.SelectedModelTypeLarge: {
					Provider: "test-provider",
					Model:    "test-model",
				},
			},
			Providers: providers,
			Agents: map[string]config.Agent{
				config.AgentCoder: {Model: config.SelectedModelTypeLarge},
			},
		}

		ui := newTestUIWithConfig(t, cfg)
		require.True(t, ui.currentModelSupportsImages())
	})
}

func newTestUIWithConfig(t *testing.T, cfg *config.Config) *UI {
	t.Helper()

	store := &config.ConfigStore{}
	setUnexportedField(t, store, "config", cfg)

	appInstance := &app.App{}
	setUnexportedField(t, appInstance, "config", store)

	return &UI{
		com: &common.Common{
			App: appInstance,
		},
	}
}

func setUnexportedField(t *testing.T, target any, name string, value any) {
	t.Helper()

	v := reflect.ValueOf(target)
	require.Equal(t, reflect.Pointer, v.Kind())
	require.False(t, v.IsNil())

	field := v.Elem().FieldByName(name)
	require.Truef(t, field.IsValid(), "field %q not found", name)

	fieldValue := reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
	fieldValue.Set(reflect.ValueOf(value))
}
