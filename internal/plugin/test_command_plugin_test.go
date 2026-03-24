package plugin

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/log"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

func TestTestCommandPlugin(t *testing.T) {
	Reset()
	defer Reset()

	workingDir := t.TempDir()
	t.Cleanup(func() {
		require.NoError(t, log.ResetForTesting())
	})
	configDir := filepath.Join(workingDir, ".crush")
	pluginsDir := filepath.Join(configDir, "plugins", "test-plugin")
	require.NoError(t, os.MkdirAll(pluginsDir, 0o755))

	// Create a simple Node.js script that keeps only the last 2 messages
	script := `const protocolVersion = 1;

async function main() {
  const raw = await readStdin();
  const request = JSON.parse(raw);
  
  if (request.version !== protocolVersion) {
    return writeResponse({ error: 'unsupported protocol version: ' + request.version });
  }

  const input = request.input || {};
  const output = request.output || {};

  if (request.event === "chat_messages_transform") {
    return handleChatMessagesTransform(input, output);
  }
  
  return writeResponse({ output });
}

async function handleChatMessagesTransform(input, output) {
  const messages = Array.isArray(output.messages) ? output.messages : [];
  
  if (messages.length > 2) {
    const lastTwoMessages = messages.slice(-2);
    return writeResponse({ 
      output: { 
        ...output, 
        messages: lastTwoMessages 
      } 
    });
  }
  
  return writeResponse({ output });
}

function readStdin() {
  return new Promise((resolve, reject) => {
    const chunks = [];
    process.stdin.on('data', (chunk) => chunks.push(chunk));
    process.stdin.on('end', () => resolve(Buffer.concat(chunks).toString('utf8')));
    process.stdin.on('error', reject);
  });
}

function writeResponse(response) {
  process.stdout.write(JSON.stringify(response));
}

main().catch((error) => {
  writeResponse({ error: error instanceof Error ? error.message : String(error) });
  process.exitCode = 1;
});`

	require.NoError(t, os.WriteFile(filepath.Join(pluginsDir, "index.js"), []byte(script), 0o644))

	// Create plugin.json
	pluginJSON := map[string]any{
		"name":    "test-plugin",
		"version": "1.0.0",
		"command": "node",
		"args":    []string{"index.js"},
		"hooks":   []string{"chat_messages_transform"},
	}
	pluginJSONBytes, err := json.Marshal(pluginJSON)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(pluginsDir, "plugin.json"), pluginJSONBytes, 0o644))

	// Initialize config and plugin
	store, err := config.Init(workingDir, "", false)
	require.NoError(t, err)

	err = Init(context.Background(), PluginInput{Config: store, WorkingDir: workingDir})
	require.NoError(t, err)

	// Create test messages
	testMessages := []message.Message{
		{ID: "msg-1", Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "Message 1"}}},
		{ID: "msg-2", Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "Message 2"}}},
		{ID: "msg-3", Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "Message 3"}}},
		{ID: "msg-4", Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "Message 4"}}},
		{ID: "msg-5", Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "Message 5"}}},
	}

	// Trigger the transform
	transformed, err := TriggerChatMessagesTransform(context.Background(), ChatMessagesTransformInput{
		SessionID: "test-session",
		Agent:     "coder",
	}, ChatMessagesTransformOutput{Messages: testMessages})

	require.NoError(t, err)
	require.Len(t, transformed.Messages, 2)
	require.Equal(t, "msg-4", transformed.Messages[0].ID)
	require.Equal(t, "msg-5", transformed.Messages[1].ID)
}
