package session

import (
	"context"

	"github.com/charmbracelet/crush/internal/db"
)

// AcquireWriter prevents independent processes from interleaving a transcript.
func (s *service) AcquireWriter(ctx context.Context, id string) error {
	return db.AcquireSessionWriter(ctx, s.db, id)
}
