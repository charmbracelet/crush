package session

import "context"

// maxAncestryDepth bounds how far [RootID] walks the parent chain.
// Subagents cannot spawn subagents today, so one hop suffices in practice;
// the extra slack guards against cycles and future nesting.
const maxAncestryDepth = 4

// Getter looks sessions up by ID.
type Getter interface {
	Get(ctx context.Context, id string) (Session, error)
}

// RootID resolves id to its top-level session by walking the parent chain,
// so a subagent's session resolves to the session the user is viewing. A
// session with no parent resolves to itself. On error, id is returned
// unchanged so callers can fall back to it.
func RootID(ctx context.Context, sessions Getter, id string) (string, error) {
	if id == "" || sessions == nil {
		return id, nil
	}
	root := id
	for range maxAncestryDepth {
		sess, err := sessions.Get(ctx, root)
		if err != nil {
			return id, err
		}
		if sess.ParentSessionID == "" || sess.ParentSessionID == root {
			break
		}
		root = sess.ParentSessionID
	}
	return root, nil
}
