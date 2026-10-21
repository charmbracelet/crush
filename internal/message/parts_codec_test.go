package message

import (
	"database/sql"
	"strings"
	"sync"
	"testing"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

func TestCompressedPartsRoundTrip(t *testing.T) {
	parts := []ContentPart{
		TextContent{Text: "hello world"},
		ToolCall{ID: "tc1", Name: "bash", Input: `{"command":"ls"}`},
	}
	data, err := marshalParts(parts)
	require.NoError(t, err)

	stored := compressParts(data)
	require.NotEqual(t, data, stored)
	require.Equal(t, zstdMagic, stored[:4], "stored parts should be a zstd frame")

	decoded, err := decompressPartsIfStored(stored)
	require.NoError(t, err)
	require.JSONEq(t, string(data), string(decoded))

	back, err := unmarshalParts(decoded)
	require.NoError(t, err)
	require.Len(t, back, 2)
	require.Equal(t, "hello world", back[0].(TextContent).Text)
}

func TestDecompressPartsLegacyJSON(t *testing.T) {
	legacy := []byte(`[{"type":"text","data":{"text":"old row"}}]`)
	decoded, err := decompressPartsIfStored(legacy)
	require.NoError(t, err)
	require.Equal(t, legacy, decoded, "plain JSON rows must pass through unchanged")

	decoded, err = decompressPartsIfStored([]byte(`[]`))
	require.NoError(t, err)
	require.Equal(t, []byte(`[]`), decoded)
}

func TestPartsCodecConcurrent(t *testing.T) {
	const workers = 8
	const iters = 50
	var wg sync.WaitGroup
	for w := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range iters {
				data, err := marshalParts([]ContentPart{TextContent{Text: strings.Repeat("x", w*i+1)}})
				require.NoError(t, err)
				stored := compressParts(data)
				decoded, err := decompressPartsIfStored(stored)
				require.NoError(t, err)
				require.Equal(t, data, decoded)
			}
		}()
	}
	wg.Wait()
}

// newTestServiceWithDB is newTestService but exposes the raw database so
// tests can assert on stored bytes.
func newTestServiceWithDB(t *testing.T) (Service, string, *sql.DB) {
	t.Helper()
	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	q := db.New(conn)
	sessions := session.NewService(q, conn)
	sess, err := sessions.Create(t.Context(), "test")
	require.NoError(t, err)
	return NewService(q), sess.ID, conn
}

// TestStoredPartsAreCompressed verifies the end-to-end path: what lands
// in SQLite is a zstd frame, not plain JSON, and the service reads it back.
func TestStoredPartsAreCompressed(t *testing.T) {
	svc, sessionID, conn := newTestServiceWithDB(t)
	text := strings.Repeat("compress me, I am long and repetitive. ", 64)
	_, err := svc.Create(t.Context(), sessionID, CreateMessageParams{
		Role:  User,
		Parts: []ContentPart{TextContent{Text: text}},
	})
	require.NoError(t, err)

	var stored []byte
	err = conn.QueryRowContext(t.Context(), "SELECT parts FROM messages LIMIT 1").Scan(&stored)
	require.NoError(t, err)
	require.Equal(t, zstdMagic, stored[:4], "stored parts should be a zstd frame")
	require.Less(t, len(stored), len(text), "compression should shrink repetitive text")

	msgs, err := svc.List(t.Context(), sessionID)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	require.Equal(t, text, msgs[0].Parts[0].(TextContent).Text)
}

// TestLegacyRowsStillReadable inserts a plain-JSON row the way every
// pre-compression crush version wrote it, then reads it through the service.
func TestLegacyRowsStillReadable(t *testing.T) {
	svc, sessionID, conn := newTestServiceWithDB(t)
	legacy := `[{"type":"text","data":{"text":"from an old version"}}]`
	_, err := conn.ExecContext(
		t.Context(),
		"INSERT INTO messages (id, session_id, role, parts, model, provider, is_summary_message, created_at, updated_at) VALUES (?, ?, 'user', ?, NULL, NULL, 0, 0, 0)",
		"legacy-row", sessionID, legacy,
	)
	require.NoError(t, err)

	msgs, err := svc.List(t.Context(), sessionID)
	require.NoError(t, err)
	var found bool
	for _, m := range msgs {
		if m.ID == "legacy-row" {
			found = true
			require.Len(t, m.Parts, 1)
			require.Equal(t, "from an old version", m.Parts[0].(TextContent).Text)
		}
	}
	require.True(t, found, "legacy row must be returned by List")
}

// TestUpdatedPartsStayCompressed covers the debounced Update/flush path:
// an UpdateMessage write must also store a zstd frame, not just Create.
func TestUpdatedPartsStayCompressed(t *testing.T) {
	svc, sessionID, conn := newTestServiceWithDB(t)
	msg, err := svc.Create(t.Context(), sessionID, CreateMessageParams{
		Role:  Assistant,
		Parts: []ContentPart{TextContent{Text: "initial"}},
		Model: "test-model",
	})
	require.NoError(t, err)

	msg.Parts = append(msg.Parts, TextContent{Text: strings.Repeat("appended reasoning delta ", 256)})
	require.NoError(t, svc.Update(t.Context(), msg))
	require.NoError(t, svc.FlushAll(t.Context()))

	var stored []byte
	err = conn.QueryRowContext(t.Context(), "SELECT parts FROM messages WHERE id = ?", msg.ID).Scan(&stored)
	require.NoError(t, err)
	require.Equal(t, zstdMagic, stored[:4], "updated parts should still be a zstd frame")

	back, err := svc.List(t.Context(), sessionID)
	require.NoError(t, err)
	require.Len(t, back, 1)
	require.Len(t, back[0].Parts, 2) // initial text + appended (Finish only auto-added for non-assistant)
}

// TestCorruptFrameErrors: a truncated/corrupt zstd frame must surface a
// decompression error, never pass through as garbage JSON.
func TestCorruptFrameErrors(t *testing.T) {
	svc, sessionID, conn := newTestServiceWithDB(t)
	frame := compressParts([]byte(`[{"type":"text","data":{"text":"payload"}}]`))
	corrupt := frame[:len(frame)-3] // truncate mid-frame

	_, err := conn.ExecContext(
		t.Context(),
		"INSERT INTO messages (id, session_id, role, parts, model, provider, is_summary_message, created_at, updated_at) VALUES (?, ?, 'user', ?, NULL, NULL, 0, 0, 0)",
		"corrupt-row", sessionID, corrupt,
	)
	require.NoError(t, err)

	_, err = svc.List(t.Context(), sessionID)
	require.Error(t, err, "corrupt frame must not decode silently")
}
