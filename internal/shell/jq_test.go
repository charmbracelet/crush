package shell

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

// TestJQ_CtxCancel verifies that handleJQ polls ctx during iteration and
// returns ctx.Err() (not an interp.ExitStatus) when the context is
// cancelled. This is what lets hook timeouts interrupt long-running jq
// filters rather than waiting for the iterator to terminate naturally.
func TestJQ_CtxCancel(t *testing.T) {
	t.Parallel()

	// `range(N)` generates a large stream of values. With a slurped input
	// the filter produces all N values in sequence; ctx cancellation
	// between values should short-circuit the loop.
	const filter = "range(10000000)"
	stdin := strings.NewReader("null\n")

	ctx, cancel := context.WithCancel(t.Context())
	// Cancel almost immediately so we catch the next iteration check.
	cancel()

	err := handleJQ(ctx, []string{"jq", filter}, stdin, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected ctx cancel error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

// TestJQ_CtxCancel_DuringFilter verifies cancellation mid-stream: ctx is
// cancelled after jq has started producing output, and the loop must
// observe the cancel on the next iteration rather than running to
// completion.
func TestJQ_CtxCancel_DuringFilter(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	// 100M values; without ctx polling this would take many seconds to
	// fully emit. With ctx polling the loop exits shortly after the
	// deadline.
	stdin := strings.NewReader("null\n")
	var stdout, stderr bytes.Buffer

	start := time.Now()
	err := handleJQ(ctx, []string{"jq", "-c", "range(100000000)"}, stdin, &stdout, &stderr)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected ctx timeout error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
	// Allow generous slack for slow CI; the important invariant is that we
	// don't run all 100M iterations (which would take orders of magnitude
	// longer than 1s).
	if elapsed > time.Second {
		t.Fatalf("handleJQ took %v after 50ms timeout; ctx polling is not tight enough", elapsed)
	}
}

// slowReader serves bytes in small chunks with a fixed delay between
// Read calls. It never blocks indefinitely — each Read returns after
// chunkDelay — so cancellation must be observed via ctxReader's ctx
// check, not by the underlying reader itself. That isolates the
// behavior we want to test: the wrapper polling ctx between chunks.
type slowReader struct {
	remaining  []byte
	chunk      int
	chunkDelay time.Duration
}

func (s *slowReader) Read(p []byte) (int, error) {
	if len(s.remaining) == 0 {
		return 0, io.EOF
	}
	time.Sleep(s.chunkDelay)
	n := min(len(p), min(s.chunk, len(s.remaining)))
	copy(p, s.remaining[:n])
	s.remaining = s.remaining[n:]
	return n, nil
}

// TestJQ_CtxCancel_MidReadAll verifies that ctx cancellation observed
// *during* io.ReadAll — after several chunks have already been consumed
// — short-circuits the read via ctxReader, rather than draining the
// whole source. This is the guarantee the hook runner relies on when
// it feeds a large bytes.Reader payload.
//
// The reader serves bytes in 512-byte chunks with a 5ms gap between
// reads. ctx is cancelled after ~50ms, so several chunks have already
// been read when ctxReader first observes the cancellation. The test
// asserts that (a) we got a context.Canceled error and (b) the call
// returned well before the reader would have been fully drained.
func TestJQ_CtxCancel_MidReadAll(t *testing.T) {
	t.Parallel()

	const (
		size       = 64 * 1024 * 1024 // 64 MiB
		chunk      = 512
		chunkDelay = 5 * time.Millisecond
	)
	// At 512 bytes / 5ms, draining 64 MiB would take ~11 minutes. Any
	// return within a second proves cancel was observed mid-stream, not
	// after EOF.
	reader := &slowReader{
		remaining:  bytes.Repeat([]byte("a"), size),
		chunk:      chunk,
		chunkDelay: chunkDelay,
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// Cancel after enough time that several Read calls have completed
	// and io.ReadAll is actively consuming the source.
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := handleJQ(ctx, []string{"jq", "-R", "."}, reader, io.Discard, io.Discard)
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	// Generous slack for slow CI; the invariant is orders-of-magnitude
	// faster than draining the full source.
	if elapsed > time.Second {
		t.Fatalf("mid-ReadAll cancel took %v; ctxReader is not polling between chunks", elapsed)
	}
	// Sanity check: we should have been cancelled mid-stream, not
	// before any reads happened. If remaining == size, cancel fired so
	// early nothing was consumed — that's a fast-fail path, not the
	// mid-read guarantee we want to verify.
	consumed := size - len(reader.remaining)
	if consumed == 0 {
		t.Fatal("reader was never read from; test did not exercise mid-ReadAll cancel")
	}
	if consumed >= size {
		t.Fatal("reader was fully drained; cancel was not observed mid-read")
	}
}

// failOnReadReader fails the test if Read is ever called. It proves that a
// code path short-circuited before touching the input, without relying on
// wall-clock timing.
type failOnReadReader struct {
	t *testing.T
}

func (r failOnReadReader) Read(p []byte) (int, error) {
	r.t.Error("Read called; outer guard did not short-circuit before io.ReadAll")
	return 0, io.EOF
}

// TestJQ_CtxCancel_PreCancel verifies the fast-fail path: a ctx already
// cancelled before handleJQ is called returns context.Canceled
// immediately via the outer-loop guard, never entering io.ReadAll.
// Complements TestJQ_CtxCancel_MidReadAll.
//
// The invariant — not wall-clock timing — is what proves the guard fired:
// the input reader fails the test if it is ever read from. A previous
// version asserted a 100ms ceiling, which measured scheduler latency rather
// than the guard and flaked on contended Windows CI runners under -race.
func TestJQ_CtxCancel_PreCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := handleJQ(ctx, []string{"jq", "-R", "."},
		failOnReadReader{t: t},
		io.Discard, io.Discard)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

// TestJQ_Success confirms the ctx-aware refactor did not regress the
// success path.
func TestJQ_Success(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	err := handleJQ(
		t.Context(),
		[]string{"jq", "-c", ".a"},
		strings.NewReader(`{"a":1}`),
		&stdout, io.Discard,
	)
	if err != nil {
		t.Fatalf("handleJQ returned error: %v", err)
	}
	if got := stdout.String(); got != "1\n" {
		t.Fatalf("stdout = %q, want %q", got, "1\n")
	}
}

// TestJQ_RawInputTrailingNewline verifies that `jq -R` treats a terminating
// newline as a line terminator, not a separator: "a\nb\n" is two lines and
// must not produce a spurious trailing empty string. Interior empty lines are
// preserved.
func TestJQ_RawInputTrailingNewline(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name, in, want string
	}{
		{"trailing newline", "a\nb\n", "\"a\"\n\"b\"\n"},
		{"interior empty preserved", "a\n\nb\n", "\"a\"\n\"\"\n\"b\"\n"},
		{"single line with newline", "a\n", "\"a\"\n"},
		{"no trailing newline", "a\nb", "\"a\"\n\"b\"\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var stdout bytes.Buffer
			err := handleJQ(
				t.Context(),
				[]string{"jq", "-R", "."},
				strings.NewReader(tc.in),
				&stdout, io.Discard,
			)
			if err != nil {
				t.Fatalf("handleJQ returned error: %v", err)
			}
			if got := stdout.String(); got != tc.want {
				t.Fatalf("stdout = %q, want %q", got, tc.want)
			}
		})
	}
}
