package pinentry

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// ErrFallback reports that the integrated pinentry could not drive a GPG
// invocation (for example the agent refuses loopback mode, or no one is
// listening for a credential prompt). Callers should run GPG normally and
// let the terminal-handover watcher handle the external pinentry.
var ErrFallback = errors.New("integrated pinentry unavailable; falling back to external pinentry")

// maxPassphraseAttempts is the number of times a bad credential is
// re-prompted before giving up, matching GPG's own retry count.
const maxPassphraseAttempts = 3

// GPGBinaries is the set of gpg executables the shell middleware
// intercepts.
var GPGBinaries = map[string]struct{}{
	"gpg":  {},
	"gpg2": {},
}

// ExitError describes a non-zero GPG exit where the exit code matters to
// the caller (it is surfaced as the shell command's exit status).
type ExitError struct {
	// Code is the process exit status.
	Code int
	// Stderr is a trimmed excerpt of the captured GPG stderr.
	Stderr string
}

// Error implements error.
func (e *ExitError) Error() string {
	if e.Stderr == "" {
		return fmt.Sprintf("gpg exited with status %d", e.Code)
	}
	return fmt.Sprintf("gpg exited with status %d: %s", e.Code, e.Stderr)
}

// RunOptions carries everything needed to execute a single GPG command.
type RunOptions struct {
	// Args is the full argv including the gpg executable as argv[0].
	Args []string
	// Env is the environment for the gpg process.
	Env []string
	// Dir is the working directory, or "" for the current directory.
	Dir string
	// Stdin is the command's standard input (may be nil). It is spooled
	// to a temporary file so retry attempts after a bad credential can
	// re-read it.
	Stdin io.Reader
	// Stdout receives the command's standard output.
	Stdout io.Writer
	// Stderr receives the command's standard error.
	Stderr io.Writer
}

// Prompter supplies a credential for a PromptRequest.
type Prompter func(ctx context.Context, req PromptRequest) (string, error)

// Runner drives GPG invocations through loopback pinentry. A nil
// Prompter means credentials cannot be collected in-process (for the
// socket-backed subprocess wrapper), in which case any password prompt
// falls back to [ErrFallback].
type Runner struct {
	// Prompter collects a credential; may be nil.
	Prompter Prompter
	// CacheTimeout enables the in-memory credential cache when > 0.
	CacheTimeout time.Duration
	// GPGPath is the resolved gpg binary; overrides PATH lookup when set.
	GPGPath string

	cache *passphraseCache
}

func (r *Runner) initCache() {
	if r.CacheTimeout > 0 && r.cache == nil {
		r.cache = newPassphraseCache(r.CacheTimeout)
	}
}

// ClearCache zeroes and drops any cached credentials. It doubles as the
// integration shutdown hook.
func (r *Runner) ClearCache() {
	r.initCache()
	if r.cache != nil {
		r.cache.clear()
	}
}

// Run executes the GPG command, prompting for a credential only when the
// agent does not already have it cached, and retrying on a bad one.
// Returns [ErrFallback] so the caller can run plain GPG instead.
func (r *Runner) Run(ctx context.Context, opts RunOptions) error {
	if len(opts.Args) == 0 {
		return errors.New("pinentry: empty gpg argv")
	}

	r.initCache()

	bin := r.GPGPath
	if bin == "" {
		var err error
		bin, err = exec.LookPath(opts.Args[0])
		if err != nil {
			return err
		}
	}

	spooled, cleanup, err := spoolStdin(opts.Stdin)
	if err != nil {
		return err
	}
	defer cleanup()

	// Probe: run with loopback and an empty passphrase. This succeeds
	// immediately when the key is unprotected or the agent already has
	// the credential cached.
	st := newStatus()
	probeErr := r.exec(ctx, bin, opts, spooled, "", st)
	if probeErr == nil {
		return nil
	}
	if !st.credentialRequired() {
		if st.unsupported() {
			return ErrFallback
		}
		return probeErr
	}

	// The operation genuinely needs a credential the agent does not
	// have. Collect one and retry, re-prompting on a bad value.
	//
	// The probe error is deliberately not carried into the first prompt:
	// it reflects the empty-passphrase probe, not a user attempt, so
	// only genuine retries (a user-provided credential GPG rejected)
	// surface an error.
	var lastErr error
	for attempt := 1; attempt <= maxPassphraseAttempts; attempt++ {
		secret, perr := r.collectCredential(ctx, st, attempt, lastErr)
		if perr != nil {
			if errors.Is(perr, ErrNoPrompter) {
				return ErrFallback
			}
			return perr
		}
		st = newStatus()
		err = r.exec(ctx, bin, opts, spooled, secret, st)
		if err == nil {
			if key := st.userID(); key != "" {
				r.storeCredential(key, secret)
			}
			return nil
		}
		if st.unsupported() && !st.credentialRequired() {
			return ErrFallback
		}
		if !st.credentialRequired() {
			return err
		}
		lastErr = err
	}
	return lastErr
}

