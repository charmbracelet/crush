package session

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/event"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
	"github.com/zeebo/xxh3"
)

type TodoStatus string

const (
	TodoStatusPending    TodoStatus = "pending"
	TodoStatusInProgress TodoStatus = "in_progress"
	TodoStatusCompleted  TodoStatus = "completed"
	TodoStatusFailed     TodoStatus = "failed"
	TodoStatusCanceled   TodoStatus = "canceled"
)

func NormalizeTodoStatus(status string) TodoStatus {
	switch TodoStatus(status) {
	case TodoStatusInProgress:
		return TodoStatusInProgress
	case TodoStatusCompleted:
		return TodoStatusCompleted
	case TodoStatusFailed:
		return TodoStatusFailed
	case TodoStatusCanceled:
		return TodoStatusCanceled
	default:
		return TodoStatusPending
	}
}

type CollaborationMode string

const (
	CollaborationModeDefault CollaborationMode = "default"
	CollaborationModePlan    CollaborationMode = "plan"
)

func NormalizeCollaborationMode(mode string) CollaborationMode {
	switch CollaborationMode(mode) {
	case CollaborationModePlan:
		return CollaborationModePlan
	default:
		return CollaborationModeDefault
	}
}

type PermissionMode string

const (
	PermissionModeDefault PermissionMode = "default"
	PermissionModeAuto    PermissionMode = "auto"
	PermissionModeYolo    PermissionMode = "yolo"
)

func NormalizePermissionMode(mode string) PermissionMode {
	switch PermissionMode(mode) {
	case PermissionModeAuto:
		return PermissionModeAuto
	case PermissionModeYolo:
		return PermissionModeYolo
	default:
		return PermissionModeDefault
	}
}

type Kind string

const (
	KindNormal  Kind = "normal"
	KindHandoff Kind = "handoff"
)

func NormalizeKind(kind string) Kind {
	switch Kind(kind) {
	case KindHandoff:
		return KindHandoff
	default:
		return KindNormal
	}
}

// HashID returns the XXH3 hash of a session ID (UUID) as a hex string.
func HashID(id string) string {
	h := xxh3.New()
	h.WriteString(id)
	return fmt.Sprintf("%x", h.Sum(nil))
}

type Todo struct {
	ID          string     `json:"id,omitempty"`
	Content     string     `json:"content"`
	Status      TodoStatus `json:"status"`
	ActiveForm  string     `json:"active_form,omitempty"`
	Progress    int        `json:"progress,omitempty"`
	CreatedAt   int64      `json:"created_at,omitempty"`
	UpdatedAt   int64      `json:"updated_at,omitempty"`
	StartedAt   int64      `json:"started_at,omitempty"`
	CompletedAt int64      `json:"completed_at,omitempty"`
}

// HasIncompleteTodos returns true if there are any non-completed todos.
func HasIncompleteTodos(todos []Todo) bool {
	for _, todo := range todos {
		if todo.Status != TodoStatusCompleted {
			return true
		}
	}
	return false
}

func normalizeTodoProgress(status TodoStatus, progress int) int {
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	switch status {
	case TodoStatusCompleted:
		return 100
	case TodoStatusFailed, TodoStatusCanceled:
		if progress == 0 {
			return 100
		}
		return progress
	default:
		return progress
	}
}

func legacyTodoID(todo Todo, index int) string {
	source := fmt.Sprintf("%d|%s|%s|%s|%d|%d|%d|%d", index, strings.TrimSpace(todo.Content), todo.Status, strings.TrimSpace(todo.ActiveForm), todo.Progress, todo.CreatedAt, todo.StartedAt, todo.CompletedAt)
	return fmt.Sprintf("todo-%x", xxh3.HashString(source))
}

func normalizeTodosForStorage(todos []Todo) []Todo {
	if len(todos) == 0 {
		return nil
	}
	return normalizeTodos(todos, true)
}

