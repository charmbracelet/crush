// Package filehistory integrates the official filesnap CLI with native turns.
package filehistory

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/charmbracelet/crush/internal/filehistory/binary"
	"github.com/charmbracelet/crush/internal/lock"
	"github.com/google/uuid"
)

// Event is a versioned filesnap JSONL record.
type Event map[string]any

// Store owns workspace-scoped file history outside the working tree.
type Store struct{ Cwd, DataDir, Binary string }

// New creates a store with a managed executable unless explicitly overridden.
func New(cwd, dataDir, executable string) *Store {
	return &Store{Cwd: cwd, DataDir: dataDir, Binary: executable}
}

// Supported reports availability of the automatic filesnap dependency.
func Supported() bool { return binary.Supported() }

type turnKey struct{}
type turn struct {
	store       *Store
	session, id string
	mu          sync.Mutex
}

// Acquire excludes another file-history restore or active turn in this workspace.
func (s *Store) Acquire(ctx context.Context) (func(), error) {
	cwd, err := filepath.EvalSymlinks(s.Cwd)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(s.DataDir, 0o700); err != nil {
		return nil, err
	}
	data, err := filepath.EvalSymlinks(s.DataDir)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(cwd, data)
	if err != nil {
		return nil, err
	}
	if rel == "." || (!filepath.IsAbs(rel) && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
		return nil, fmt.Errorf("file history data directory must be outside the workspace")
	}
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(cwd)))
	release, err := lock.TryFile(filepath.Join(data, key+".lock"))
	if err != nil {
		return nil, fmt.Errorf("file history is busy: %w", err)
	}
	if err := ctx.Err(); err != nil {
		release()
		return nil, err
	}
	return release, nil
}

// Begin captures a prompt before execution and holds its workspace lease.
func (s *Store) Begin(ctx context.Context, session string) (context.Context, func(), error) {
	release, err := s.Acquire(ctx)
	if err != nil {
		return ctx, nil, err
	}
	state, err := s.navigation(session)
	if err != nil {
		release()
		return ctx, nil, err
	}
	if state.Pending != nil {
		release()
		return ctx, nil, fmt.Errorf("interrupted rewind; run /rewind-recover first")
	}
	id := "crush-" + uuid.NewString()
	events, err := s.run(ctx, session, []string{"capture", "--turn", id}, "capture.done", "")
	if err == nil && events[len(events)-1]["dropped"] != float64(0) {
		err = fmt.Errorf("file checkpoint skipped paths; inspect coverage before continuing")
	}
	if err != nil {
		release()
		return ctx, nil, err
	}
	return context.WithValue(ctx, turnKey{}, &turn{store: s, session: session, id: id}), release, nil
}

// Declare captures an edit preimage in the active parent turn, including absence.
func Declare(ctx context.Context, path string) error {
	active, ok := ctx.Value(turnKey{}).(*turn)
	if !ok {
		return nil
	}
	active.mu.Lock()
	defer active.mu.Unlock()
	_, err := active.store.run(ctx, active.session, []string{"declare", "--turn", active.id, "--path", path}, "declare.done", "")
	return err
}

// WorkingDir returns the owning turn's workspace, if file history is enabled.
func WorkingDir(ctx context.Context) (string, bool) {
	active, ok := ctx.Value(turnKey{}).(*turn)
	if !ok {
		return "", false
	}
	return active.store.Cwd, true
}

// boundedBuffer drains excess output without terminating an in-flight restore.
type boundedBuffer struct {
	bytes.Buffer
	overflow bool
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	remaining := 8*1024*1024 - b.Len()
	if len(p) > remaining {
		b.overflow = true
		p = p[:remaining]
	}
	_, _ = b.Buffer.Write(p)
	return n, nil
}

