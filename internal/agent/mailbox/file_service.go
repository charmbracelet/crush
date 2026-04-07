package mailbox

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// Message represents a mailbox message for cross-process communication.
type Message struct {
	From      string `json:"from"`
	Text      string `json:"text"`
	Timestamp int64  `json:"timestamp"`
	Read      bool   `json:"read"`
	Color     string `json:"color,omitempty"`
	Summary   string `json:"summary,omitempty"`
}

// FileService provides filesystem-based mailbox operations for cross-process communication.
type FileService struct {
	teamsDir string
}

// NewFileService creates a new filesystem-based mailbox service.
// The teamsDir should be the global data directory's teams folder.
func NewFileService(globalDataDir string) *FileService {
	return &FileService{
		teamsDir: filepath.Join(globalDataDir, "teams"),
	}
}

// GetInboxPath returns the path to an agent's inbox file.
func (s *FileService) GetInboxPath(agentName, teamName string) string {
	sanitizedTeam := sanitizeName(teamName)
	sanitizedAgent := sanitizeName(agentName)
	return filepath.Join(s.teamsDir, sanitizedTeam, "inboxes", sanitizedAgent+".json")
}

// getLockPath returns the path to an inbox's lock file.
func (s *FileService) getLockPath(agentName, teamName string) string {
	inboxPath := s.GetInboxPath(agentName, teamName)
	return inboxPath + ".lock"
}

// withLock acquires an exclusive file lock and executes the provided function.
// This ensures atomic read-modify-write operations across processes.
func (s *FileService) withLock(agentName, teamName string, fn func() error) error {
	lockPath := s.getLockPath(agentName, teamName)
	lockDir := filepath.Dir(lockPath)

	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		return fmt.Errorf("create lock directory: %w", err)
	}

	// Create lock file.
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("create lock file: %w", err)
	}
	defer lockFile.Close()

	// Acquire exclusive lock (blocking with timeout).
	const maxRetries = 50
	const retryInterval = 100 * time.Millisecond

	for i := 0; i < maxRetries; i++ {
		err := tryLock(lockFile)
		if err == nil {
			defer unlock(lockFile)
			return fn()
		}
		if i < maxRetries-1 {
			time.Sleep(retryInterval)
		}
	}
	return fmt.Errorf("failed to acquire lock after %d attempts", maxRetries)
}

// tryLock attempts to acquire an exclusive lock on the file.
func tryLock(f *os.File) error {
	switch runtime.GOOS {
	case "windows":
		return tryLockWindows(f)
	default:
		return tryLockUnix(f)
	}
}

// unlock releases the lock on the file.
func unlock(f *os.File) error {
	switch runtime.GOOS {
	case "windows":
		return unlockWindows(f)
	default:
		return unlockUnix(f)
	}
}

// Read reads all messages from an agent's inbox.
func (s *FileService) Read(agentName, teamName string) ([]Message, error) {
	inboxPath := s.GetInboxPath(agentName, teamName)
	data, err := os.ReadFile(inboxPath)
	if os.IsNotExist(err) {
		return []Message{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read inbox: %w", err)
	}

	var messages []Message
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, fmt.Errorf("parse inbox: %w", err)
	}

	return messages, nil
}

// ReadUnread reads only unread messages from an agent's inbox.
func (s *FileService) ReadUnread(agentName, teamName string) ([]Message, error) {
	messages, err := s.Read(agentName, teamName)
	if err != nil {
		return nil, err
	}

	var unread []Message
	for _, m := range messages {
		if !m.Read {
			unread = append(unread, m)
		}
	}
	return unread, nil
}

// Write writes a message to an agent's inbox with file locking for cross-process safety.
func (s *FileService) Write(recipientName, teamName string, message Message) error {
	return s.withLock(recipientName, teamName, func() error {
		inboxPath := s.GetInboxPath(recipientName, teamName)
		inboxDir := filepath.Dir(inboxPath)

		if err := os.MkdirAll(inboxDir, 0o755); err != nil {
			return fmt.Errorf("create inbox directory: %w", err)
		}

		messages, err := s.Read(recipientName, teamName)
		if err != nil {
			return err
		}

		message.Timestamp = time.Now().UnixMilli()
		message.Read = false
		messages = append(messages, message)

		data, err := json.MarshalIndent(messages, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal messages: %w", err)
		}

		if err := os.WriteFile(inboxPath, data, 0o644); err != nil {
			return fmt.Errorf("write inbox: %w", err)
		}

		return nil
	})
}

// MarkAsRead marks all messages in an inbox as read with file locking.
func (s *FileService) MarkAsRead(agentName, teamName string) error {
	return s.withLock(agentName, teamName, func() error {
		inboxPath := s.GetInboxPath(agentName, teamName)

		messages, err := s.Read(agentName, teamName)
		if err != nil {
			return err
		}

		for i := range messages {
			messages[i].Read = true
		}

		data, err := json.MarshalIndent(messages, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal messages: %w", err)
		}

		if err := os.WriteFile(inboxPath, data, 0o644); err != nil {
			return fmt.Errorf("write inbox: %w", err)
		}

		return nil
	})
}

// MarkMessageAsRead marks a specific message as read by index with file locking.
func (s *FileService) MarkMessageAsRead(agentName, teamName string, index int) error {
	return s.withLock(agentName, teamName, func() error {
		inboxPath := s.GetInboxPath(agentName, teamName)

		messages, err := s.Read(agentName, teamName)
		if err != nil {
			return err
		}

		if index < 0 || index >= len(messages) {
			return fmt.Errorf("invalid message index %d", index)
		}

		messages[index].Read = true

		data, err := json.MarshalIndent(messages, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal messages: %w", err)
		}

		if err := os.WriteFile(inboxPath, data, 0o644); err != nil {
			return fmt.Errorf("write inbox: %w", err)
		}

		return nil
	})
}

// Clear clears all messages from an inbox with file locking.
func (s *FileService) Clear(agentName, teamName string) error {
	return s.withLock(agentName, teamName, func() error {
		inboxPath := s.GetInboxPath(agentName, teamName)
		inboxDir := filepath.Dir(inboxPath)

		if err := os.MkdirAll(inboxDir, 0o755); err != nil {
			return fmt.Errorf("create inbox directory: %w", err)
		}

		if err := os.WriteFile(inboxPath, []byte("[]"), 0o644); err != nil {
			return fmt.Errorf("clear inbox: %w", err)
		}

		return nil
	})
}

// sanitizeName converts a name to a safe file/directory name.
// Uses a regex allowlist to prevent path traversal (e.g. "..").
func sanitizeName(name string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	sanitized := re.ReplaceAllString(strings.TrimSpace(name), "-")
	// Remove leading/trailing hyphens.
	sanitized = strings.Trim(sanitized, "-")
	if sanitized == "" {
		sanitized = "default"
	}
	return sanitized
}
