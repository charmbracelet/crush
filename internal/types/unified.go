package types

import (
	"database/sql"
	"errors"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
)

// UnifiedMessage represents a message with consistent typing across database and runtime
// This eliminates split-brain between sql.NullString vs string and int64 vs bool
type UnifiedMessage struct {
	ID               string             `json:"id"`
	SessionID        string             `json:"session_id"`
	Role             message.MessageRole `json:"role"`
	Parts            []message.ContentPart `json:"parts"`
	Model            OptionalString     `json:"model"`
	CreatedAt        int64              `json:"created_at"`
	UpdatedAt        int64              `json:"updated_at"`
	FinishedAt       OptionalInt64      `json:"finished_at"`
	Provider         OptionalString     `json:"provider"`
	IsSummary       bool               `json:"is_summary"`
}

// OptionalString provides type-safe optional string handling
type OptionalString struct {
	Value string
	Valid bool
}

// NewOptionalString creates a new optional string
func NewOptionalString(value string) OptionalString {
	return OptionalString{
		Value: value,
		Valid: true,
	}
}

// NewEmptyOptionalString creates an empty optional string
func NewEmptyOptionalString() OptionalString {
	return OptionalString{
		Value: "",
		Valid: false,
	}
}

// FromSQLNullString converts from sql.NullString to OptionalString
func (os *OptionalString) FromSQLNullString(ns sql.NullString) {
	os.Value = ns.String
	os.Valid = ns.Valid
}

// ToSQLNullString converts from OptionalString to sql.NullString
func (os OptionalString) ToSQLNullString() sql.NullString {
	return sql.NullString{
		String: os.Value,
		Valid:  os.Valid,
	}
}

// String returns the string value or empty if invalid
func (os OptionalString) String() string {
	if os.Valid {
		return os.Value
	}
	return ""
}

// IsEmpty returns true if the optional string is empty or invalid
func (os OptionalString) IsEmpty() bool {
	return !os.Valid || os.Value == ""
}

// OptionalInt64 provides type-safe optional int64 handling
type OptionalInt64 struct {
	Value int64
	Valid bool
}

// NewOptionalInt64 creates a new optional int64
func NewOptionalInt64(value int64) OptionalInt64 {
	return OptionalInt64{
		Value: value,
		Valid: true,
	}
}

// NewEmptyOptionalInt64 creates an empty optional int64
func NewEmptyOptionalInt64() OptionalInt64 {
	return OptionalInt64{
		Value: 0,
		Valid: false,
	}
}

// FromSQLNullInt64 converts from sql.NullInt64 to OptionalInt64
func (oi *OptionalInt64) FromSQLNullInt64(ni sql.NullInt64) {
	oi.Value = ni.Int64
	oi.Valid = ni.Valid
}

// ToSQLNullInt64 converts from OptionalInt64 to sql.NullInt64
func (oi OptionalInt64) ToSQLNullInt64() sql.NullInt64 {
	return sql.NullInt64{
		Int64: oi.Value,
		Valid:  oi.Valid,
	}
}

// Int64 returns the int64 value or 0 if invalid
func (oi OptionalInt64) Int64() int64 {
	if oi.Valid {
		return oi.Value
	}
	return 0
}

// IsEmpty returns true if the optional int64 is empty or invalid
func (oi OptionalInt64) IsEmpty() bool {
	return !oi.Valid
}

// UnifiedSession represents a session with consistent typing
type UnifiedSession struct {
	ID               string          `json:"id"`
	ParentSessionID   OptionalString  `json:"parent_session_id"`
	Title             string          `json:"title"`
	MessageCount     int64           `json:"message_count"`
	PromptTokens     int64           `json:"prompt_tokens"`
	CompletionTokens int64           `json:"completion_tokens"`
	Cost             float64         `json:"cost"`
	UpdatedAt        int64            `json:"updated_at"`
	CreatedAt        int64            `json:"created_at"`
	SummaryMessageID OptionalString  `json:"summary_message_id"`
}

// ToRuntimeMessage converts UnifiedMessage to runtime message.Message
func (um *UnifiedMessage) ToRuntimeMessage() message.Message {
	// Create with proper field mapping - remove fields that don't exist in runtime
	return message.Message{
		ID:               um.ID,
		SessionID:        um.SessionID,
		Role:             um.Role,
		Parts:            um.Parts,
		Model:            um.Model.String(),
		CreatedAt:        um.CreatedAt,
		UpdatedAt:        um.UpdatedAt,
		Provider:         um.Provider.String(),
		IsSummaryMessage: um.IsSummary,
	}
}