// collectCredential returns a credential for the current attempt,
// preferring the optional cache on the first attempt.
func (r *Runner) collectCredential(ctx context.Context, st *status, attempt int, lastErr error) (string, error) {
	if attempt == 1 {
		if key := st.userID(); key != "" {
			if secret, ok := r.cache.get(key); ok {
				return secret, nil
			}
		}
	}
	req := PromptRequest{
		KeyInfo:    st.userID(),
		Kind:       classifyKind(st),
		RetryCount: attempt - 1,
	}
	if req.Kind == KindPIN {
		req.Prompt = "Enter the PIN for your security key to continue"
	} else {
		req.Prompt = "Enter the passphrase for your GPG key to continue"
	}
	if lastErr != nil {
		req.Error = lastErr.Error()
	}
	if r.Prompter == nil {
		return "", ErrNoPrompter
	}
	return r.Prompter(ctx, req)
}

func (r *Runner) storeCredential(key, secret string) {
	r.initCache()
	if r.cache != nil {
		r.cache.set(key, secret)
	}
}

// exec runs a single gpg attempt. An empty passphrase is the probe (the
// operation succeeds if the credential is cached/unprotectd). The secret
// is written over a dedicated pipe (fd 3) as `--passphrase-fd 3`.
func (r *Runner) exec(ctx context.Context, bin string, opts RunOptions, stdin *os.File, passphrase string, st *status) error {
	args := slices.Clone(opts.Args)
	args[0] = bin
	args = append(args, "--batch", "--pinentry-mode", "loopback", "--passphrase-fd", passphraseFD())

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = opts.Env
	cmd.Dir = opts.Dir
	prepareProcess(cmd)

	if _, err := stdin.Seek(0, io.SeekStart); err != nil {
		return err
	}

	var out bytes.Buffer
	cmd.Stdin = stdin
	cmd.Stdout = firstWriter(opts.Stdout, &out)
	cmd.Stderr = statusWriter{inner: opts.Stderr, st: st}

	// The passphrase pipe is always wired up, even for the empty probe:
	// gpg is told to read fd 3 and would fail with a bad-descriptor
	// error if it were missing. The write is best-effort in a goroutine
	// so a gpg that exits early (or never reads fd 3) cannot block.
	pr, pw, err := os.Pipe()
	if err != nil {
		return err
	}
	defer pr.Close()
	cmd.ExtraFiles = []*os.File{pr}
	go func() {
		_, _ = io.WriteString(pw, passphrase+"\n")
		_ = pw.Close()
	}()

	if err := cmd.Start(); err != nil {
		return err
	}
	stopKill := watchCancel(ctx, cmd)
	err = cmd.Wait()
	stopKill()
	if err == nil {
		return nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return &ExitError{Code: ee.ExitCode(), Stderr: st.excerpt()}
	}
	return err
}

func firstWriter(a, b io.Writer) io.Writer {
	if a != nil {
		return a
	}
	return b
}

// status accumulates the stderr of a single gpg invocation so the
// calling flow can classify the outcome. It captures up to
// statusCaptureLimit bytes, which is ample for status/diagnostic lines
// (binary payloads go to stdout on the probe and on success).
type status struct {
	mu  sync.Mutex
	raw []byte
	cap int
}

const statusCaptureLimit = 8192

func newStatus() *status { return &status{} }

// statusWriter tees stderr through to the caller while recording it for
// classification.
type statusWriter struct {
	inner io.Writer
	st    *status
}

func (w statusWriter) Write(p []byte) (int, error) {
	w.st.record(p)
	if w.inner != nil {
		return w.inner.Write(p)
	}
	return len(p), nil
}

