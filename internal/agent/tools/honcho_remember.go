package tools

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/honcho"
)

// honchoRememberClamp bounds a stored conclusion. A conclusion is
// meant to be one durable statement; anything longer is a transcript
// excerpt wearing a disguise, and storing it would crowd out the
// conclusions that earn their place.
const honchoRememberClamp = 600

//go:embed honcho_remember.md
var honchoRememberDescription string

type HonchoRememberParams struct {
	Content string `json:"content" description:"One durable fact about the user or this project, written as a complete standalone sentence."`
}

type HonchoRememberResponseMetadata struct {
	Content   string `json:"content"`
	Observer  string `json:"observer"`
	Observed  string `json:"observed"`
	Session   string `json:"session"`
	Truncated bool   `json:"truncated"`
}

// NewHonchoRememberTool builds the tool that writes a conclusion
// directly, bypassing Honcho's background derivation.
//
// Direct writes are durable and unreviewed, so the description is
// deliberately strict about what belongs here: a wrong conclusion
// outlives the session that produced it and quietly misinforms every
// later one.
func NewHonchoRememberTool(svc *honcho.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		HonchoRememberToolName,
		honchoRememberDescription,
		func(ctx context.Context, params HonchoRememberParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if svc == nil {
				return memoryNotConfigured(), nil
			}
			content := strings.TrimSpace(params.Content)
			if content == "" {
				return fantasy.NewTextErrorResponse("content is required"), nil
			}

			stored := honchoTruncate(content, honchoRememberClamp)
			id := svc.Identity()
			observer := id.ObserverPeer(svc.Config().ObservationMode)

			err := svc.Client().CreateConclusions(ctx, []honcho.Conclusion{{
				Content:    stored,
				ObserverID: observer,
				ObservedID: id.UserPeer,
				SessionID:  id.SessionKey,
			}})
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to save memory: %v", err)), nil
			}

			metadata := HonchoRememberResponseMetadata{
				Content:   stored,
				Observer:  observer,
				Observed:  id.UserPeer,
				Session:   id.SessionKey,
				Truncated: stored != content,
			}

			result := "Saved to memory: " + stored
			if metadata.Truncated {
				result += "\n\n(Content was longer than " +
					fmt.Sprint(honchoRememberClamp) +
					" characters and was shortened. Prefer one short, complete statement.)"
			}
			return fantasy.WithResponseMetadata(fantasy.NewTextResponse(result), metadata), nil
		},
	)
}
