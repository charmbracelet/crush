package lsp

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"unsafe"

	"github.com/charmbracelet/crush/internal/config"
	powernapconfig "github.com/charmbracelet/x/powernap/pkg/config"
	"github.com/stretchr/testify/require"
)

func TestManagerStartServerRespectsAutoLSPDisabled(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	autoLSP := false
	store := &config.ConfigStore{}
	setUnexportedField(store, "config", &config.Config{
		Options: &config.Options{
			AutoLSP: &autoLSP,
		},
		LSP: make(map[string]config.LSPConfig),
	})
	setUnexportedField(store, "workingDir", workingDir)

	unavailable.Del("gopls")
	manager := NewManager(store)
	callbackCalled := false
	manager.SetCallback(func(string, *Client) {
		callbackCalled = true
	})

	server := &powernapconfig.ServerConfig{
		Command:     "definitely-not-an-installed-lsp",
		FileTypes:   []string{"go"},
		RootMarkers: []string{"go.mod"},
	}

	manager.startServer(context.Background(), "gopls", filepath.Join(workingDir, "main.go"), server)

	_, ok := manager.clients.Get("gopls")
	require.False(t, ok)
	_, unavailable := unavailable.Get("gopls")
	require.False(t, unavailable)
	require.False(t, callbackCalled)
}

func setUnexportedField(target any, fieldName string, value any) {
	field := reflect.ValueOf(target).Elem().FieldByName(fieldName)
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(value))
}