func normalizeTodosForLoad(todos []Todo) []Todo {
	if len(todos) == 0 {
		return []Todo{}
	}
	return normalizeTodos(todos, false)
}

func normalizeTodos(todos []Todo, populateTimestamps bool) []Todo {
	normalized := make([]Todo, len(todos))
	seenIDs := make(map[string]struct{}, len(todos))
	now := time.Now().Unix()

	for i, todo := range todos {
		current := Todo{
			ID:          strings.TrimSpace(todo.ID),
			Content:     strings.TrimSpace(todo.Content),
			Status:      NormalizeTodoStatus(string(todo.Status)),
			ActiveForm:  strings.TrimSpace(todo.ActiveForm),
			Progress:    normalizeTodoProgress(NormalizeTodoStatus(string(todo.Status)), todo.Progress),
			CreatedAt:   todo.CreatedAt,
			UpdatedAt:   todo.UpdatedAt,
			StartedAt:   todo.StartedAt,
			CompletedAt: todo.CompletedAt,
		}

		if current.ID == "" {
			if populateTimestamps {
				current.ID = uuid.New().String()
			} else {
				current.ID = legacyTodoID(current, i)
			}
		} else if _, exists := seenIDs[current.ID]; exists {
			current.ID = uuid.New().String()
		}
		seenIDs[current.ID] = struct{}{}

		if populateTimestamps {
			if current.CreatedAt == 0 {
				current.CreatedAt = now
			}
			current.UpdatedAt = now
			switch current.Status {
			case TodoStatusInProgress:
				if current.StartedAt == 0 {
					current.StartedAt = now
				}
				current.CompletedAt = 0
			case TodoStatusCompleted:
				if current.StartedAt == 0 {
					current.StartedAt = now
				}
				if current.CompletedAt == 0 {
					current.CompletedAt = now
				}
			case TodoStatusFailed, TodoStatusCanceled:
				if current.StartedAt == 0 {
					current.StartedAt = now
				}
				if current.CompletedAt == 0 {
					current.CompletedAt = now
				}
			default:
				current.CompletedAt = 0
			}
		}

		normalized[i] = current
	}

	return normalized
}

type Session struct {
	ID                     string
	ParentSessionID        string
	Kind                   Kind
	Title                  string
	WorkspaceCWD           string
	CollaborationMode      CollaborationMode
	PermissionMode         PermissionMode
	HandoffSourceSessionID string
	HandoffGoal            string
	HandoffDraftPrompt     string
	HandoffRelevantFiles   []string
	MessageCount           int64
	PromptTokens           int64
	CompletionTokens       int64
	LastPromptTokens       int64
	LastCompletionTokens   int64
	SummaryMessageID       string
	Cost                   float64
	Todos                  []Todo
	CreatedAt              int64
	UpdatedAt              int64
}

func (s Session) LastInputTokens() int64 {
	return s.LastPromptTokens
}

func (s Session) LastOutputTokens() int64 {
	return s.LastCompletionTokens
}

func (s Session) LastExchangeTokens() int64 {
	return s.LastPromptTokens + s.LastCompletionTokens
}

// LastTotalTokens returns the total token count for the last exchange,
// including prompt tokens, completion tokens, and cached tokens.
// This matches opencode's token counting approach for consistent UI display
// and context window threshold calculations.
func (s Session) LastTotalTokens() int64 {
	return s.LastPromptTokens + s.LastCompletionTokens
}

