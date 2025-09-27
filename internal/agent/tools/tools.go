package tools

import (
	"context"
)

type (
	sessionIDContextKey string
	messageIDContextKey string
)

const (
	SessionIDContextKey sessionIDContextKey = "session_id"
)

func GetSessionFromContext(ctx context.Context) string {
	sessionID := ctx.Value(SessionIDContextKey)
	if sessionID == nil {
		return ""
	}
	s, ok := sessionID.(string)
	if !ok {
		return ""
	}
	return s
}
