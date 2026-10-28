package clientserverrace_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// signalExitTimeout bounds how long `crush run` may take to exit, and to
// stop its agent's turn, after a termination signal. A run that honours the
// signal needs well under a second; one that ignores it streams until the
// agent finishes, which the LLM below never lets happen.
const signalExitTimeout = 10 * time.Second

// TestRunStopsOnTerminationSignal checks that `crush run` stops promptly,
// along with its agent's turn, on every termination signal in local and
// client/server mode: Ctrl+C, SIGTERM (service managers, CI cancellation,
// timeout(1); Go also delivers a closed Windows console as SIGTERM) and
// SIGHUP (a closed terminal). The root command catches all of them, so a
// run that does not follow the root command's context never hears of
// SIGTERM or SIGHUP and carries on until the agent finishes.
func TestRunStopsOnTerminationSignal(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping crush run signal test in -short mode")
	}
	// Windows cannot send these signals to another process.
	if runtime.GOOS == "windows" {
		t.Skip("skipping signal delivery test on windows")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("skipping: 'go' not available on PATH")
	}

	bin := buildCrushBinary(t, repoRootFromTest(t))

	signals := []struct {
		name string
		sig  syscall.Signal
	}{
		{"SIGINT", syscall.SIGINT},
		{"SIGTERM", syscall.SIGTERM},
		{"SIGHUP", syscall.SIGHUP},
	}
	for _, clientServer := range []bool{false, true} {
		mode := "local"
		if clientServer {
			mode = "client-server"
		}
		for _, s := range signals {
			t.Run(mode+"/"+s.name, func(t *testing.T) {
				llm := startSlowLLM(t)
				sb := newSignalSandbox(t, llm.url, clientServer)
				if clientServer {
					sb.startServer(t, bin)
				}

				run := sb.start(t, bin, "run", "--host", sb.host, "--cwd", sb.project, "run", "hello")
				select {
				case <-llm.streaming:
				case <-run.done:
					t.Fatalf("crush run exited before its agent started streaming: %v\n%s", run.err, sb.output("run"))
				case <-time.After(time.Minute):
					t.Fatalf("crush run's agent never started streaming\n%s", sb.output("run"))
				}

				require.NoError(t, run.cmd.Process.Signal(s.sig))
				select {
				case <-run.done:
				case <-time.After(signalExitTimeout):
					t.Fatalf("crush run still running %s after %s", signalExitTimeout, s.name)
				}
				select {
				case <-llm.cancelled:
				case <-time.After(signalExitTimeout):
					t.Fatalf("crush run exited on %s but its agent's turn kept streaming", s.name)
				}
			})
		}
	}
}

// slowLLM is an OpenAI-compatible chat completions endpoint whose agent
// turns stream a token every 100ms until the request goes away, so a
// `crush run` against it only ends when something stops it.
type slowLLM struct {
	url       string
	streaming chan struct{} // Closed once an agent turn is streaming.
	cancelled chan struct{} // Closed once that turn's request is gone.
}

func startSlowLLM(t *testing.T) *slowLLM {
	t.Helper()

	const chunk = `{"id":"c","object":"chat.completion.chunk","created":0,"model":"mock-model","choices":[{"index":0,"delta":%s,"finish_reason":%s}]}`

	llm := &slowLLM{
		streaming: make(chan struct{}),
		cancelled: make(chan struct{}),
	}
	stop := make(chan struct{})
	var streamingOnce, cancelledOnce sync.Once
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		// Title generation runs alongside the turn; answer it at once.
		if strings.Contains(string(body), "Generate a concise title") {
			fmt.Fprintf(w, "data: "+chunk+"\n\n", `{"role":"assistant","content":"Title"}`, "null")
			fmt.Fprintf(w, "data: "+chunk+"\n\n", `{}`, `"stop"`)
			fmt.Fprint(w, "data: [DONE]\n\n")
			return
		}
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			fmt.Fprintf(w, "data: "+chunk+"\n\n", `{"role":"assistant","content":"tok "}`, "null")
			w.(http.Flusher).Flush()
			streamingOnce.Do(func() { close(llm.streaming) })
			select {
			case <-r.Context().Done():
				cancelledOnce.Do(func() { close(llm.cancelled) })
				return
			case <-stop:
				return
			case <-ticker.C:
			}
		}
	}))
	t.Cleanup(func() {
		close(stop)
		srv.Close()
	})
	llm.url = srv.URL + "/v1"
	return llm
}