type Service interface {
	pubsub.Subscriber[Session]
	Create(ctx context.Context, title string) (Session, error)
	CreateTitleSession(ctx context.Context, parentSessionID string) (Session, error)
	CreateTaskSession(ctx context.Context, toolCallID, parentSessionID, title string) (Session, error)
	CreateHandoffSession(ctx context.Context, sourceSessionID, title, goal, draftPrompt string, files []string) (Session, error)
	Get(ctx context.Context, id string) (Session, error)
	GetLast(ctx context.Context) (Session, error)
	List(ctx context.Context) ([]Session, error)
	ListChildren(ctx context.Context, parentID string) ([]Session, error)
	Save(ctx context.Context, session Session) (Session, error)
	UpdateCollaborationMode(ctx context.Context, sessionID string, mode CollaborationMode) (Session, error)
	UpdatePermissionMode(ctx context.Context, sessionID string, mode PermissionMode) (Session, error)
	SetDefaultPermissionMode(mode PermissionMode)
	UpdateTitleAndUsage(ctx context.Context, sessionID, title string, promptTokens, completionTokens int64, cost float64) error
	Rename(ctx context.Context, id string, title string) error
	Delete(ctx context.Context, id string) error

	// Agent tool session management
	CreateAgentToolSessionID(messageID, toolCallID string) string
	ParseAgentToolSessionID(sessionID string) (messageID string, toolCallID string, ok bool)
	IsAgentToolSession(sessionID string) bool
}

type service struct {
	*pubsub.Broker[Session]
	db                       *sql.DB
	q                        *db.Queries
	defaultCollaborationMode CollaborationMode
	defaultPermissionMode    PermissionMode
	onDeleteSession          func(sessionID string)
}

func (s *service) Create(ctx context.Context, title string) (Session, error) {
	dbSession, err := s.q.CreateSession(ctx, db.CreateSessionParams{
		ID:                   uuid.New().String(),
		Title:                title,
		WorkspaceCwd:         sql.NullString{},
		CollaborationMode:    string(s.defaultCollaborationMode),
		PermissionMode:       string(s.defaultPermissionMode),
		Kind:                 string(KindNormal),
		HandoffGoal:          "",
		HandoffDraftPrompt:   "",
		HandoffRelevantFiles: "[]",
	})
	if err != nil {
		return Session{}, err
	}
	session := s.fromDBItem(dbSession)
	s.Publish(pubsub.CreatedEvent, session)
	event.SessionCreated()
	return session, nil
}

func (s *service) CreateTaskSession(ctx context.Context, toolCallID, parentSessionID, title string) (Session, error) {
	parentSession, err := s.Get(ctx, parentSessionID)
	if err != nil {
		return Session{}, err
	}

	dbSession, err := s.q.CreateSession(ctx, db.CreateSessionParams{
		ID:              toolCallID,
		ParentSessionID: sql.NullString{String: parentSessionID, Valid: true},
		Title:           title,
		WorkspaceCwd: sql.NullString{
			String: parentSession.WorkspaceCWD,
			Valid:  parentSession.WorkspaceCWD != "",
		},
		CollaborationMode:    string(parentSession.CollaborationMode),
		PermissionMode:       string(parentSession.PermissionMode),
		Kind:                 string(KindNormal),
		HandoffGoal:          "",
		HandoffDraftPrompt:   "",
		HandoffRelevantFiles: "[]",
	})
	if err != nil {
		return Session{}, err
	}
	session := s.fromDBItem(dbSession)
	s.Publish(pubsub.CreatedEvent, session)
	return session, nil
}

func (s *service) CreateTitleSession(ctx context.Context, parentSessionID string) (Session, error) {
	dbSession, err := s.q.CreateSession(ctx, db.CreateSessionParams{
		ID:                   "title-" + parentSessionID,
		ParentSessionID:      sql.NullString{String: parentSessionID, Valid: true},
		Title:                "Generate a title",
		WorkspaceCwd:         sql.NullString{},
		CollaborationMode:    string(s.defaultCollaborationMode),
		PermissionMode:       string(s.defaultPermissionMode),
		Kind:                 string(KindNormal),
		HandoffGoal:          "",
		HandoffDraftPrompt:   "",
		HandoffRelevantFiles: "[]",
	})
	if err != nil {
		return Session{}, err
	}
	session := s.fromDBItem(dbSession)
	s.Publish(pubsub.CreatedEvent, session)
	return session, nil
}

