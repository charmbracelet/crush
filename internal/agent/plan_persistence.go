package agent

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/plan"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
)

// planRunCompletePublisher persists final plan responses before publishing
// their terminal event. Both normal and queued plan turns use this publisher.
type planRunCompletePublisher struct {
	workingDir string
	notify     pubsub.Publisher[notify.Notification]
	downstream pubsub.Publisher[notify.RunComplete]
}

func (p *planRunCompletePublisher) Publish(eventType pubsub.EventType, complete notify.RunComplete) {
	complete = p.persist(context.Background(), complete)
	p.downstream.Publish(eventType, complete)
}

func (p *planRunCompletePublisher) PublishMustDeliver(ctx context.Context, eventType pubsub.EventType, complete notify.RunComplete) {
	complete = p.persist(ctx, complete)
	p.downstream.PublishMustDeliver(ctx, eventType, complete)
}

// persist saves a completed ready plan and returns the run event enriched
// with the saved path. Runs that did not produce a ready plan (errors,
// cancellations, intermediate replies) are returned unchanged.
func (p *planRunCompletePublisher) persist(ctx context.Context, complete notify.RunComplete) notify.RunComplete {
	if complete.Error != "" || complete.Cancelled || !plan.ReadyMarkerPresent(complete.Text) {
		return complete
	}
	path, err := saveReadyPlan(p.workingDir, complete, time.Now())
	if err != nil {
		slog.Error("Failed to save plan", "session_id", complete.SessionID, "message_id", complete.MessageID, "error", err)
		if p.notify != nil {
			p.notify.PublishMustDeliver(ctx, pubsub.CreatedEvent, notify.Notification{
				SessionID: complete.SessionID,
				Type:      notify.TypePlanSaveError,
				Message:   err.Error(),
			})
		}
		return complete
	}
	complete.PlanPath = path
	if p.notify != nil {
		p.notify.PublishMustDeliver(ctx, pubsub.CreatedEvent, notify.Notification{
			SessionID: complete.SessionID,
			Type:      notify.TypePlanSaved,
			Message:   path,
		})
	}
	return complete
}

func saveReadyPlan(workingDir string, complete notify.RunComplete, at time.Time) (string, error) {
	if _, err := uuid.Parse(complete.SessionID); err != nil {
		return "", fmt.Errorf("invalid plan session ID: %w", err)
	}
	if _, err := uuid.Parse(complete.MessageID); err != nil {
		return "", fmt.Errorf("invalid plan message ID: %w", err)
	}

	content := strings.Trim(plan.StripMarkers(complete.Text), "\r\n")
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("completed plan is empty")
	}
	content += "\n"

	dir := filepath.Join(workingDir, ".crush", "plans")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create plan directory: %w", err)
	}

	base := strings.TrimSuffix(plan.Filename(content, at), ".md")
	for attempt := 1; ; attempt++ {
		name := base + ".md"
		if attempt > 1 {
			name = fmt.Sprintf("%s-%d.md", base, attempt)
		}
		relativePath := filepath.Join(".crush", "plans", name)
		path := filepath.Join(dir, name)
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if os.IsExist(err) {
			existing, readErr := os.ReadFile(path)
			if readErr != nil {
				return "", fmt.Errorf("read existing plan: %w", readErr)
			}
			if string(existing) == content {
				// The same plan was saved before (or two plans share a
				// timestamp and title); reuse the existing file.
				return relativePath, nil
			}
			// A different plan landed on the same name: disambiguate with a
			// numeric suffix rather than overwriting it.
			continue
		}
		if err != nil {
			return "", fmt.Errorf("create plan file: %w", err)
		}
		if _, err := io.WriteString(f, content); err != nil {
			f.Close()
			os.Remove(path)
			return "", fmt.Errorf("write plan file: %w", err)
		}
		if err := f.Close(); err != nil {
			os.Remove(path)
			return "", fmt.Errorf("close plan file: %w", err)
		}
		return relativePath, nil
	}
}
