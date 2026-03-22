package plugin

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/log"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

func TestPersistentPlugin(t *testing.T) {
	Reset()
	defer Reset()

	workingDir := t.TempDir()
	t.Cleanup(func() {
		require.NoError(t, log.ResetForTesting())
	})
	configDir := filepath.Join(workingDir, ".crush")
	require.NoError(t, os.MkdirAll(configDir, 0o755))

	// Persistent mode: reads JSON lines, writes JSON lines with request id.
	script := `const protocolVersion = 1;
const rl = require('readline').createInterface({ input: process.stdin, crlfDelay: Infinity });

rl.on('line', (line) => {
  if (!line.trim()) return;
  const request = JSON.parse(line);
  const id = request.id;
  const output = request.output || {};

  if (request.event === 'chat_messages_transform') {
    const messages = Array.isArray(output.messages) ? output.messages : [];
    messages.push({
      role: 'user',
      session_id: 'session-1',
      parts: [{ type: 'text', data: { text: 'persistent compacted history' } }]
    });
    process.stdout.write(JSON.stringify({ id, output: { ...output, messages } }) + '\n');
  } else if (request.event === 'session_compacting') {
    process.stdout.write(JSON.stringify({ id, output: { context: ['persistent compact note'], prompt: 'persistent compact prompt' } }) + '\n');
  } else if (request.event === 'chat_system_transform') {
    process.stdout.write(JSON.stringify({ id, output: { ...output } }) + '\n');
  } else {
    process.stdout.write(JSON.stringify({ id, output }) + '\n');
  }
});
`
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "persistent-test.js"), []byte(script), 0o644))

	// Create plugin.json with mode: persistent
	pluginJSON := map[string]any{
		"name":    "persistent-test",
		"command": "node",
		"args":    []string{filepath.Join(configDir, "persistent-test.js")},
		"hooks":   []string{commandPluginHookMessages, commandPluginHookCompacting, commandPluginHookSystem},
		"mode":    "persistent",
	}

	// Also need to set up as a configured plugin via crush.json
	cfg := map[string]any{
		"plugins": []map[string]any{pluginJSON},
	}
	cfgJSON, err := json.Marshal(cfg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "crush.json"), cfgJSON, 0o644))

	store, err := config.Init(workingDir, "", false)
	require.NoError(t, err)

	err = Init(context.Background(), PluginInput{Config: store, WorkingDir: workingDir})
	require.NoError(t, err)

	// Verify the plugin was registered.
	names := ListPlugins()
	require.Contains(t, names, "persistent-test")

	// Test chat_messages_transform hook.
	transformed, err := TriggerChatMessagesTransform(context.Background(), ChatMessagesTransformInput{
		SessionID: "session-1",
		Agent:     "coder",
		Purpose:   ChatTransformPurposeRequest,
	}, ChatMessagesTransformOutput{Messages: []message.Message{{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "seed"}}}}})
	require.NoError(t, err)
	require.Len(t, transformed.Messages, 2)
	require.Equal(t, "persistent compacted history", transformed.Messages[1].Content().Text)

	// Test again to verify the persistent process stays alive.
	transformed2, err := TriggerChatMessagesTransform(context.Background(), ChatMessagesTransformInput{
		SessionID: "session-1",
		Agent:     "coder",
		Purpose:   ChatTransformPurposeRequest,
	}, ChatMessagesTransformOutput{Messages: []message.Message{{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "seed2"}}}}})
	require.NoError(t, err)
	require.Len(t, transformed2.Messages, 2)
	require.Equal(t, "persistent compacted history", transformed2.Messages[1].Content().Text)

	// Test session_compacting hook.
	compacting, err := TriggerSessionCompacting(context.Background(), SessionCompactingInput{
		SessionID: "session-1",
		Agent:     "coder",
		Purpose:   ChatTransformPurposeRecover,
	}, SessionCompactingOutput{})
	require.NoError(t, err)
	require.Equal(t, []string{"persistent compact note"}, compacting.Context)
	require.Equal(t, "persistent compact prompt", compacting.Prompt)

	// Test chat_system_transform hook.
	systemTransformed, err := TriggerChatSystemTransform(context.Background(), ChatSystemTransformInput{
		SessionID: "session-1",
		Agent:     "coder",
		Purpose:   ChatTransformPurposeRequest,
	}, ChatSystemTransformOutput{System: []string{"base-system"}, Prefix: "base-prefix"})
	require.NoError(t, err)
	require.Equal(t, []string{"base-system"}, systemTransformed.System)
	require.Equal(t, "base-prefix", systemTransformed.Prefix)

	// Close the persistent plugin.
	Close(context.Background())
}

func TestPersistentPluginShutdownRespectsContext(t *testing.T) {
	cfg := resolvedCommandPluginConfig{
		name:    "persistent-hanging-test",
		command: os.Args[0],
		args:    []string{"-test.run=TestPersistentPluginShutdownHelperProcess"},
		env:     []string{"GO_WANT_PERSISTENT_PLUGIN_SHUTDOWN_HELPER=1"},
		cwd:     t.TempDir(),
		mode:    commandPluginModePersistent,
	}

	mgr, err := newPersistentPluginManager(context.Background(), cfg)
	require.NoError(t, err)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	err = mgr.shutdown(shutdownCtx)
	elapsed := time.Since(start)

	require.Error(t, err)
	require.ErrorContains(t, err, context.DeadlineExceeded.Error())
	require.Less(t, elapsed, time.Second)
}

func TestPersistentPluginShutdownHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_PERSISTENT_PLUGIN_SHUTDOWN_HELPER") != "1" {
		return
	}

	_, _ = io.Copy(io.Discard, os.Stdin)
	for {
		time.Sleep(time.Second)
	}
}
