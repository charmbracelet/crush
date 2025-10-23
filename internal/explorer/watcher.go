package explorer

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/charmbracelet/log/v2"
	"github.com/fsnotify/fsnotify"
)

// Event represents a file system event
type Event struct {
	Type  EventType
	Path  string
	Name  string
	IsDir bool
}

// EventType represents the type of file system event
type EventType int

const (
	EventCreate EventType = iota
	EventWrite
	EventRemove
	EventRename
	EventChmod
)

// Watcher monitors file system changes with debouncing
type Watcher struct {
	watcher      *fsnotify.Watcher
	tree         *Tree
	debounceTime time.Duration
	eventChan    chan Event
	stopChan     chan struct{}
	mu           sync.RWMutex
	subscribers  []chan Event
}

// NewWatcher creates a new file system watcher
func NewWatcher(tree *Tree, debounceTime time.Duration) (*Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &Watcher{
		watcher:      watcher,
		tree:         tree,
		debounceTime: debounceTime,
		eventChan:    make(chan Event, 100),
		stopChan:     make(chan struct{}),
		subscribers:  []chan Event{},
	}, nil
}

// Start begins watching the file system
func (w *Watcher) Start() error {
	// Watch the root directory and all subdirectories
	if err := w.watchDirectory(w.tree.Root.Path); err != nil {
		return err
	}

	// Start the event processing goroutine
	go w.processEvents()

	// Start the debouncing goroutine
	go w.debounceEvents()

	return nil
}

// Stop stops the file system watcher
func (w *Watcher) Stop() {
	close(w.stopChan)
	w.watcher.Close()
}

// Subscribe adds a subscriber to receive file system events
func (w *Watcher) Subscribe() chan Event {
	w.mu.Lock()
	defer w.mu.Unlock()

	eventChan := make(chan Event, 50)
	w.subscribers = append(w.subscribers, eventChan)
	return eventChan
}

// Unsubscribe removes a subscriber
func (w *Watcher) Unsubscribe(eventChan chan Event) {
	w.mu.Lock()
	defer w.mu.Unlock()

	for i, sub := range w.subscribers {
		if sub == eventChan {
			w.subscribers = append(w.subscribers[:i], w.subscribers[i+1:]...)
			close(sub)
			break
		}
	}
}

// watchDirectory recursively adds a directory to the watcher
func (w *Watcher) watchDirectory(path string) error {
	// Add the current directory
	if err := w.watcher.Add(path); err != nil {
		log.Errorf("Failed to watch directory %s: %v", path, err)
		return err
	}

	// Recursively watch subdirectories
	entries, err := filepath.Glob(filepath.Join(path, "*"))
	if err != nil {
		return err
	}

	for _, entry := range entries {
		info, err := os.Stat(entry)
		if err != nil {
			continue
		}

		if info.IsDir() {
			if err := w.watchDirectory(entry); err != nil {
				log.Warnf("Failed to watch subdirectory %s: %v", entry, err)
			}
		}
	}

	return nil
}

// processEvents processes raw fsnotify events and converts them to our Event type
func (w *Watcher) processEvents() {
	for {
		select {
		case <-w.stopChan:
			return

		case fsEvent, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			event := w.convertEvent(fsEvent)
			if event != nil {
				select {
				case w.eventChan <- *event:
				case <-w.stopChan:
					return
				}
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			log.Errorf("File system watcher error: %v", err)
		}
	}
}

// convertEvent converts fsnotify events to our Event type
func (w *Watcher) convertEvent(fsEvent fsnotify.Event) *Event {
	path := fsEvent.Name
	if path == "" {
		return nil
	}

	info, err := os.Stat(path)
	if err != nil {
		// File might have been deleted, try to get info from parent
		return &Event{
			Type:  EventRemove,
			Path:  path,
			Name:  filepath.Base(path),
			IsDir: false,
		}
	}

	var eventType EventType
	switch {
	case fsEvent.Op&fsnotify.Create == fsnotify.Create:
		eventType = EventCreate
	case fsEvent.Op&fsnotify.Write == fsnotify.Write:
		eventType = EventWrite
	case fsEvent.Op&fsnotify.Remove == fsnotify.Remove:
		eventType = EventRemove
	case fsEvent.Op&fsnotify.Rename == fsnotify.Rename:
		eventType = EventRename
	case fsEvent.Op&fsnotify.Chmod == fsnotify.Chmod:
		eventType = EventChmod
	default:
		return nil
	}

	return &Event{
		Type:  eventType,
		Path:  path,
		Name:  filepath.Base(path),
		IsDir: info.IsDir(),
	}
}

// debounceEvents debounces rapid file system events
func (w *Watcher) debounceEvents() {
	var pendingEvents map[string]Event
	var debounceTimer *time.Timer

	for {
		select {
		case <-w.stopChan:
			return

		case event := <-w.eventChan:
			if pendingEvents == nil {
				pendingEvents = make(map[string]Event)
			}

			// Group events by path
			pendingEvents[event.Path] = event

			// Reset the debounce timer
			if debounceTimer != nil {
				debounceTimer.Stop()
			}

			debounceTimer = time.AfterFunc(w.debounceTime, func() {
				w.flushPendingEvents(pendingEvents)
				pendingEvents = nil
			})

		case <-w.stopChan:
			return
		}
	}
}

// flushPendingEvents processes all pending events and notifies subscribers
func (w *Watcher) flushPendingEvents(events map[string]Event) {
	// Refresh the tree to reflect file system changes
	if err := w.tree.Refresh(); err != nil {
		log.Errorf("Failed to refresh tree: %v", err)
	}

	// Notify all subscribers
	w.mu.RLock()
	subscribers := make([]chan Event, len(w.subscribers))
	copy(subscribers, w.subscribers)
	w.mu.RUnlock()

	for _, event := range events {
		for _, sub := range subscribers {
			select {
			case sub <- event:
			case <-w.stopChan:
				return
			default:
				// Subscriber channel is full, skip
			}
		}
	}
}

// RefreshWatcher rebuilds the watcher with new directories
func (w *Watcher) RefreshWatcher() error {
	// Remove current watches
	w.watcher.Remove(w.tree.Root.Path)

	// Re-add watches
	return w.watchDirectory(w.tree.Root.Path)
}
