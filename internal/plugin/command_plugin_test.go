package plugin

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/log"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

func TestConfiguredCommandPlugin(t *testing.T) {
	Reset()
	defer Reset()

	workingDir := t.TempDir()
	t.Cleanup(func() {
		require.NoError(t, log.ResetForTesting())
	})
	configDir := filepath.Join(workingDir, ".crush")
	require.NoError(t, os.MkdirAll(configDir, 0o755))

	cfg := map[string]any{
		"plugins": []map[string]any{
			{
				"name":    "helper-plugin",
				"command": os.Args[0],
				"args":    []string{"-test.run=TestCommandPluginHelperProcess"},
				"hooks":   []string{commandPluginHookMessages, commandPluginHookCompacting, commandPluginHookSystem},
				"env": map[string]string{
					"GO_WANT_COMMAND_PLUGIN_HELPER": "1",
				},
			},
		},
	}
	cfgJSON, err := json.Marshal(cfg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "crush.json"), cfgJSON, 0o644))

	store, err := config.Init(workingDir, "", false)
	require.NoError(t, err)

	err = Init(context.Background(), PluginInput{Config: store, WorkingDir: workingDir})
	require.NoError(t, err)

	transformed, err := TriggerChatMessagesTransform(context.Background(), ChatMessagesTransformInput{
		SessionID: "session-1",
		Agent:     "session",
		Purpose:   ChatTransformPurposeRequest,
	}, ChatMessagesTransformOutput{Messages: []message.Message{{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "seed"}}}}})
	require.NoError(t, err)
	require.Len(t, transformed.Messages, 2)
	require.Equal(t, "helper compacted history", transformed.Messages[1].Content().Text)

	systemTransformed, err := TriggerChatSystemTransform(context.Background(), ChatSystemTransformInput{
		SessionID: "session-1",
		Agent:     "session",
		Purpose:   ChatTransformPurposeRequest,
	}, ChatSystemTransformOutput{System: []string{"base-system"}, Prefix: "base-prefix"})
	require.NoError(t, err)
	require.Equal(t, []string{"base-system"}, systemTransformed.System)
	require.Equal(t, "base-prefix", systemTransformed.Prefix)

	compacting, err := TriggerSessionCompacting(context.Background(), SessionCompactingInput{
		SessionID: "session-1",
		Agent:     "session",
		Purpose:   ChatTransformPurposeRecover,
	}, SessionCompactingOutput{})
	require.NoError(t, err)
	require.Equal(t, []string{"helper compact note"}, compacting.Context)
	require.Equal(t, "helper compact prompt", compacting.Prompt)
}

func TestCommandPluginHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_COMMAND_PLUGIN_HELPER") != "1" {
		return
	}

	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		panic(err)
	}
	var request commandPluginRequest
	if err := json.Unmarshal(raw, &request); err != nil {
		panic(err)
	}

	var response commandPluginResponse
	switch request.Event {
	case commandPluginHookMessages:
		var output commandPluginChatMessagesTransformOutput
		if err := json.Unmarshal(request.Output, &output); err != nil {
			panic(err)
		}
		output.Messages = append(output.Messages, commandPluginMessage{
			Role:      string(message.User),
			SessionID: "session-1",
			Parts: []commandPluginPart{{
				Type: commandPluginPartText,
				Data: mustJSON(message.TextContent{Text: "helper compacted history"}),
			}},
		})
		response.Output = mustJSON(output)
	case commandPluginHookSystem:
		response.Output = mustJSON(commandPluginChatSystemTransformOutput{})
	case commandPluginHookCompacting:
		response.Output = mustJSON(commandPluginSessionCompactingOutput{
			Context: []string{"helper compact note"},
			Prompt:  "helper compact prompt",
		})
	default:
		response.Error = "unexpected event"
	}

	if _, err := os.Stdout.Write(mustJSON(response)); err != nil {
		panic(err)
	}
	os.Exit(0)
}

func TestBoundedBuffer(t *testing.T) {
	t.Run("within_limit", func(t *testing.T) {
		buf := newBoundedBuffer(8)
		n, err := buf.Write([]byte("abc"))
		require.NoError(t, err)
		require.Equal(t, 3, n)
		require.False(t, buf.Truncated())
		require.Equal(t, "abc", buf.String())
		require.Equal(t, []byte("abc"), buf.BytesForJSON())
	})

	t.Run("exceeds_limit", func(t *testing.T) {
		buf := newBoundedBuffer(5)
		n, err := buf.Write([]byte("123456789"))
		require.NoError(t, err)
		require.Equal(t, 9, n)
		require.True(t, buf.Truncated())
		require.Equal(t, "12345\n... [output truncated]", buf.String())
		require.Equal(t, []byte("12345"), buf.BytesForJSON())
	})

	t.Run("empty_truncated_json_stub", func(t *testing.T) {
		buf := newBoundedBuffer(0)
		_, err := buf.Write([]byte("abcdef"))
		require.NoError(t, err)
		require.True(t, buf.Truncated())
		require.Equal(t, commandPluginTruncatedSuffix, buf.String())
		require.Equal(t, []byte(commandPluginTruncatedJSONStub), buf.BytesForJSON())
	})
}

func TestResolveCommandPluginOutputMaxBytes(t *testing.T) {
	t.Run("default_when_unset", func(t *testing.T) {
		t.Setenv("CRUSH_PLUGIN_OUTPUT_MAX_BYTES", "")
		require.Equal(t, commandPluginDefaultOutputMaxBytes, resolveCommandPluginOutputMaxBytes())
	})

	t.Run("default_when_invalid", func(t *testing.T) {
		t.Setenv("CRUSH_PLUGIN_OUTPUT_MAX_BYTES", "invalid")
		require.Equal(t, commandPluginDefaultOutputMaxBytes, resolveCommandPluginOutputMaxBytes())
	})

	t.Run("default_when_non_positive", func(t *testing.T) {
		t.Setenv("CRUSH_PLUGIN_OUTPUT_MAX_BYTES", "0")
		require.Equal(t, commandPluginDefaultOutputMaxBytes, resolveCommandPluginOutputMaxBytes())
	})

	t.Run("uses_env_value", func(t *testing.T) {
		t.Setenv("CRUSH_PLUGIN_OUTPUT_MAX_BYTES", "2097152")
		require.Equal(t, 2<<20, resolveCommandPluginOutputMaxBytes())
	})

	t.Run("clamps_to_maximum", func(t *testing.T) {
		t.Setenv("CRUSH_PLUGIN_OUTPUT_MAX_BYTES", "999999999")
		require.Equal(t, commandPluginMaxOutputMaxBytes, resolveCommandPluginOutputMaxBytes())
	})
}

func mustJSON(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}
