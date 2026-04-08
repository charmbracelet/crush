// Package audit provides a structured, tamper-evident audit logging system
// for the SecOps agent. It supports local file-based logging with optional
// SIEM export to Syslog, Splunk HEC, and ELK/OpenSearch.
package audit

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"
)

// EventAction describes what operation was performed.
type EventAction string

const (
	ActionSecurityScan    EventAction = "security_scan"
	ActionComplianceCheck EventAction = "compliance_check"
	ActionLogAnalyze      EventAction = "log_analyze"
	ActionMonitoringQuery EventAction = "monitoring_query"
	ActionNetworkDiag     EventAction = "network_diagnostics"
	ActionCertificateAudit EventAction = "certificate_audit"
	ActionCommandExec     EventAction = "command_execute"
	ActionPermissionGrant EventAction = "permission_grant"
	ActionPermissionDeny  EventAction = "permission_deny"
	ActionFileAccess      EventAction = "file_access"
	ActionConfigChange    EventAction = "config_change"
)

// ResourceType describes the kind of resource being acted upon.
type ResourceType string

const (
	ResourceHost       ResourceType = "host"
	ResourceContainer  ResourceType = "container"
	ResourceDatabase   ResourceType = "database"
	ResourceFile       ResourceType = "file"
	ResourceNetwork    ResourceType = "network"
	ResourceCertificate ResourceType = "certificate"
	ResourceProcess    ResourceType = "process"
)

// Event is a single audit log entry.
type Event struct {
	ID        string     `json:"id"`
	Timestamp time.Time  `json:"timestamp"`
	SessionID string     `json:"session_id"`
	TraceID   string     `json:"trace_id,omitempty"`

	// Who
	Actor   string `json:"actor"`
	ActorIP string `json:"actor_ip,omitempty"`
	Role    string `json:"role,omitempty"`

	// What
	Action      EventAction `json:"action"`
	ToolName    string      `json:"tool_name"`
	Description string      `json:"description"`

	// Target
	Resource     Resource `json:"resource"`

	// Outcome
	Result    Result `json:"result"`
	RiskScore int    `json:"risk_score"`
	RiskLevel string `json:"risk_level"`

	// Change tracking
	Changes []Change `json:"changes,omitempty"`

	// Compliance
	ComplianceFramework string `json:"compliance_framework,omitempty"`
	ApprovalID          string `json:"approval_id,omitempty"`
	RollbackPlan        string `json:"rollback_plan,omitempty"`

	// Impact
	Duration        time.Duration `json:"duration_ms"`
	AffectedSystems []string      `json:"affected_systems,omitempty"`

	// Integrity
	PreviousHash string `json:"previous_hash,omitempty"`
	Hash         string `json:"hash,omitempty"`
}

// Resource is the target of an audit event.
type Resource struct {
	Type ResourceType `json:"type"`
	Name string       `json:"name"`
	ID   string       `json:"id,omitempty"`
}

// Result is the outcome of an audit event.
type Result struct {
	Status    string `json:"status"` // "success", "denied", "error"
	ErrorCode string `json:"error_code,omitempty"`
	Message   string `json:"message,omitempty"`
}

// Change records a before/after modification.
type Change struct {
	Field  string `json:"field"`
	Before string `json:"before"`
	After  string `json:"after"`
}

// Filter is used to query audit events.
type Filter struct {
	StartTime  time.Time    `json:"start_time,omitempty"`
	EndTime    time.Time    `json:"end_time,omitempty"`
	Actor      string       `json:"actor,omitempty"`
	Action     EventAction  `json:"action,omitempty"`
	Resource   ResourceType `json:"resource_type,omitempty"`
	RiskLevel  string       `json:"risk_level,omitempty"`
	SessionID  string       `json:"session_id,omitempty"`
	Limit      int          `json:"limit,omitempty"`
}

// Logger provides structured, tamper-evident audit logging.
type Logger struct {
	mu           sync.Mutex
	file         *os.File
	filePath     string
	hmacKey      []byte
	lastHash     string
	events       []Event // in-memory buffer for queries
	maxBuffer    int
	exporters    []Exporter
}

// Exporter is an interface for sending audit events to external systems.
type Exporter interface {
	Export(ctx context.Context, events []Event) error
	Name() string
}

