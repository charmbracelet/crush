package pinentry

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

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
// Prompter means credentials cannot be collected in-process, in which
// case GPG runs as invoked and the agent's own pinentry asks.
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
// agent does not already have it cached, and retrying on a bad one. When
// loopback pinentry is unavailable, GPG runs as invoked so the agent's
// own pinentry collects the credential.
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

	stdin, cleanup, err := spoolStdin(opts.Stdin)
	if err != nil {
		return err
	}
	defer cleanup()

	// Only the final attempt's output reaches the caller: a failed
	// attempt can leave partial output behind (a clearsign header, say)
	// that would corrupt the real result.
	// ponytail: output is held in memory; spool it to a temp file if
	// large decrypts through Crush ever matter.
	aio := &attemptIO{stdin: stdin}
	defer aio.replay(opts.Stdout, opts.Stderr)

	// Probe: run with loopback and an empty passphrase. This succeeds
	// immediately when the key is unprotected or the agent already has
	// the credential cached.
	st := newStatus()
	probeErr := r.exec(ctx, bin, opts, aio, "", st)
	if probeErr == nil {
		return nil
	}
	if !st.credentialRequired() {
		if st.unsupported() {
			return aio.plain(ctx, bin, opts)
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
				return aio.plain(ctx, bin, opts)
			}
			return perr
		}
		st = newStatus()
		err = r.exec(ctx, bin, opts, aio, secret, st)
		if err == nil {
			r.storeCredential(st.cacheKey(), secret)
			return nil
		}
		if st.unsupported() && !st.credentialRequired() {
			return aio.plain(ctx, bin, opts)
		}
		if !st.credentialRequired() {
			return err
		}
		// Never replay a rejected secret from the cache: a stale security
		// key PIN would burn the card's retry counter.
		r.cache.forget(st.cacheKey())
		lastErr = err
	}
	return lastErr
}

// collectCredential returns a credential for the current attempt,
// preferring the optional cache on the first attempt.
func (r *Runner) collectCredential(ctx context.Context, st *status, attempt int, lastErr error) (string, error) {
	if attempt == 1 {
		if key := st.cacheKey(); key != "" {
			if secret, ok := r.cache.get(key); ok {
				return secret, nil
			}
		}
	}
	req := PromptRequest{
		Key:        st.key(),
		Kind:       classifyKind(st),
		RetryCount: attempt - 1,
	}
	req.Prompt = promptMessage(req.Kind)
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

// exec runs a single gpg attempt through loopback pinentry. An empty
// passphrase is the probe (the operation succeeds if the credential is
// cached or the key is unprotected). The secret is written over a
// dedicated pipe (fd 3) as `--passphrase-fd 3`.
func (r *Runner) exec(ctx context.Context, bin string, opts RunOptions, aio *attemptIO, passphrase string, st *status) error {
	// The passphrase pipe is always wired up, even for the empty probe:
	// gpg is told to read fd 3 and would fail with a bad-descriptor
	// error if it were missing. The write is best-effort in a goroutine
	// so a gpg that exits early (or never reads fd 3) cannot block.
	pr, pw, err := os.Pipe()
	if err != nil {
		return err
	}
	defer pr.Close()
	go func() {
		_, _ = io.WriteString(pw, passphrase+"\n")
		_ = pw.Close()
	}()

	// The loopback flags go first: gpg stops parsing options at the
	// first non-option argument, so flags after a file operand would be
	// taken as more file names.
	args := append([]string{"--batch", "--pinentry-mode", "loopback", "--passphrase-fd", passphraseFD()}, opts.Args[1:]...)
	return aio.run(ctx, bin, args, opts, st, pr)
}

// attemptIO is the stdio shared by the gpg attempts of one [Runner.Run]:
// the spooled stdin every attempt re-reads (the caller's reader is
// consumed once), and the output of the latest attempt.
type attemptIO struct {
	stdin          *os.File
	stdout, stderr bytes.Buffer
}

// run executes one gpg attempt with the given arguments (argv[0]
// excluded), replacing the previous attempt's output. A non-nil
// passphrase file is passed to gpg as fd 3.
func (a *attemptIO) run(ctx context.Context, bin string, args []string, opts RunOptions, st *status, passphrase *os.File) error {
	if _, err := a.stdin.Seek(0, io.SeekStart); err != nil {
		return err
	}
	a.stdout.Reset()
	a.stderr.Reset()

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = opts.Env
	cmd.Dir = opts.Dir
	cmd.Stdin = a.stdin
	cmd.Stdout = &a.stdout
	cmd.Stderr = statusWriter{inner: &a.stderr, st: st}
	if passphrase != nil {
		cmd.ExtraFiles = []*os.File{passphrase}
	}
	prepareProcess(cmd)

	if err := cmd.Start(); err != nil {
		return err
	}
	stopKill := watchCancel(ctx, cmd)
	err := cmd.Wait()
	stopKill()
	if err == nil {
		return nil
	}
	if ee, ok := errors.AsType[*exec.ExitError](err); ok {
		return &ExitError{Code: ee.ExitCode(), Stderr: st.excerpt()}
	}
	return err
}

// plain runs gpg exactly as the caller invoked it, for when loopback
// pinentry is unavailable: the agent's own pinentry collects the
// credential.
func (a *attemptIO) plain(ctx context.Context, bin string, opts RunOptions) error {
	return a.run(ctx, bin, opts.Args[1:], opts, newStatus(), nil)
}

// replay writes the final attempt's output to the caller.
func (a *attemptIO) replay(stdout, stderr io.Writer) {
	if stdout != nil {
		_, _ = stdout.Write(a.stdout.Bytes())
	}
	if stderr != nil {
		_, _ = stderr.Write(a.stderr.Bytes())
	}
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

// cacheKey returns a stable identity for the key being unlocked, or ""
// when GPG reported none. The fingerprint comes first: GPG 2.1+ reports
// KEY_CONSIDERED but omits USERID_HINT when the passphrase comes from a
// file descriptor.
func (s *status) cacheKey() string {
	k := s.key()
	return cmp.Or(k.Fingerprint, k.KeyID, k.Holder)
}

// key parses the key details GPG reported for the credential. It reads
// the status lines GPG emits alongside a credential request:
// USERID_HINT (holder), NEED_PASSPHRASE (key ID), KEY_CONSIDERED
// (fingerprint), and CARDCTRL (smartcard serial number).
func (s *status) key() Key {
	var k Key
	for _, line := range strings.Split(s.body(), "\n") {
		fs := strings.Fields(line)
		if len(fs) < 3 {
			continue
		}
		switch fs[1] {
		case "USERID_HINT":
			if k.KeyID == "" {
				k.KeyID = fs[2]
			}
			if len(fs) > 3 {
				k.Holder = strings.Join(fs[3:], " ")
			}
		case "NEED_PASSPHRASE":
			if k.KeyID == "" {
				k.KeyID = fs[2]
			}
		case "KEY_CONSIDERED":
			k.Fingerprint = fs[2]
		case "CARDCTRL":
			// CARDCTRL <what> [<serialno>]: the serial only rides
			// along for the values that concern a specific card.
			if len(fs) > 3 {
				k.CardNumber = fs[3]
			}
		}
	}
	return k
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
		"--sign": true, "--clearsign": true, "--clear-sign": true,
		"--detach-sign": true, "--decrypt": true, "--symmetric": true,
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