// signalSandbox isolates one crush client, and its server in client/server
// mode, from the user's real crush state and default socket.
type signalSandbox struct {
	dir     string
	project string
	socket  string
	host    string
	env     []string
}

func newSignalSandbox(t *testing.T, llmURL string, clientServer bool) *signalSandbox {
	t.Helper()

	dir := t.TempDir()
	// Keep the socket path under darwin's 104-byte sockaddr_un limit,
	// which a t.TempDir() path can exceed.
	sockDir, err := os.MkdirTemp("/tmp", "crush-sig-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(sockDir) })

	sb := &signalSandbox{
		dir:     dir,
		project: filepath.Join(dir, "project"),
		socket:  filepath.Join(sockDir, "crush.sock"),
	}
	sb.host = "unix://" + sb.socket

	var (
		home       = filepath.Join(dir, "home")
		configHome = filepath.Join(dir, "config")
		dataHome   = filepath.Join(dir, "data")
		cacheHome  = filepath.Join(dir, "cache")
	)
	for _, d := range []string{sb.project, home, filepath.Join(configHome, "crush"), dataHome, cacheHome} {
		require.NoError(t, os.MkdirAll(d, 0o700))
	}

	cfg := fmt.Sprintf(`{
  "providers": {
    "mock": {
      "type": "openai-compat",
      "base_url": %q,
      "api_key": "test",
      "models": [{"id": "mock-model", "name": "Mock", "context_window": 8192, "default_max_tokens": 128}]
    }
  },
  "models": {
    "large": {"provider": "mock", "model": "mock-model"},
    "small": {"provider": "mock", "model": "mock-model"}
  }
}`, llmURL)
	require.NoError(t, os.WriteFile(filepath.Join(configHome, "crush", "crush.json"), []byte(cfg), 0o600))

	sb.env = append(
		os.Environ(),
		"HOME="+home,
		"XDG_CONFIG_HOME="+configHome,
		"XDG_DATA_HOME="+dataHome,
		"XDG_CACHE_HOME="+cacheHome,
		"CRUSH_GLOBAL_CONFIG="+filepath.Join(configHome, "crush"),
		"CRUSH_GLOBAL_DATA="+filepath.Join(dataHome, "crush"),
		"CRUSH_CACHE_DIR="+filepath.Join(cacheHome, "crush"),
		"CRUSH_DISABLE_PROVIDER_AUTO_UPDATE=1",
		"CRUSH_DISABLE_DEFAULT_PROVIDERS=1",
		"CRUSH_DISABLE_METRICS=1",
		"CRUSH_CLIENT_SERVER="+strconv.FormatBool(clientServer),
	)
	return sb
}

// signalProc is a crush process started by [signalSandbox.start].
type signalProc struct {
	cmd  *exec.Cmd
	done chan struct{}
	err  error // Set once done is closed.
}

// start runs the crush binary with args, logging its output to name.log in
// the sandbox. The process is killed and reaped when the test ends.
func (sb *signalSandbox) start(t *testing.T, bin, name string, args ...string) *signalProc {
	t.Helper()

	log, err := os.Create(filepath.Join(sb.dir, name+".log"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = log.Close() })

	cmd := exec.CommandContext(t.Context(), bin, args...)
	cmd.Env = sb.env
	cmd.Stdout = log
	cmd.Stderr = log
	require.NoError(t, cmd.Start())

	p := &signalProc{cmd: cmd, done: make(chan struct{})}
	go func() {
		p.err = cmd.Wait()
		close(p.done)
	}()
	// The test's context is cancelled, which kills the process, just
	// before cleanups run.
	t.Cleanup(func() { <-p.done })
	return p
}

// startServer runs the server the client connects to in client/server
// mode, instead of letting the client spawn a detached one, so the test
// owns its lifetime.
func (sb *signalSandbox) startServer(t *testing.T, bin string) {
	t.Helper()

	srv := sb.start(t, bin, "server", "server", "--host", sb.host)
	deadline := time.Now().Add(30 * time.Second)
	for pingHealth(sb.socket) != nil {
		if time.Now().After(deadline) {
			t.Fatalf("crush server never became healthy\n%s", sb.output("server"))
		}
		select {
		case <-srv.done:
			t.Fatalf("crush server exited: %v\n%s", srv.err, sb.output("server"))
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// output returns what the named process has logged so far.
func (sb *signalSandbox) output(name string) string {
	out, _ := os.ReadFile(filepath.Join(sb.dir, name+".log"))
	return string(out)
}