func (s *service) CreateHandoffSession(ctx context.Context, sourceSessionID, title, goal, draftPrompt string, files []string) (Session, error) {
	relevantFilesJSON, err := marshalStringSlice(files)
	if err != nil {
		return Session{}, err
	}

	dbSession, err := s.q.CreateSession(ctx, db.CreateSessionParams{
		ID:                uuid.New().String(),
		Title:             title,
		WorkspaceCwd:      sql.NullString{},
		CollaborationMode: string(s.defaultCollaborationMode),
		PermissionMode:    string(s.defaultPermissionMode),
		Kind:              string(KindHandoff),
		HandoffSourceSessionID: sql.NullString{
			String: sourceSessionID,
			Valid:  sourceSessionID != "",
		},
		HandoffGoal:          goal,
		HandoffDraftPrompt:   draftPrompt,
		HandoffRelevantFiles: relevantFilesJSON,
	})
	if err != nil {
		return Session{}, err
	}
	session := s.fromDBItem(dbSession)
	s.Publish(pubsub.CreatedEvent, session)
	event.SessionCreated()
	return session, nil
}

func (s *service) Delete(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	qtx := s.q.WithTx(tx)

	dbSession, err := qtx.GetSessionByID(ctx, id)
	if err != nil {
		return err
	}
	if err = qtx.DeleteSessionMessages(ctx, dbSession.ID); err != nil {
		return fmt.Errorf("deleting session messages: %w", err)
	}
	if err = qtx.DeleteSessionFiles(ctx, dbSession.ID); err != nil {
		return fmt.Errorf("deleting session files: %w", err)
	}
	if err = qtx.DeleteSession(ctx, dbSession.ID); err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	session := s.fromDBItem(dbSession)
	if s.onDeleteSession != nil {
		s.onDeleteSession(session.ID)
	}
	s.Publish(pubsub.DeletedEvent, session)
	event.SessionDeleted()
	return nil
}

func (s *service) Get(ctx context.Context, id string) (Session, error) {
	dbSession, err := s.q.GetSessionByID(ctx, id)
	if err != nil {
		return Session{}, err
	}
	return s.fromDBItem(dbSession), nil
}

func (s *service) GetLast(ctx context.Context) (Session, error) {
	dbSession, err := s.q.GetLastSession(ctx)
	if err != nil {
		return Session{}, err
	}
	return s.fromDBItem(dbSession), nil
}

func (s *service) Save(ctx context.Context, session Session) (Session, error) {
	todosJSON, err := marshalTodos(session.Todos)
	if err != nil {
		return Session{}, err
	}
	relevantFilesJSON, err := marshalStringSlice(session.HandoffRelevantFiles)
	if err != nil {
		return Session{}, err
	}

	dbSession, err := s.q.UpdateSession(ctx, db.UpdateSessionParams{
		ID:                session.ID,
		Title:             session.Title,
		WorkspaceCwd:      sql.NullString{String: session.WorkspaceCWD, Valid: session.WorkspaceCWD != ""},
		CollaborationMode: string(NormalizeCollaborationMode(string(session.CollaborationMode))),
		PermissionMode:    string(NormalizePermissionMode(string(session.PermissionMode))),
		Kind:              string(NormalizeKind(string(session.Kind))),
		HandoffSourceSessionID: sql.NullString{
			String: session.HandoffSourceSessionID,
			Valid:  session.HandoffSourceSessionID != "",
		},
		HandoffGoal:          session.HandoffGoal,
		HandoffDraftPrompt:   session.HandoffDraftPrompt,
		HandoffRelevantFiles: relevantFilesJSON,
		PromptTokens:         session.PromptTokens,
		CompletionTokens:     session.CompletionTokens,
		LastPromptTokens:     session.LastPromptTokens,
		LastCompletionTokens: session.LastCompletionTokens,
		SummaryMessageID: sql.NullString{
			String: session.SummaryMessageID,
			Valid:  session.SummaryMessageID != "",
		},
		Cost: session.Cost,
		Todos: sql.NullString{
			String: todosJSON,
			Valid:  todosJSON != "",
		},
	})
	if err != nil {
		return Session{}, err
	}
	session = s.fromDBItem(dbSession)
	s.Publish(pubsub.UpdatedEvent, session)
	return session, nil
}

