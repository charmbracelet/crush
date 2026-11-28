package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/shell"
)

// errNoSession is returned when a job tool runs outside any session, where
// there is no session for its jobs to belong to.
var errNoSession = errors.New("background jobs need a session")

// BackgroundJobs scopes the background job tools to the jobs their owner
// started in the current session tree. A subagent's jobs belong to the
// session that dispatched it, so the parent can read and stop them.
type BackgroundJobs struct {
	owner    any
	sessions session.Getter
	roots    *session.Roots
	report   JobReporter
}

// JobReporter tells sessionID that one of its background jobs finished.
type JobReporter func(sessionID string, job *shell.BackgroundShell)

// WithReports returns b with finished jobs reported through report. Only
// an interactive top-level agent should have them: a subagent's session is
// not driven by the agent that delivers reports, and a headless run exits,
// killing its jobs, before a report could arrive.
func (b BackgroundJobs) WithReports(report JobReporter) BackgroundJobs {
	b.report = report
	return b
}

// Reports reports whether finished jobs report back.
func (b BackgroundJobs) Reports() bool { return b.report != nil }

// nextSteps tells the agent what to do now that a job runs in the
// background.
func (b BackgroundJobs) nextSteps() string {
	if !b.Reports() {
		return "Use job_output tool to view output or job_kill to terminate."
	}
	return "You will get a <job-report> when it finishes, so keep working, or end your turn if the rest of your work needs its result. Use job_output to check on it sooner or job_kill to terminate."
}

// watch arranges for job to be reported to the session in scope once it
// finishes.
func (b BackgroundJobs) watch(scope shell.JobScope, job *shell.BackgroundShell) {
	if !b.Reports() || scope.Session == "" {
		return
	}
	job.ReportWhenDone(func() { b.report(scope.Session, job) })
}

// NewBackgroundJobs returns a scope for jobs started on behalf of owner.
// sessions resolves subagent sessions to their top-level session, nil
// treating every session as top-level; roots remembers each resolution.
func NewBackgroundJobs(owner any, sessions session.Getter, roots *session.Roots) BackgroundJobs {
	return BackgroundJobs{owner: owner, sessions: sessions, roots: roots}
}

// Scope returns the job scope for the session running ctx. It fails rather
// than guess when there is no session or its top-level session cannot be
// found, since a job tagged with the wrong session would be lost to the
// session it really belongs to.
func (b BackgroundJobs) Scope(ctx context.Context) (shell.JobScope, error) {
	id := GetSessionFromContext(ctx)
	if id == "" {
		return shell.JobScope{}, errNoSession
	}
	root, err := b.roots.Resolve(ctx, b.sessions, id)
	if err != nil {
		return shell.JobScope{}, fmt.Errorf("could not find the session this job belongs to: %w", err)
	}
	return shell.JobScope{Owner: b.owner, Session: root}, nil
}