// ToDBMessage converts UnifiedMessage to database db.Message
func (um *UnifiedMessage) ToDBMessage() db.Message {
	return db.Message{
		ID:               um.ID,
		SessionID:        um.SessionID,
		Role:             string(um.Role),
		Parts:            "", // Will be serialized separately
		Model:            um.Model.ToSQLNullString(),
		CreatedAt:        um.CreatedAt,
		UpdatedAt:        um.UpdatedAt,
		FinishedAt:       um.FinishedAt.ToSQLNullInt64(),
		Provider:         um.Provider.ToSQLNullString(),
		IsSummaryMessage: boolToInt64(um.IsSummary),
	}
}

// FromDBMessage creates UnifiedMessage from database db.Message
func FromDBMessage(dbm db.Message) UnifiedMessage {
	return UnifiedMessage{
		ID:               dbm.ID,
		SessionID:        dbm.SessionID,
		Role:             message.MessageRole(dbm.Role),
		Parts:            nil, // Will be deserialized separately
		Model:            NewOptionalString(dbm.Model.String),
		CreatedAt:        dbm.CreatedAt,
		UpdatedAt:        dbm.UpdatedAt,
		FinishedAt:        func() OptionalInt64 {
			oi := NewEmptyOptionalInt64()
			oi.FromSQLNullInt64(dbm.FinishedAt)
			return oi
		}(),
		Provider:         NewOptionalString(dbm.Provider.String),
		IsSummary:       int64ToBool(dbm.IsSummaryMessage),
	}
}

// FromRuntimeMessage creates UnifiedMessage from runtime message.Message
func FromRuntimeMessage(msg message.Message) UnifiedMessage {
	return UnifiedMessage{
		ID:               msg.ID,
		SessionID:        msg.SessionID,
		Role:             msg.Role,
		Parts:            msg.Parts,
		Model:            NewOptionalString(msg.Model),
		CreatedAt:        msg.CreatedAt,
		UpdatedAt:        msg.UpdatedAt,
		FinishedAt:        NewEmptyOptionalInt64(), // Runtime message doesn't have FinishedAt
		Provider:         NewOptionalString(msg.Provider),
		IsSummary:       msg.IsSummaryMessage,
	}
}

// Helper functions for type conversions
func boolToInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func int64ToBool(i int64) bool {
	return i != 0
}

// Validation helper ensures message state consistency
func (um *UnifiedMessage) Validate() error {
	// Validate role
	if err := um.Role.Validate(); err != nil {
		return err
	}
	
	// Validate timestamps
	if um.CreatedAt <= 0 {
		return errors.New("created_at must be positive")
	}
	
	if um.UpdatedAt <= 0 {
		return errors.New("updated_at must be positive")
	}
	
	if um.UpdatedAt < um.CreatedAt {
		return errors.New("updated_at cannot be before created_at")
	}
	
	// Validate finished timestamp
	if um.FinishedAt.Valid && um.FinishedAt.Value < um.CreatedAt {
		return errors.New("finished_at cannot be before created_at")
	}
	
	// Validate ID and session ID
	if um.ID == "" {
		return errors.New("message ID cannot be empty")
	}
	
	if um.SessionID == "" {
		return errors.New("session ID cannot be empty")
	}
	
	return nil
}

// Validation helper for sessions
func (us *UnifiedSession) Validate() error {
	// Validate timestamps
	if us.CreatedAt <= 0 {
		return errors.New("created_at must be positive")
	}
	
	if us.UpdatedAt <= 0 {
		return errors.New("updated_at must be positive")
	}
	
	// Validate ID
	if us.ID == "" {
		return errors.New("session ID cannot be empty")
	}
	
	// Validate message count
	if us.MessageCount < 0 {
		return errors.New("message_count cannot be negative")
	}
	
	// Validate token counts
	if us.PromptTokens < 0 {
		return errors.New("prompt_tokens cannot be negative")
	}
	
	if us.CompletionTokens < 0 {
		return errors.New("completion_tokens cannot be negative")
	}
	
	// Validate cost
	if us.Cost < 0 {
		return errors.New("cost cannot be negative")
	}
	
	return nil
}