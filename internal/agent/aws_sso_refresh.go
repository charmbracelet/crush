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
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/pubsub"
)

// awsSSORefreshTimeout bounds how long the AWS SSO refresh command may run.
// Browser-based SSO needs time, so it is generous, and the login runs on its
// own context so a cancelled turn doesn't abort it.
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
	// Every request that hits the expired session shares one login, so
	// parallel subagents open one browser flow rather than one each.
	// The login owns its context; a cancelled caller only stops waiting.
	done := c.awsRefresh.DoChan(providerCfg.ID, func() (any, error) {
		return nil, c.loginAWS(providerCfg)
	})
	select {
	case res := <-done:
		if res.Err != nil {
			return res.Err
		}
	case <-ctx.Done():
		return ctx.Err()
	}
	// The login may finish just as this request is cancelled. Don't retry
	// it: any error here makes fantasy skip the retry and surface the
	// original auth error, the same as a cancel during the wait above.
	if ctx.Err() != nil {
		return ctx.Err()
	}
	slog.Info("AWS SSO refresh complete, retrying request", "provider", providerCfg.ID)
	return nil
}

// loginAWS runs the provider's refresh command once and rebuilds the models
// so the AWS SDK re-reads the refreshed SSO cache. It is shared by every
// waiting request, so it runs on its own context rather than any caller's.
func (c *coordinator) loginAWS(providerCfg config.ProviderConfig) error {
	slog.Info("AWS credentials expired, running refresh command",
		"provider", providerCfg.ID, "command", providerCfg.AWSAuthRefresh)

	// Open the dialog immediately so the user sees progress even before the
	// command prints its verification URL.
	c.notify.Publish(pubsub.CreatedEvent, notify.Notification{
		Type:         notify.TypeAWSSSOAuth,
		ProviderID:   providerCfg.ID,
		AWSSOCommand: providerCfg.AWSAuthRefresh,
	})

	runCtx, cancel := context.WithTimeout(context.Background(), awsSSORefreshTimeout)
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
	if err := c.rebuildModels(runCtx); err != nil {
		slog.Error("Failed to update models after AWS SSO refresh", "provider", providerCfg.ID, "error", err)
		return err
	}
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

	// Drain stdout and stderr concurrently so a command that fills one pipe
	// buffer before closing the other can't deadlock. Both are scanned for
	// the verification URL; stderr is also captured for error detail.
	var (
		stderrBuf bytes.Buffer
		mu        sync.Mutex // Guards the single-shot URL publish across goroutines.
		urlSent   bool
	)
	publishURL := func(line string) {
		mu.Lock()
		defer mu.Unlock()
		if urlSent {
			return
		}
		if url := extractAWSSSOURL(line); url != "" {
			urlSent = true
			// Second phase of the two-part publish: loginAWS already opened
			// the dialog; this fills in the URL on it.
			c.notify.Publish(pubsub.CreatedEvent, notify.Notification{
				Type:         notify.TypeAWSSSOAuth,
				ProviderID:   providerCfg.ID,
				AWSSOCommand: providerCfg.AWSAuthRefresh,
				AWSSOURL:     url,
			})
		}
	}

	var wg sync.WaitGroup
	scan := func(r io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			publishURL(scanner.Text())
		}
	}
	wg.Add(2)
	go scan(stdout)
	go scan(io.TeeReader(stderrPipe, &stderrBuf))
	wg.Wait()

	if err := cmd.Wait(); err != nil {
		if stderr := strings.TrimSpace(stderrBuf.String()); stderr != "" {
			return fmt.Errorf("%w: %s", err, stderr)
		}
		return err
	}
	return nil
}
