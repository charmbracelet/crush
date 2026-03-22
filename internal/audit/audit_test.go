package audit

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestLogger(t *testing.T) *Logger {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	logger, err := NewLogger(path, "test-hmac-key-32byteslong12345")
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	t.Cleanup(func() { _ = logger.Close() })
	return logger
}

func makeEvent(actor, action string) Event {
	return Event{
		ID:        "ev-" + actor + "-" + action,
		SessionID: "test-session",
		Actor:     actor,
		Action:    EventAction(action),
		ToolName:  "test_tool",
		Resource: Resource{
			Type: ResourceHost,
			Name: "localhost",
		},
		Result: Result{
			Status: "success",
		},
		RiskScore: 10,
		RiskLevel: "LOW",
	}
}

func TestLogAndQuery(t *testing.T) {
	t.Parallel()
	logger := newTestLogger(t)
	ctx := context.Background()

	events := []Event{
		makeEvent("alice", string(ActionSecurityScan)),
		makeEvent("bob", string(ActionComplianceCheck)),
		makeEvent("alice", string(ActionLogAnalyze)),
	}
	for _, ev := range events {
		if err := logger.Log(ctx, ev); err != nil {
			t.Fatalf("Log: %v", err)
		}
	}

	// Query all
	results := logger.Query(ctx, Filter{Limit: 100})
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	// Query by actor
	aliceResults := logger.Query(ctx, Filter{Actor: "alice"})
	if len(aliceResults) != 2 {
		t.Errorf("expected 2 alice results, got %d", len(aliceResults))
	}

	// Query by action
	scanResults := logger.Query(ctx, Filter{Action: ActionSecurityScan})
	if len(scanResults) != 1 {
		t.Errorf("expected 1 scan result, got %d", len(scanResults))
	}
}

func TestQueryTimeRange(t *testing.T) {
	t.Parallel()
	logger := newTestLogger(t)
	ctx := context.Background()

	start := time.Now()
	ev := makeEvent("alice", string(ActionCommandExec))
	ev.Timestamp = start
	_ = logger.Log(ctx, ev)

	// Query with future start — should return nothing
	results := logger.Query(ctx, Filter{
		StartTime: start.Add(time.Hour),
	})
	if len(results) != 0 {
		t.Errorf("expected 0 results for future start time, got %d", len(results))
	}

	// Query with past start — should return event
	results = logger.Query(ctx, Filter{
		StartTime: start.Add(-time.Second),
	})
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestAuditChainIntegrity(t *testing.T) {
	t.Parallel()
	logger := newTestLogger(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		ev := makeEvent("user", string(ActionMonitoringQuery))
		if err := logger.Log(ctx, ev); err != nil {
			t.Fatalf("Log: %v", err)
		}
	}

	ok, count := logger.Verify()
	if !ok {
		t.Errorf("chain verification failed at event %d", count)
	}
	if count != 5 {
		t.Errorf("expected 5 verified events, got %d", count)
	}
}

func TestAuditChainTamperDetection(t *testing.T) {
	t.Parallel()
	logger := newTestLogger(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		ev := makeEvent("user", string(ActionFileAccess))
		_ = logger.Log(ctx, ev)
	}

	// Tamper with an event's hash
	logger.mu.Lock()
	if len(logger.events) > 1 {
		logger.events[1].Hash = "tampered"
	}
	logger.mu.Unlock()

	ok, _ := logger.Verify()
	if ok {
		t.Error("expected chain verification to fail after tampering")
	}
}

func TestLogWritesToFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	logger, err := NewLogger(path, "test-key")
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}

	ev := makeEvent("alice", string(ActionConfigChange))
	if err := logger.Log(context.Background(), ev); err != nil {
		t.Fatalf("Log: %v", err)
	}
	_ = logger.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(data) == 0 {
		t.Error("audit log file is empty")
	}

	// Ensure the file contains valid JSON
	var parsed Event
	if err := json.Unmarshal(data[:len(data)-1], &parsed); err != nil {
		t.Errorf("audit file does not contain valid JSON: %v", err)
	}
}

func TestQueryLimit(t *testing.T) {
	t.Parallel()
	logger := newTestLogger(t)
	ctx := context.Background()

	for i := 0; i < 20; i++ {
		ev := makeEvent("user", string(ActionLogAnalyze))
		_ = logger.Log(ctx, ev)
	}

	results := logger.Query(ctx, Filter{Limit: 5})
	if len(results) != 5 {
		t.Errorf("expected 5 results with limit=5, got %d", len(results))
	}
}

func TestQueryByRiskLevel(t *testing.T) {
	t.Parallel()
	logger := newTestLogger(t)
	ctx := context.Background()

	high := makeEvent("user", string(ActionCommandExec))
	high.RiskLevel = "HIGH"
	low := makeEvent("user", string(ActionLogAnalyze))
	low.RiskLevel = "LOW"
	_ = logger.Log(ctx, high)
	_ = logger.Log(ctx, low)

	results := logger.Query(ctx, Filter{RiskLevel: "HIGH"})
	if len(results) != 1 {
		t.Errorf("expected 1 HIGH result, got %d", len(results))
	}
}

func TestTimestampAutoSet(t *testing.T) {
	t.Parallel()
	logger := newTestLogger(t)
	ev := makeEvent("user", string(ActionNetworkDiag))
	// Leave Timestamp zero
	before := time.Now().UTC()
	_ = logger.Log(context.Background(), ev)
	after := time.Now().UTC()

	results := logger.Query(context.Background(), Filter{Limit: 1})
	if len(results) == 0 {
		t.Fatal("no results found")
	}
	ts := results[0].Timestamp
	if ts.Before(before) || ts.After(after) {
		t.Errorf("auto-set timestamp %v is outside expected range [%v, %v]", ts, before, after)
	}
}

// mockExporter records exported events.
type mockExporter struct {
	exported []Event
}

func (m *mockExporter) Export(_ context.Context, events []Event) error {
	m.exported = append(m.exported, events...)
	return nil
}
func (m *mockExporter) Name() string { return "mock" }

func TestExporterCalled(t *testing.T) {
	t.Parallel()
	logger := newTestLogger(t)
	exp := &mockExporter{}
	logger.AddExporter(exp)

	ev := makeEvent("alice", string(ActionSecurityScan))
	_ = logger.Log(context.Background(), ev)

	// Give the background goroutine a moment to complete.
	time.Sleep(50 * time.Millisecond)

	if len(exp.exported) == 0 {
		t.Error("expected exporter to have received at least one event")
	}
}
