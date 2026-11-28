package session

import (
	"context"
	"sync"
)

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

// Roots remembers the top-level session of every session [Roots.Resolve]
// resolved successfully. Parentage is fixed when a session is created, so a
// remembered root never goes stale, and a session that resolved once keeps
// resolving even if a later lookup would fail. The zero value is ready to
// use; a nil Roots remembers nothing.
type Roots struct {
	cache sync.Map // map[string]string
}

// Resolve returns the top-level session of id, looking sessions up in
// sessions when it has not resolved id before. On error, id is returned
// unchanged and nothing is remembered.
func (r *Roots) Resolve(ctx context.Context, sessions Getter, id string) (string, error) {
	if r == nil {
		return RootID(ctx, sessions, id)
	}
	if id == "" {
		return id, nil
	}
	if root, ok := r.cache.Load(id); ok {
		return root.(string), nil
	}
	root, err := RootID(ctx, sessions, id)
	if err != nil {
		return id, err
	}
	r.cache.Store(id, root)
	return root, nil
}
