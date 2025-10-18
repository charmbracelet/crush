package commandhistory

import (
	"context"
	"database/sql"
	"strings"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
)

type CommandHistory struct {
	ID        string
	SessionID string
	Command   string
	CreatedAt int64
	UpdatedAt int64
}

type Service interface {
	pubsub.Suscriber[CommandHistory]
	Add(ctx context.Context, sessionID, command string) (CommandHistory, error)
	ListBySession(ctx context.Context, sessionID string, limit int) ([]CommandHistory, error)
	DeleteSessionHistory(ctx context.Context, sessionID string) error
}

type service struct {
	*pubsub.Broker[CommandHistory]
	db *sql.DB
	q  *db.Queries
}

const MaxHistorySize = 1000

func NewService(q *db.Queries, db *sql.DB) Service {
	return &service{
		Broker: pubsub.NewBroker[CommandHistory](),
		q:      q,
		db:     db,
	}
}

func (s *service) Add(ctx context.Context, sessionID, command string) (CommandHistory, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return CommandHistory{}, nil
	}

	// Get current count for this session
	countRow, err := s.q.GetCommandHistoryCount(ctx, db.GetCommandHistoryCountParams{
		SessionID: sessionID,
	})
	if err != nil {
		return CommandHistory{}, err
	}

	// If we're at the limit, remove oldest entries
	if int(countRow.Count) >= MaxHistorySize {
		history, err := s.q.ListCommandHistoryBySession(ctx, db.ListCommandHistoryBySessionParams{
			SessionID: sessionID,
		})
		if err != nil {
			return CommandHistory{}, err
		}

		// Remove oldest entries to make room
		toRemove := int(countRow.Count) - MaxHistorySize + 1
		for i := 0; i < toRemove && i < len(history); i++ {
			// Simple deletion - in a more sophisticated implementation,
			// we might want to batch delete
			if _, err := s.db.ExecContext(ctx, "DELETE FROM command_history WHERE id = ?", history[i].ID); err != nil {
				return CommandHistory{}, err
			}
		}
	}

	dbHistory, err := s.q.CreateCommandHistory(ctx, db.CreateCommandHistoryParams{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		Command:   command,
	})
	if err != nil {
		return CommandHistory{}, err
	}

	history := CommandHistory{
		ID:        dbHistory.ID,
		SessionID: dbHistory.SessionID,
		Command:   dbHistory.Command,
		CreatedAt: dbHistory.CreatedAt,
		UpdatedAt: dbHistory.UpdatedAt,
	}

	s.Publish(pubsub.CreatedEvent, history)
	return history, nil
}

func (s *service) ListBySession(ctx context.Context, sessionID string, limit int) ([]CommandHistory, error) {
	if limit <= 0 {
		limit = MaxHistorySize
	}

	dbHistory, err := s.q.ListLatestCommandHistoryBySession(ctx, db.ListLatestCommandHistoryBySessionParams{
		SessionID: sessionID,
		Limit:     int64(limit),
	})
	if err != nil {
		return nil, err
	}

	history := make([]CommandHistory, len(dbHistory))
	for i, dbItem := range dbHistory {
		// Reverse the slice so callers see commands in chronological order.
		history[len(dbHistory)-1-i] = CommandHistory{
			ID:        dbItem.ID,
			SessionID: dbItem.SessionID,
			Command:   dbItem.Command,
			CreatedAt: dbItem.CreatedAt,
			UpdatedAt: dbItem.UpdatedAt,
		}
	}
	return history, nil
}

func (s *service) DeleteSessionHistory(ctx context.Context, sessionID string) error {
	err := s.q.DeleteSessionCommandHistory(ctx, db.DeleteSessionCommandHistoryParams{
		SessionID: sessionID,
	})
	if err != nil {
		return err
	}
	// Publish deletion event
	s.Publish(pubsub.DeletedEvent, CommandHistory{SessionID: sessionID})
	return nil
}