func (s *service) UpdateCollaborationMode(ctx context.Context, sessionID string, mode CollaborationMode) (Session, error) {
	current, err := s.Get(ctx, sessionID)
	if err != nil {
		return Session{}, err
	}
	transition := NewCollaborationModeTransition(current, mode)
	if !transition.Changed() {
		return current, nil
	}

	dbSession, err := s.q.UpdateSessionCollaborationMode(ctx, db.UpdateSessionCollaborationModeParams{
		ID:                sessionID,
		CollaborationMode: string(transition.Current.CollaborationMode),
	})
	if err != nil {
		return Session{}, err
	}
	updated := s.fromDBItem(dbSession)
	s.Publish(pubsub.UpdatedEvent, updated)
	return updated, nil
}

func (s *service) UpdatePermissionMode(ctx context.Context, sessionID string, mode PermissionMode) (Session, error) {
	current, err := s.Get(ctx, sessionID)
	if err != nil {
		return Session{}, err
	}
	transition := NewPermissionModeTransition(current, mode)
	if !transition.Changed() {
		return current, nil
	}

	dbSession, err := s.q.UpdateSessionPermissionMode(ctx, db.UpdateSessionPermissionModeParams{
		ID:             sessionID,
		PermissionMode: string(transition.Current.PermissionMode),
	})
	if err != nil {
		return Session{}, err
	}
	updated := s.fromDBItem(dbSession)
	s.Publish(pubsub.UpdatedEvent, updated)
	return updated, nil
}

func (s *service) SetDefaultPermissionMode(mode PermissionMode) {
	s.defaultPermissionMode = NormalizePermissionMode(string(mode))
}

// UpdateTitleAndUsage updates only the title and usage fields atomically.
// This is safer than fetching, modifying, and saving the entire session.
func (s *service) UpdateTitleAndUsage(ctx context.Context, sessionID, title string, promptTokens, completionTokens int64, cost float64) error {
	dbSession, err := s.q.UpdateSessionTitleAndUsage(ctx, db.UpdateSessionTitleAndUsageParams{
		ID:               sessionID,
		Title:            title,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		Cost:             cost,
	})
	if err != nil {
		return err
	}
	session := s.fromDBItem(dbSession)
	s.Publish(pubsub.UpdatedEvent, session)
	return nil
}

// Rename updates only the title of a session without touching updated_at or
// usage fields.
func (s *service) Rename(ctx context.Context, id string, title string) error {
	return s.q.RenameSession(ctx, db.RenameSessionParams{
		ID:    id,
		Title: title,
	})
}

func (s *service) List(ctx context.Context) ([]Session, error) {
	dbSessions, err := s.q.ListSessions(ctx)
	if err != nil {
		return nil, err
	}
	sessions := make([]Session, len(dbSessions))
	for i, dbSession := range dbSessions {
		sessions[i] = s.fromDBItem(dbSession)
	}
	return sessions, nil
}

func (s *service) ListChildren(ctx context.Context, parentID string) ([]Session, error) {
	dbSessions, err := s.q.ListSessionsByParentID(ctx, sql.NullString{String: parentID, Valid: parentID != ""})
	if err != nil {
		return nil, err
	}
	sessions := make([]Session, len(dbSessions))
	for i, dbSession := range dbSessions {
		sessions[i] = s.fromDBItem(dbSession)
	}
	return sessions, nil
}

