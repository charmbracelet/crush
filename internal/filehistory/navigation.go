package filehistory

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/google/uuid"
)

// Choice is the public, human-readable part of a prompt checkpoint.
type Choice struct {
	Turn  string `json:"turn"`
	Label string `json:"label"`
	Time  string `json:"time"`
}
type point struct {
	Choice
	Conversation json.RawMessage `json:"conversation"`
}
type recovery struct {
	Point  point   `json:"point"`
	Points []point `json:"points"`
	Policy string  `json:"policy"`
}
type navigation struct {
	Points  []point    `json:"points"`
	Redo    []recovery `json:"redo"`
	Pending *recovery  `json:"pending,omitempty"`
}

// Request invokes a native history operation. Restore IDs never need to be typed by users.
type Request struct {
	Action string `json:"action"`
	Target string `json:"target,omitempty"`
}

// Result contains prompt choices or completion of a combined restore.
type Result struct {
	Points []Choice `json:"points,omitempty"`
}

func (s *Store) navigationPath(session string) string {
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(s.Cwd+"\x00"+session)))
	return filepath.Join(s.DataDir, key+".navigation.json")
}
func (s *Store) navigation(session string) (navigation, error) {
	var state navigation
	bytes, err := os.ReadFile(s.navigationPath(session))
	if os.IsNotExist(err) {
		return state, nil
	}
	if err != nil {
		return state, err
	}
	err = json.Unmarshal(bytes, &state)
	return state, err
}
func (s *Store) saveNavigation(session string, state navigation) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.DataDir, "navigation-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err = tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.navigationPath(session))
}

// RecordConversation binds the pre-prompt database state to the active file checkpoint.
func RecordConversation(ctx context.Context, label string, data json.RawMessage) error {
	active, ok := ctx.Value(turnKey{}).(*turn)
	if !ok {
		return nil
	}
	state, err := active.store.navigation(active.session)
	if err != nil {
		return err
	}
	if state.Pending != nil {
		return fmt.Errorf("interrupted rewind; use /rewind-recover")
	}
	state.Points = append(state.Points, point{Choice: Choice{Turn: active.id, Label: label, Time: time.Now().Format(time.RFC3339)}, Conversation: data})
	state.Redo = nil
	return active.store.saveNavigation(active.session, state)
}

// Navigate journals recovery before writing files and only commits history after
// the host atomically replaces its conversation. The workspace lease excludes turns.
func (s *Store) Navigate(ctx context.Context, session string, request Request,
	snapshot func() (json.RawMessage, error), apply func(json.RawMessage) error) (Result, error) {
	release, err := s.Acquire(ctx)
	if err != nil {
		return Result{}, err
	}
	defer release()
	state, err := s.navigation(session)
	if err != nil {
		return Result{}, err
	}
	if request.Action == "list" {
		result := Result{}
		for _, p := range slices.Backward(state.Points) {
			result.Points = append(result.Points, p.Choice)
		}
		return result, nil
	}
	ctx = context.WithoutCancel(ctx)
	restore := func(turn, policy string) error {
		_, err := s.run(ctx, session, []string{"restore", "--turn", turn, "--ignore-rules-stdin"}, "restore.done", policy)
		return err
	}
	recoverState := func(saved recovery) error {
		if err := restore(saved.Point.Turn, saved.Policy); err != nil {
			return err
		}
		return apply(saved.Point.Conversation)
	}
	if request.Action == "recover" {
		if state.Pending == nil {
			return Result{}, fmt.Errorf("no interrupted rewind to recover")
		}
		if err := recoverState(*state.Pending); err != nil {
			return Result{}, err
		}
		state.Pending = nil
		return Result{}, s.saveNavigation(session, state)
	}
	if state.Pending != nil {
		return Result{}, fmt.Errorf("interrupted rewind; run /rewind-recover first")
	}
	var targets []string
	var destination json.RawMessage
	var nextPoints []point
	switch request.Action {
	case "rewind":
		index := slices.IndexFunc(state.Points, func(p point) bool { return p.Turn == request.Target })
		if index < 0 {
			return Result{}, fmt.Errorf("checkpoint is not on the current branch")
		}
		for _, p := range slices.Backward(state.Points[index:]) {
			targets = append(targets, p.Turn)
		}
		destination = state.Points[index].Conversation
		nextPoints = slices.Clone(state.Points[:index])
	case "redo":
		if len(state.Redo) == 0 {
			return Result{}, fmt.Errorf("nothing to redo; a new prompt clears redo")
		}
		saved := state.Redo[len(state.Redo)-1]
		targets = []string{saved.Point.Turn}
		destination = saved.Point.Conversation
		nextPoints = saved.Points
	default:
		return Result{}, fmt.Errorf("unknown rewind action")
	}
	current, err := snapshot()
	if err != nil {
		return Result{}, err
	}
	rescue := "rescue-" + uuid.NewString()
	args := []string{"prepare", "--turn", rescue}
	for _, target := range targets {
		args = append(args, "--target", target)
	}
	preview, err := s.run(ctx, session, args, "prepare.done", "")
	if err != nil {
		return Result{}, err
	}
	policy, ok := preview[len(preview)-1]["ignoreRules"].(string)
	if !ok {
		return Result{}, fmt.Errorf("missing restore policy")
	}
	saved := recovery{Point: point{Choice: Choice{Turn: rescue}, Conversation: current}, Points: slices.Clone(state.Points), Policy: policy}
	state.Pending = &saved
	if err := s.saveNavigation(session, state); err != nil {
		return Result{}, err
	}
	for _, target := range targets {
		if err = restore(target, policy); err != nil {
			break
		}
	}
	if err == nil {
		err = apply(destination)
	}
	if err != nil {
		if recoverErr := recoverState(saved); recoverErr != nil {
			return Result{}, fmt.Errorf("rewind failed (%v); recovery failed (%v); run /rewind-recover", err, recoverErr)
		}
		state.Pending = nil
		if saveErr := s.saveNavigation(session, state); saveErr != nil {
			return Result{}, saveErr
		}
		return Result{}, fmt.Errorf("rewind failed; original files and conversation recovered: %w", err)
	}
	state.Pending = nil
	state.Points = nextPoints
	if request.Action == "rewind" {
		state.Redo = append(state.Redo, saved)
	} else {
		state.Redo = state.Redo[:len(state.Redo)-1]
	}
	return Result{}, s.saveNavigation(session, state)
}
