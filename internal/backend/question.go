package backend

import (
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/question"
)

// AnswerQuestion submits answers for the question batch the request
// names. The returned bool reports whether this call resolved the
// pending batch (true) or found it already resolved by a previous
// caller (false).
func (b *Backend) AnswerQuestion(workspaceID string, req proto.QuestionAnswer) (bool, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return false, err
	}

	responses := make([]question.Answer, len(req.Responses))
	for i, r := range req.Responses {
		responses[i] = question.Answer{
			QuestionID:  r.QuestionID,
			SelectedIDs: r.SelectedIDs,
			FillInText:  r.FillInText,
			Yes:         r.Yes,
			Notes:       r.Notes,
		}
	}

	return ws.Questions.Answer(req.BatchRequestID, responses), nil
}

// CancelQuestion cancels the pending question batch with the given ID
// for a workspace. Returns true if a question was cancelled, false if
// none was pending.
func (b *Backend) CancelQuestion(workspaceID, batchID string) (bool, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return false, err
	}
	return ws.Questions.Cancel(batchID), nil
}
