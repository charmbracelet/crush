package tools

import (
	"context"

	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/shell"
)

// BackgroundJobs scopes the background job tools to the jobs their owner
// started in the current session tree. A subagent's jobs belong to the
// session that dispatched it, so the parent can read and stop them.
type BackgroundJobs struct {
	owner    any
	sessions session.Getter
}

// NewBackgroundJobs returns a scope for jobs started on behalf of owner.
// sessions resolves subagent sessions to their top-level session; nil
// treats every session as top-level.
func NewBackgroundJobs(owner any, sessions session.Getter) BackgroundJobs {
	return BackgroundJobs{owner: owner, sessions: sessions}
}

// Scope returns the job scope for the session running ctx.
func (b BackgroundJobs) Scope(ctx context.Context) shell.JobScope {
	root, _ := session.RootID(ctx, b.sessions, GetSessionFromContext(ctx))
	return shell.JobScope{Owner: b.owner, Session: root}
}