func (s service) fromDBItem(item db.Session) Session {
	todos, err := unmarshalTodos(item.Todos.String)
	if err != nil {
		slog.Error("Failed to unmarshal todos", "session_id", item.ID, "error", err)
	}
	relevantFiles, err := unmarshalStringSlice(item.HandoffRelevantFiles)
	if err != nil {
		slog.Error("Failed to unmarshal handoff relevant files", "session_id", item.ID, "error", err)
	}
	return Session{
		ID:                     item.ID,
		ParentSessionID:        item.ParentSessionID.String,
		Kind:                   NormalizeKind(item.Kind),
		Title:                  item.Title,
		WorkspaceCWD:           item.WorkspaceCwd.String,
		CollaborationMode:      NormalizeCollaborationMode(item.CollaborationMode),
		PermissionMode:         NormalizePermissionMode(item.PermissionMode),
		HandoffSourceSessionID: item.HandoffSourceSessionID.String,
		HandoffGoal:            item.HandoffGoal,
		HandoffDraftPrompt:     item.HandoffDraftPrompt,
		HandoffRelevantFiles:   relevantFiles,
		MessageCount:           item.MessageCount,
		PromptTokens:           item.PromptTokens,
		CompletionTokens:       item.CompletionTokens,
		LastPromptTokens:       item.LastPromptTokens,
		LastCompletionTokens:   item.LastCompletionTokens,
		SummaryMessageID:       item.SummaryMessageID.String,
		Cost:                   item.Cost,
		Todos:                  todos,
		CreatedAt:              item.CreatedAt,
		UpdatedAt:              item.UpdatedAt,
	}
}

func marshalTodos(todos []Todo) (string, error) {
	if len(todos) == 0 {
		return "", nil
	}
	data, err := json.Marshal(normalizeTodosForStorage(todos))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func unmarshalTodos(data string) ([]Todo, error) {
	if data == "" {
		return []Todo{}, nil
	}
	var todos []Todo
	if err := json.Unmarshal([]byte(data), &todos); err != nil {
		return []Todo{}, err
	}
	return normalizeTodosForLoad(todos), nil
}

func marshalStringSlice(values []string) (string, error) {
	if len(values) == 0 {
		return "[]", nil
	}
	data, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func unmarshalStringSlice(data string) ([]string, error) {
	if data == "" {
		return []string{}, nil
	}
	var values []string
	if err := json.Unmarshal([]byte(data), &values); err != nil {
		return []string{}, err
	}
	return values, nil
}

func NewService(q *db.Queries, conn *sql.DB, defaultModes ...CollaborationMode) Service {
	return NewServiceWithDeleteCallback(q, conn, nil, defaultModes...)
}

func NewServiceWithDeleteCallback(q *db.Queries, conn *sql.DB, onDeleteSession func(sessionID string), defaultModes ...CollaborationMode) Service {
	broker := pubsub.NewBroker[Session]()
	defaultMode := CollaborationModeDefault
	if len(defaultModes) > 0 {
		defaultMode = NormalizeCollaborationMode(string(defaultModes[0]))
	}
	return &service{
		Broker:                   broker,
		db:                       conn,
		q:                        q,
		defaultCollaborationMode: defaultMode,
		defaultPermissionMode:    PermissionModeAuto,
		onDeleteSession:          onDeleteSession,
	}
}

// CreateAgentToolSessionID creates a session ID for agent tool sessions using the format "messageID$$toolCallID"
func (s *service) CreateAgentToolSessionID(messageID, toolCallID string) string {
	return fmt.Sprintf("%s$$%s", messageID, toolCallID)
}

// ParseAgentToolSessionID parses an agent tool session ID into its components
func (s *service) ParseAgentToolSessionID(sessionID string) (messageID string, toolCallID string, ok bool) {
	parts := strings.Split(sessionID, "$$")
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// IsAgentToolSession checks if a session ID follows the agent tool session format
func (s *service) IsAgentToolSession(sessionID string) bool {
	_, _, ok := s.ParseAgentToolSessionID(sessionID)
	return ok
}
