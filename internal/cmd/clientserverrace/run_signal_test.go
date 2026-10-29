package clientserverrace_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunStopsOnTerminationSignal guards against `crush run` ignoring
// SIGTERM and SIGHUP, which the root command catches, and streaming on
// until its agent finishes.
func TestRunStopsOnTerminationSignal(t *testing.T) {
	bin := crushBinary(t)
	for _, clientServer := range []bool{false, true} {
		for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP} {
			t.Run(fmt.Sprintf("client-server=%t/%v", clientServer, sig), func(t *testing.T) {
				streaming, cancelled, url := startSlowLLM(t)
				sb := newSandbox(t, bin,
					"CRUSH_CLIENT_SERVER="+strconv.FormatBool(clientServer),
					"CRUSH_DISABLE_DEFAULT_PROVIDERS=1")
				writeMockConfig(t, sb, url)
				if clientServer {
					sb.start(t, "server", "server", "--host", sb.host)
					if !assert.Eventually(t, func() bool { return pingHealth(sb.socket) == nil },
						30*time.Second, 50*time.Millisecond) {
						t.Fatalf("server never became healthy\n%s", sb.output("server"))
					}
				}

				run := sb.start(t, "run", "--host", sb.host, "--cwd", sb.project, "run", "hello")
				select {
				case <-streaming:
				case <-run.done:
					t.Fatalf("run exited before streaming: %v\n%s", run.err, sb.output("run"))
				case <-time.After(time.Minute):
					t.Fatalf("run never started streaming\n%s", sb.output("run"))
				}

				require.NoError(t, run.cmd.Process.Signal(sig))
				for what, ch := range map[string]chan struct{}{"run exited": run.done, "turn cancelled": cancelled} {
					select {
					case <-ch:
					case <-time.After(10 * time.Second):
						t.Fatalf("no %q within 10s of %v", what, sig)
					}
				}
			})
		}
	}
}

// startSlowLLM serves OpenAI-compatible chat completions whose turns stream
// until the request goes away. It returns channels closed once a turn is
// streaming and once that turn's request is gone, and its base URL.
func startSlowLLM(t *testing.T) (streaming, cancelled chan struct{}, url string) {
	t.Helper()
	const chunk = `data: {"id":"c","object":"chat.completion.chunk","created":0,` +
		`"model":"mock-model","choices":[{"index":0,"delta":%s,"finish_reason":%s}]}` + "\n\n"
	streaming, cancelled = make(chan struct{}), make(chan struct{})
	stop := make(chan struct{})
	var streamingOnce, cancelledOnce sync.Once
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		// Title generation runs alongside the turn; answer it at once.
		if strings.Contains(string(body), "Generate a concise title") {
			fmt.Fprintf(w, chunk, `{"role":"assistant","content":"Title"}`, "null")
			fmt.Fprintf(w, chunk, `{}`, `"stop"`)
			fmt.Fprint(w, "data: [DONE]\n\n")
			return
		}
		for {
			fmt.Fprintf(w, chunk, `{"role":"assistant","content":"tok "}`, "null")
			w.(http.Flusher).Flush()
			streamingOnce.Do(func() { close(streaming) })
			select {
			case <-r.Context().Done():
				cancelledOnce.Do(func() { close(cancelled) })
				return
			case <-stop:
				return
			case <-time.After(100 * time.Millisecond):
			}
		}
	}))
	t.Cleanup(func() {
		close(stop)
		srv.Close()
	})
	return streaming, cancelled, srv.URL + "/v1"
}

// writeMockConfig points the sandbox's global config at the LLM at url.
func writeMockConfig(t *testing.T, sb *sandbox, url string) {
	t.Helper()
	dir := filepath.Join(sb.dir, "xdg_config_home", "crush")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	cfg := fmt.Sprintf(`{
  "providers": {"mock": {"type": "openai-compat", "base_url": %q, "api_key": "test",
    "models": [{"id": "mock-model", "name": "Mock", "context_window": 8192, "default_max_tokens": 128}]}},
  "models": {"large": {"provider": "mock", "model": "mock-model"},
    "small": {"provider": "mock", "model": "mock-model"}}
}`, url)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "crush.json"), []byte(cfg), 0o600))
}
