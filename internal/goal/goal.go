package goal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
)

type GoalStatus string

const (
	GoalActive   GoalStatus = "active"
	GoalPaused   GoalStatus = "paused"
	GoalComplete GoalStatus = "complete"
)

type contextKey struct{ name string }

// GoalIDContextKey is the context key used to propagate the active goal ID across tool calls.
var GoalIDContextKey = contextKey{"goal_id"}

type Goal struct {
	SessionID     string     `json:"session_id"`
	GoalID        string     `json:"goal_id"`
	Objective     string     `json:"objective"`
	Status        GoalStatus `json:"status"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	ActiveSeconds int64      `json:"active_seconds"`
}

type Service interface {
	pubsub.Subscriber[Goal]
	Get(ctx context.Context, sessionID string) (*Goal, error)
	Create(ctx context.Context, sessionID string, objective string) (*Goal, error)
	UpdateStatus(ctx context.Context, sessionID string, goalID string, status GoalStatus) (*Goal, error)
	Clear(ctx context.Context, sessionID string) (*Goal, error)
}

type service struct {
	*pubsub.Broker[Goal]
	db *sql.DB
	q  *db.Queries
}

func NewService(q *db.Queries, conn *sql.DB) Service {
	broker := pubsub.NewBroker[Goal]()
	return &service{
		Broker: broker,
		db:     conn,
		q:      q,
	}
}

func (s *service) Get(ctx context.Context, sessionID string) (*Goal, error) {
	dbGoal, err := s.q.GetGoalBySessionID(ctx, sessionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting goal: %w", err)
	}
	return s.fromDBItem(dbGoal), nil
}

func (s *service) Create(ctx context.Context, sessionID string, objective string) (*Goal, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	qtx := s.q.WithTx(tx)

	existing, err := qtx.GetGoalBySessionID(ctx, sessionID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("getting goal: %w", err)
	}

	if err == nil {
		existingGoal := s.fromDBItem(existing)
		if existingGoal.Status != GoalComplete {
			return nil, fmt.Errorf("session already has an active goal")
		}
		if err = qtx.DeleteGoal(ctx, sessionID); err != nil {
			return nil, fmt.Errorf("clearing completed goal: %w", err)
		}
	}

	goalID := uuid.New().String()
	dbGoal, err := qtx.CreateGoal(ctx, db.CreateGoalParams{
		SessionID: sessionID,
		GoalID:    goalID,
		Objective: objective,
		Status:    string(GoalActive),
	})
	if err != nil {
		return nil, fmt.Errorf("creating goal: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	goal := s.fromDBItem(dbGoal)
	s.Publish(pubsub.UpdatedEvent, *goal)
	return goal, nil
}

func (s *service) UpdateStatus(ctx context.Context, sessionID string, goalID string, status GoalStatus) (*Goal, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	qtx := s.q.WithTx(tx)

	if status != GoalActive {
		if err := qtx.AccumulateActiveTime(ctx, sessionID); err != nil {
			return nil, fmt.Errorf("accumulating active time: %w", err)
		}
	}
	dbGoal, err := qtx.UpdateGoalStatus(ctx, db.UpdateGoalStatusParams{
		SessionID: sessionID,
		GoalID:    goalID,
		Status:    string(status),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("goal not found or stale goal ID")
		}
		return nil, fmt.Errorf("updating goal status: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	goal := s.fromDBItem(dbGoal)
	s.Publish(pubsub.UpdatedEvent, *goal)
	return goal, nil
}

func (s *service) Clear(ctx context.Context, sessionID string) (*Goal, error) {
	goal, err := s.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if goal == nil {
		return nil, nil
	}
	if err = s.q.DeleteGoal(ctx, sessionID); err != nil {
		return nil, fmt.Errorf("deleting goal: %w", err)
	}
	s.Publish(pubsub.DeletedEvent, *goal)
	return goal, nil
}

func (s *service) fromDBItem(item db.Goal) *Goal {
	return &Goal{
		SessionID:     item.SessionID,
		GoalID:        item.GoalID,
		Objective:     item.Objective,
		Status:        GoalStatus(item.Status),
		CreatedAt:     time.Unix(item.CreatedAt, 0),
		UpdatedAt:     time.Unix(item.UpdatedAt, 0),
		ActiveSeconds: item.ActiveSeconds,
	}
}