func (s *Store) run(ctx context.Context, session string, args []string, doneType, stdin string) ([]Event, error) {
	argv := append([]string{"--data-dir", s.DataDir}, args...)
	if args[0] != "gc" {
		argv = append(argv, "--cwd", s.Cwd, "--session", session)
	}
	executable := s.Binary
	if executable == "" {
		var err error
		executable, err = binary.Resolve(ctx, s.DataDir)
		if err != nil {
			return nil, err
		}
	}
	cmd := exec.CommandContext(ctx, executable, argv...)
	cmd.Dir = s.Cwd
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr boundedBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	if stdout.overflow {
		return nil, fmt.Errorf("file history response exceeded its limit; inspect recovery before continuing")
	}
	decoder := json.NewDecoder(&stdout)
	var events []Event
	for {
		var event Event
		err := decoder.Decode(&event)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("invalid filesnap response: %w", err)
		}
		if event["v"] != float64(1) {
			return nil, fmt.Errorf("unsupported filesnap schema")
		}
		if _, ok := event["type"].(string); !ok {
			return nil, fmt.Errorf("missing filesnap event type")
		}
		events = append(events, event)
	}
	if runErr != nil {
		return events, fmt.Errorf("filesnap operation failed: %w: %s", runErr, strings.TrimSpace(stderr.String()))
	}
	if len(events) == 0 || events[len(events)-1]["type"] != doneType {
		return events, fmt.Errorf("filesnap did not report %s", doneType)
	}
	if failed, ok := events[len(events)-1]["failed"]; ok && failed != float64(0) {
		return events, fmt.Errorf("filesnap reported failed paths")
	}
	return events, nil
}

// Command performs an explicit file operation without rewriting conversation state.
func (s *Store) Command(ctx context.Context, session, action, target string) ([]Event, error) {
	if action != "list" && action != "restore" && action != "redo" && action != "delete" {
		return nil, fmt.Errorf("expected list, restore, redo, or delete")
	}
	if action == "restore" && target == "" {
		return nil, fmt.Errorf("restore requires a checkpoint ID")
	}
	release, err := s.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	if action == "list" {
		return s.run(ctx, session, []string{"log"}, "log.done", "")
	}
	// Once a restore starts, cancellation must not kill it between file writes.
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ctx = context.WithoutCancel(ctx)
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(s.Cwd+"\x00"+session)))
	recovery := filepath.Join(s.DataDir, key+".recovery")
	if action == "delete" {
		events, err := s.run(ctx, session, []string{"delete"}, "delete.done", "")
		if err != nil {
			return events, err
		}
		for _, record := range []string{recovery, s.navigationPath(session)} {
			if err := os.Remove(record); err != nil && !os.IsNotExist(err) {
				return events, err
			}
		}
		gc, err := s.run(ctx, session, []string{"gc"}, "gc.done", "")
		return append(events, gc...), err
	}
	if action == "redo" {
		data, err := os.ReadFile(recovery)
		if err != nil {
			return nil, err
		}
		target = strings.TrimSpace(string(data))
	}
	entries, err := s.run(ctx, session, []string{"log"}, "log.done", "")
	if err != nil {
		return nil, err
	}
	found := false
	for _, entry := range entries {
		if entry["type"] == "log.entry" && entry["turn"] == target {
			found = true
		}
	}
	if !found {
		return nil, fmt.Errorf("checkpoint does not belong to this session")
	}
	rescue := "rescue-" + uuid.NewString()
	preview, err := s.run(ctx, session, []string{"prepare", "--turn", rescue, "--target", target}, "prepare.done", "")
	if err != nil {
		return nil, err
	}
	policy, ok := preview[len(preview)-1]["ignoreRules"].(string)
	if !ok {
		return nil, fmt.Errorf("restore preparation omitted ignore rules")
	}
	tmp, err := os.CreateTemp(s.DataDir, "recovery-*.tmp")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())
	if _, err = tmp.WriteString(rescue); err != nil {
		tmp.Close()
		return nil, err
	}
	if err = tmp.Sync(); err != nil {
		tmp.Close()
		return nil, err
	}
	if err = tmp.Close(); err != nil {
		return nil, err
	}
	if err = os.Rename(tmp.Name(), recovery); err != nil {
		return nil, err
	}
	events, err := s.run(ctx, session, []string{"restore", "--turn", target, "--ignore-rules-stdin"}, "restore.done", policy)
	if err != nil {
		return events, fmt.Errorf("%w; recovery checkpoint %s is available through file-history redo", err, rescue)
	}
	return append(events, Event{"v": 1, "type": "file-history.recovery", "turn": rescue}), nil
}
