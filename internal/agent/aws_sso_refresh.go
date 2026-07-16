package agent

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/pubsub"
)

// awsSSORefreshTimeout bounds how long the AWS SSO refresh command may run.
// Browser-based SSO needs time, so it is generous, and it runs on a context
// detached from the agent turn so a cancelled turn doesn't abort the login.
const awsSSORefreshTimeout = 5 * time.Minute

// awsSSOURLRe matches the https verification URL that `aws sso login` and
// related commands print to stdout or stderr.
var awsSSOURLRe = regexp.MustCompile(`https://[^\s]+`)

// extractAWSSSOURL returns the first HTTPS URL in the given command output
// line, or empty if none is present.
func extractAWSSSOURL(line string) string {
	return awsSSOURLRe.FindString(line)
}

// refreshAWSCredentials runs the provider's configured AWS SSO refresh
// command (e.g. "aws sso login") on the machine that makes the Bedrock
// calls, streaming the verification URL to the UI for display, then rebuilds
// models so the AWS SDK re-reads the refreshed credentials. It returns nil to
// signal that the failed request should be retried.
//
// The command runs here, in the coordinator, rather than in the UI dialog so
// the refreshed credentials land where the model calls are made. This is
// correct in both single-process and client/server deployments.
func (c *coordinator) refreshAWSCredentials(ctx context.Context, providerCfg config.ProviderConfig) error {
	if c.notify == nil {
		return errNoInteractiveAuth
	}
	slog.Info("AWS credentials expired, running refresh command",
		"provider", providerCfg.ID, "command", providerCfg.AWSAuthRefresh)

	// Open the dialog immediately so the user sees progress even before the
	// command prints its verification URL.
	c.notify.Publish(pubsub.CreatedEvent, notify.Notification{
		Type:         notify.TypeAWSSSOAuth,
		ProviderID:   providerCfg.ID,
		AWSSOCommand: providerCfg.AWSAuthRefresh,
	})

	// Detach from the turn's context (with a generous timeout) so cancelling
	// the turn doesn't kill an in-progress browser login.
	runCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), awsSSORefreshTimeout)
	defer cancel()

	runErr := c.runAWSAuthRefresh(runCtx, providerCfg)

	result := notify.Notification{Type: notify.TypeAWSSSOAuthResult, ProviderID: providerCfg.ID}
	if runErr != nil {
		result.Message = runErr.Error()
	}
	c.notify.Publish(pubsub.CreatedEvent, result)

	if runErr != nil {
		slog.Error("AWS SSO refresh command failed", "provider", providerCfg.ID, "error", runErr)
		return runErr
	}
	// If the turn's context was cancelled while the command ran, fantasy's
	// retry would fail immediately, so surface the cancellation instead.
	if ctx.Err() != nil {
		return ctx.Err()
	}
	// Rebuild models so the AWS SDK credential chain re-reads the refreshed
	// SSO cache on the next attempt.
	if err := c.UpdateModels(runCtx); err != nil {
		slog.Error("Failed to update models after AWS SSO refresh", "provider", providerCfg.ID, "error", err)
		return err
	}
	slog.Info("AWS SSO refresh complete, retrying request", "provider", providerCfg.ID)
	return nil
}

// runAWSAuthRefresh executes the refresh command, publishing the SSO
// verification URL to the UI as soon as it appears in the output and
// returning any failure with captured stderr for context.
func (c *coordinator) runAWSAuthRefresh(ctx context.Context, providerCfg config.ProviderConfig) error {
	cmd := exec.CommandContext(ctx, "sh", "-c", providerCfg.AWSAuthRefresh)
	cmd.Dir = c.cfg.WorkingDir()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	// Merge stdout and stderr so the URL is caught regardless of where the
	// command writes it, while separately capturing stderr for error detail.
	var stderrBuf bytes.Buffer
	combined := io.MultiReader(stdout, io.TeeReader(stderrPipe, &stderrBuf))
	scanner := bufio.NewScanner(combined)
	urlSent := false
	for scanner.Scan() {
		// Keep draining after the URL is found so the command isn't blocked
		// on a full pipe and stderrBuf captures the full output for errors.
		if urlSent {
			continue
		}
		if url := extractAWSSSOURL(scanner.Text()); url != "" {
			urlSent = true
			// Second phase of the two-part publish: the dialog is already
			// open from refreshAWSCredentials; this fills in the URL on it.
			c.notify.Publish(pubsub.CreatedEvent, notify.Notification{
				Type:         notify.TypeAWSSSOAuth,
				ProviderID:   providerCfg.ID,
				AWSSOCommand: providerCfg.AWSAuthRefresh,
				AWSSOURL:     url,
			})
		}
	}

	if err := cmd.Wait(); err != nil {
		if stderr := strings.TrimSpace(stderrBuf.String()); stderr != "" {
			return fmt.Errorf("%w: %s", err, stderr)
		}
		return err
	}
	return nil
}
