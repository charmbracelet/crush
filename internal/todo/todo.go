package todo

import (
	"context"
	"time"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/google/uuid"
)

type Status string

const (
	StatusPending    Status = "pending"
	StatusInProgress Status = "in_progress"
	StatusCompleted  Status = "completed"
)

type Todo struct {
	ID        string
	SessionID string
	Content   string
	Status    Status
	CreatedAt int64
	UpdatedAt int64
}

type CreateTodoParams struct {
	SessionID string
	Content   string
	Status    Status
}

type UpdateTodoParams struct {
	Content string
	Status  Status
}

type ListTodosParams struct {
	SessionID string
	Status    *Status // optional filter
}

type Service interface {
	Create(ctx context.Context, params CreateTodoParams) (Todo, error)
	Get(ctx context.Context, id string) (Todo, error)
	List(ctx context.Context, params ListTodosParams) ([]Todo, error)
	Update(ctx context.Context, id string, params UpdateTodoParams) (Todo, error)
	UpdateStatus(ctx context.Context, id string, status Status) (Todo, error)
	Delete(ctx context.Context, id string) error
	DeleteBySession(ctx context.Context, sessionID string) error
	CountBySessionAndStatus(ctx context.Context, sessionID string, status Status) (int64, error)
}

type service struct {
	q db.Querier
}

func NewService(q db.Querier) Service {
	return &service{q: q}
}

func (s *service) Create(ctx context.Context, params CreateTodoParams) (Todo, error) {
	now := time.Now().UnixMilli()
	dbTodo, err := s.q.CreateTodo(ctx, db.CreateTodoParams{
		ID:        uuid.New().String(),
		SessionID: params.SessionID,
		Content:   params.Content,
		Status:    string(params.Status),
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		return Todo{}, err
	}
	return s.fromDBItem(dbTodo), nil
}

func (s *service) Get(ctx context.Context, id string) (Todo, error) {
	dbTodo, err := s.q.GetTodo(ctx, id)
	if err != nil {
		return Todo{}, err
	}
	return s.fromDBItem(dbTodo), nil
}

func (s *service) List(ctx context.Context, params ListTodosParams) ([]Todo, error) {
	var dbTodos []db.Todo
	var err error

	if params.Status == nil {
		dbTodos, err = s.q.ListTodosBySession(ctx, params.SessionID)
	} else {
		dbTodos, err = s.q.ListTodosBySessionAndStatus(ctx, db.ListTodosBySessionAndStatusParams{
			SessionID: params.SessionID,
			Status:    string(*params.Status),
		})
	}

	if err != nil {
		return nil, err
	}

	todos := make([]Todo, len(dbTodos))
	for i, dbTodo := range dbTodos {
		todos[i] = s.fromDBItem(dbTodo)
	}
	return todos, nil
}

func (s *service) Update(ctx context.Context, id string, params UpdateTodoParams) (Todo, error) {
	now := time.Now().UnixMilli()
	dbTodo, err := s.q.UpdateTodo(ctx, db.UpdateTodoParams{
		Content:   params.Content,
		Status:    string(params.Status),
		UpdatedAt: now,
		ID:        id,
	})
	if err != nil {
		return Todo{}, err
	}
	return s.fromDBItem(dbTodo), nil
}

func (s *service) UpdateStatus(ctx context.Context, id string, status Status) (Todo, error) {
	now := time.Now().UnixMilli()
	dbTodo, err := s.q.UpdateTodoStatus(ctx, db.UpdateTodoStatusParams{
		Status:    string(status),
		UpdatedAt: now,
		ID:        id,
	})
	if err != nil {
		return Todo{}, err
	}
	return s.fromDBItem(dbTodo), nil
}

func (s *service) Delete(ctx context.Context, id string) error {
	return s.q.DeleteTodo(ctx, id)
}

func (s *service) DeleteBySession(ctx context.Context, sessionID string) error {
	return s.q.DeleteTodosBySession(ctx, sessionID)
}

func (s *service) CountBySessionAndStatus(ctx context.Context, sessionID string, status Status) (int64, error) {
	return s.q.CountTodosBySessionAndStatus(ctx, db.CountTodosBySessionAndStatusParams{
		SessionID: sessionID,
		Status:    string(status),
	})
}

func (s *service) fromDBItem(dbTodo db.Todo) Todo {
	return Todo{
		ID:        dbTodo.ID,
		SessionID: dbTodo.SessionID,
		Content:   dbTodo.Content,
		Status:    Status(dbTodo.Status),
		CreatedAt: dbTodo.CreatedAt,
		UpdatedAt: dbTodo.UpdatedAt,
	}
}