func (s *status) record(p []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cap < statusCaptureLimit {
		room := statusCaptureLimit - s.cap
		if len(p) > room {
			p = p[:room]
		}
		s.raw = append(s.raw, p...)
		s.cap += len(p)
	}
}

func (s *status) body() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return strings.TrimSpace(string(s.raw))
}

func (s *status) excerpt() string { return s.body() }

// userID returns a stable identity for the key being unlocked, from the
// USERID_HINT status line when present.
func (s *status) userID() string {
	for _, line := range strings.Split(s.body(), "\n") {
		fs := strings.Fields(line)
		if len(fs) >= 4 && fs[1] == "USERID_HINT" {
			return strings.Join(fs[3:], " ")
		}
	}
	return ""
}

// credentialRequired reports whether gpg rejected the attempt because of
// a missing or bad credential (so we should prompt and retry).
func (s *status) credentialRequired() bool {
	low := strings.ToLower(s.body())
	for _, m := range []string{
		"[gnupg:] need_passphrase",
		"[gnupg:] missing_passphrase",
		"[gnupg:] bad_passphrase",
		"bad passphrase",
		"bad pin",
		"invalid pin",
		"no passphrase given",
		"no passphrase was given",
	} {
		if strings.Contains(low, m) {
			return true
		}
	}
	return false
}

// unsupported reports whether the agent is unable to participate in
// loopback pinentry (so we must fall back to the external pinentry).
func (s *status) unsupported() bool {
	low := strings.ToLower(s.body())
	for _, m := range []string{
		"inappropriate ioctl",
		"no pinentry",
		"not allowed",
		"unknown option",
		"not supported",
	} {
		if strings.Contains(low, m) {
			return true
		}
	}
	return false
}

// classifyKind guesses whether the credential is a security key PIN from
// the gpg output; otherwise it is a passphrase.
func classifyKind(st *status) Kind {
	lower := strings.ToLower(st.body())
	if strings.Contains(lower, "pin") && !strings.Contains(lower, "passphrase") {
		return KindPIN
	}
	return KindPassphrase
}

// ShouldIntercept reports whether an external command should be routed
// through the integrated pinentry runner: a gpg binary performing a
// credential-relevant operation without its own pinentry management.
func ShouldIntercept(args []string) bool {
	if !IntegrationEnabled() {
		return false
	}
	if len(args) == 0 {
		return false
	}
	base := filepath.Base(args[0])
	if _, ok := GPGBinaries[base]; !ok {
		return false
	}
	// Never touch invocations that manage their own credential/pinentry.
	for _, a := range args[1:] {
		switch {
		case strings.HasPrefix(a, "--passphrase") || a == "--passphrase-fd" || a == "--passphrase-file":
			return false
		case a == "--pinentry-mode" || strings.HasPrefix(a, "--pinentry-mode="):
			return false
		}
	}
	return hasCredentialOperation(args)
}

// hasCredentialOperation reports whether args request a sign, decrypt,
// or symmetric-encrypt operation (the ones that can need a credential).
func hasCredentialOperation(args []string) bool {
	credentialLong := map[string]bool{
		"--sign": true, "--clearsign": true, "--detach-sign": true,
		"--decrypt": true, "--symmetric": true,
	}
	credentialShort := map[rune]bool{
		's': true, 'b': true, 'c': true, 'd': true,
	}
	for _, a := range args[1:] {
		switch {
		case strings.HasPrefix(a, "--"):
			if credentialLong[a] {
				return true
			}
		case strings.HasPrefix(a, "-") && len(a) > 1:
			for _, ch := range a[1:] {
				if credentialShort[ch] {
					return true
				}
			}
		}
	}
	return false
}

// spoolStdin copies the command's stdin into a temporary file so it can
// be re-read across retry attempts. Returns the file (positioned at 0)
// and a cleanup function.
func spoolStdin(r io.Reader) (*os.File, func(), error) {
	f, err := os.CreateTemp("", "crush-gpg-stdin-*")
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() {
		name := f.Name()
		_ = f.Close()
		_ = os.Remove(name)
	}
	if r != nil {
		if _, err := io.Copy(f, r); err != nil {
			cleanup()
			return nil, nil, err
		}
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		cleanup()
		return nil, nil, err
	}
	return f, cleanup, nil
}

// passphraseFD is the file descriptor used for the passphrase pipe;
// ExtraFiles[0] is assigned fd 3 in the child.
func passphraseFD() string { return "3" }