// NewLogger creates a new audit logger that writes to the given file path.
func NewLogger(filePath string, hmacKey string) (*Logger, error) {
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log: %w", err)
	}

	return &Logger{
		file:      f,
		filePath:  filePath,
		hmacKey:   []byte(hmacKey),
		events:    make([]Event, 0, 1000),
		maxBuffer: 10000,
	}, nil
}

// AddExporter registers an external exporter (SIEM integration).
func (l *Logger) AddExporter(exp Exporter) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.exporters = append(l.exporters, exp)
}

// Log records a single audit event, computing its integrity hash and writing
// it to the log file and any registered exporters.
func (l *Logger) Log(ctx context.Context, event Event) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Set timestamp if not already set
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	// Compute chain hash for tamper detection
	event.PreviousHash = l.lastHash
	event.Hash = l.computeHash(event)
	l.lastHash = event.Hash

	// Write to file
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal audit event: %w", err)
	}
	if _, err := l.file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write audit event: %w", err)
	}

	// Buffer for queries
	if len(l.events) >= l.maxBuffer {
		// Evict oldest 10%
		evict := l.maxBuffer / 10
		l.events = l.events[evict:]
	}
	l.events = append(l.events, event)

	// Export to external systems (non-blocking).
	// Use context.Background so exports are not cancelled when the caller's
	// context expires — audit events must always reach SIEM.
	for _, exp := range l.exporters {
		go func(e Exporter) {
			exportCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := e.Export(exportCtx, []Event{event}); err != nil {
				slog.Error("audit export failed", "exporter", e.Name(), "error", err)
			}
		}(exp)
	}

	return nil
}

// Query returns events matching the given filter from the in-memory buffer.
func (l *Logger) Query(_ context.Context, filter Filter) []Event {
	l.mu.Lock()
	defer l.mu.Unlock()

	var results []Event
	limit := filter.Limit
	if limit == 0 {
		limit = 100
	}

	for i := len(l.events) - 1; i >= 0 && len(results) < limit; i-- {
		ev := l.events[i]
		if matchesFilter(ev, filter) {
			results = append(results, ev)
		}
	}
	return results
}

// Verify checks the integrity of the audit chain in the in-memory buffer.
func (l *Logger) Verify() (bool, int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	prevHash := ""
	for i, ev := range l.events {
		if ev.PreviousHash != prevHash {
			return false, i
		}
		expected := l.computeHash(ev)
		if ev.Hash != expected {
			return false, i
		}
		prevHash = ev.Hash
	}
	return true, len(l.events)
}

// ExportAll sends all buffered events to all registered exporters.
func (l *Logger) ExportAll(ctx context.Context) error {
	l.mu.Lock()
	events := make([]Event, len(l.events))
	copy(events, l.events)
	exporters := make([]Exporter, len(l.exporters))
	copy(exporters, l.exporters)
	l.mu.Unlock()

	for _, exp := range exporters {
		if err := exp.Export(ctx, events); err != nil {
			return fmt.Errorf("export to %s failed: %w", exp.Name(), err)
		}
	}
	return nil
}

// Close flushes and closes the audit log file.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.file.Close()
}

func (l *Logger) computeHash(event Event) string {
	payload := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%d|%s",
		event.Timestamp.Format(time.RFC3339Nano),
		event.SessionID,
		event.Actor,
		event.Action,
		event.Resource.Name,
		event.Result.Status,
		event.RiskScore,
		event.PreviousHash,
	)
	mac := hmac.New(sha256.New, l.hmacKey)
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func matchesFilter(ev Event, f Filter) bool {
	if !f.StartTime.IsZero() && ev.Timestamp.Before(f.StartTime) {
		return false
	}
	if !f.EndTime.IsZero() && ev.Timestamp.After(f.EndTime) {
		return false
	}
	if f.Actor != "" && ev.Actor != f.Actor {
		return false
	}
	if f.Action != "" && ev.Action != f.Action {
		return false
	}
	if f.Resource != "" && ev.Resource.Type != f.Resource {
		return false
	}
	if f.RiskLevel != "" && ev.RiskLevel != f.RiskLevel {
		return false
	}
	if f.SessionID != "" && ev.SessionID != f.SessionID {
		return false
	}
	return true
}
