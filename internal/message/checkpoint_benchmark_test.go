package message

import (
	"strings"
	"testing"
	"time"

	"github.com/asx8678/ultra/internal/db"
	"github.com/asx8678/ultra/internal/session"
)

func BenchmarkMessageCheckpoint_LongResponse(b *testing.B) {
	dataDir := b.TempDir()
	conn, err := db.Connect(b.Context(), dataDir)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = db.Release(dataDir) })
	q := db.New(conn)
	sessions := session.NewService(q, conn)
	sess, err := sessions.Create(b.Context(), "benchmark")
	if err != nil {
		b.Fatal(err)
	}
	svc := NewService(q, WithDebounce(time.Hour))
	payload := strings.Repeat("streamed response ", 16_384)
	b.ResetTimer()
	for b.Loop() {
		msg, createErr := svc.Create(b.Context(), sess.ID, CreateMessageParams{Role: Assistant})
		if createErr != nil {
			b.Fatal(createErr)
		}
		msg.AppendContent(payload)
		if updateErr := svc.Update(b.Context(), msg); updateErr != nil {
			b.Fatal(updateErr)
		}
		if flushErr := svc.Flush(b.Context(), msg.ID); flushErr != nil {
			b.Fatal(flushErr)
		}
	}
}
