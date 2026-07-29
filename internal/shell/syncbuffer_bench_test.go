package shell

import (
	"bytes"
	"testing"
)

// BenchmarkSyncBufferString measures the cost the jobs read path must not
// pay per frame.
//
// String() materialises the whole retained window as a Go string, so at the
// default 1 MiB head + 1 MiB tail it copies ~2 MiB every call. That is
// inherent to returning a string, not a regression — which is exactly why
// the jobs list payload (proto.BackgroundJob) carries no output and the
// jobs dialog never calls it. Output is fetched one job at a time by the
// job_output tool instead.
func BenchmarkSyncBufferString(b *testing.B) {
	sb := newSyncBuffer()
	chunk := bytes.Repeat([]byte("x"), 64*1024)
	for range 64 { // 4 MiB written, 2 MiB retained
		if _, err := sb.Write(chunk); err != nil {
			b.Fatal(err)
		}
	}

	b.ReportAllocs()
	for b.Loop() {
		_ = sb.String()
	}
}
