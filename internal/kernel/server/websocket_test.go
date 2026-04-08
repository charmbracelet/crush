package server

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
)

// ============================================================
// WebSocket Handler Tests
// ============================================================

func TestWSHandler_Create(t *testing.T) {
	h := NewWSHandler()
	if h == nil {
		t.Fatal("NewWSHandler returned nil")
	}
	if h.clients == nil {
		t.Error("clients map not initialized")
	}
	if h.config.ReadTimeout != 60*time.Second {
		t.Errorf("expected ReadTimeout 60s, got %v", h.config.ReadTimeout)
	}
}

func TestWSHandler_ClientCount(t *testing.T) {
	h := NewWSHandler()
	count := h.GetClientCount()
	if count != 0 {
		t.Errorf("expected 0 clients, got %d", count)
	}
}

func TestWSHandler_Broadcast(t *testing.T) {
	h := NewWSHandler()
	// Should not panic with no clients
	h.BroadcastToAll("test", map[string]string{"msg": "hello"})
}

func TestDefaultWebSocketConfig(t *testing.T) {
	cfg := DefaultWebSocketConfig()
	if cfg.ReadBufferSize != 1024 {
		t.Errorf("expected ReadBufferSize 1024, got %d", cfg.ReadBufferSize)
	}
	if cfg.WriteBufferSize != 1024 {
		t.Errorf("expected WriteBufferSize 1024, got %d", cfg.WriteBufferSize)
	}
	if cfg.ReadTimeout != 60*time.Second {
		t.Errorf("expected ReadTimeout 60s, got %v", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout != 60*time.Second {
		t.Errorf("expected WriteTimeout 60s, got %v", cfg.WriteTimeout)
	}
	if cfg.PingInterval != 30*time.Second {
		t.Errorf("expected PingInterval 30s, got %v", cfg.PingInterval)
	}
}

// ============================================================
// WebSocket Message JSON Tests
// ============================================================

func TestWSMessage_JSON(t *testing.T) {
	msg := WSMessage{
		Type:    "execute",
		Payload: json.RawMessage(`{"prompt":"test"}`),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}

	var parsed WSMessage
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	if parsed.Type != "execute" {
		t.Errorf("expected type 'execute', got '%s'", parsed.Type)
	}
}

func TestWSTaskRequest_JSON(t *testing.T) {
	req := WSTaskRequest{
		Prompt:   "Hello world",
		Tools:    []string{"bash", "read"},
		Executor: "test-executor",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}

	var parsed WSTaskRequest
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	if parsed.Prompt != "Hello world" {
		t.Errorf("expected prompt 'Hello world', got '%s'", parsed.Prompt)
	}
	if len(parsed.Tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(parsed.Tools))
	}
	if parsed.Executor != "test-executor" {
		t.Errorf("expected executor 'test-executor', got '%s'", parsed.Executor)
	}
}

func TestWSTaskRequest_EmptyTools(t *testing.T) {
	req := WSTaskRequest{
		Prompt: "No tools",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}

	var parsed WSTaskRequest
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	if len(parsed.Tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(parsed.Tools))
	}
}

func TestWSTaskPayload_JSON(t *testing.T) {
	payload := WSTaskPayload{
		SessionID: "session-123",
		Text:      "Hello",
		Done:      true,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}

	var parsed WSTaskPayload
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	if parsed.SessionID != "session-123" {
		t.Errorf("expected session ID 'session-123', got '%s'", parsed.SessionID)
	}
	if !parsed.Done {
		t.Error("expected Done=true")
	}
}

func TestWSTaskPayload_DoneFalse(t *testing.T) {
	payload := WSTaskPayload{
		SessionID: "session-456",
		Text:      "Working...",
		Done:      false,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}

	var parsed WSTaskPayload
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	if parsed.Done {
		t.Error("expected Done=false")
	}
}

// ============================================================
// Chunk Text Tests
// ============================================================

func TestChunkText_Basic(t *testing.T) {
	chunks := chunkText("Hello World", 5)
	if len(chunks) != 3 {
		t.Errorf("expected 3 chunks, got %d", len(chunks))
	}
	if chunks[0] != "Hello" {
		t.Errorf("expected 'Hello', got '%s'", chunks[0])
	}
	if chunks[1] != " Worl" {
		t.Errorf("expected ' Worl', got '%s'", chunks[1])
	}
	if chunks[2] != "d" {
		t.Errorf("expected 'd', got '%s'", chunks[2])
	}
}

func TestChunkText_ExactDivision(t *testing.T) {
	chunks := chunkText("ABCDEF", 3)
	if len(chunks) != 2 {
		t.Errorf("expected 2 chunks, got %d", len(chunks))
	}
	if chunks[0] != "ABC" {
		t.Errorf("expected 'ABC', got '%s'", chunks[0])
	}
	if chunks[1] != "DEF" {
		t.Errorf("expected 'DEF', got '%s'", chunks[1])
	}
}

func TestChunkText_LargerThanText(t *testing.T) {
	chunks := chunkText("Hi", 100)
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0] != "Hi" {
		t.Errorf("expected 'Hi', got '%s'", chunks[0])
	}
}

func TestChunkText_Empty(t *testing.T) {
	chunks := chunkText("", 5)
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk for empty string, got %d", len(chunks))
	}
	if chunks[0] != "" {
		t.Errorf("expected empty chunk, got '%s'", chunks[0])
	}
}

func TestChunkText_SingleChar(t *testing.T) {
	chunks := chunkText("A", 1)
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0] != "A" {
		t.Errorf("expected 'A', got '%s'", chunks[0])
	}
}

func TestChunkText_ThreeChunks(t *testing.T) {
	chunks := chunkText("ABCDEFGH", 3)
	if len(chunks) != 3 {
		t.Errorf("expected 3 chunks, got %d", len(chunks))
	}
	if chunks[0] != "ABC" {
		t.Errorf("expected 'ABC', got '%s'", chunks[0])
	}
	if chunks[1] != "DEF" {
		t.Errorf("expected 'DEF', got '%s'", chunks[1])
	}
	if chunks[2] != "GH" {
		t.Errorf("expected 'GH', got '%s'", chunks[2])
	}
}

// ============================================================
// Concurrent Client Tests
// ============================================================

func TestWSHandler_ConcurrentClientAccess(t *testing.T) {
	h := NewWSHandler()

	var wg sync.WaitGroup
	clientCount := 100

	for i := 0; i < clientCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_ = h.GetClientCount()
		}(i)
	}

	wg.Wait()
}

func TestWSHandler_ConcurrentBroadcast(t *testing.T) {
	h := NewWSHandler()

	var wg sync.WaitGroup
	broadcastCount := 50

	for i := 0; i < broadcastCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			h.BroadcastToAll("test", map[string]int{"id": id})
		}(i)
	}

	wg.Wait()
}

func TestWSHandler_ConcurrentGetClientCount(t *testing.T) {
	h := NewWSHandler()

	// Add some clients first
	for i := 0; i < 10; i++ {
		client := &wsClient{
			ID:   fmt.Sprintf("client-%d", i),
			conn: nil,
			send: make(chan []byte, 256),
		}
		h.addClient(client)
	}

	var wg sync.WaitGroup
	iterations := 1000

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = h.GetClientCount()
		}()
	}

	wg.Wait()

	if h.GetClientCount() != 10 {
		t.Errorf("expected 10 clients, got %d", h.GetClientCount())
	}
}

// ============================================================
// Client Map Thread Safety Tests
// ============================================================

func TestWSHandler_AddRemoveClients(t *testing.T) {
	h := NewWSHandler()
	client := &wsClient{
		ID:   "test-client-1",
		conn: nil,
		send: make(chan []byte, 256),
	}

	h.addClient(client)
	if h.GetClientCount() != 1 {
		t.Errorf("expected 1 client, got %d", h.GetClientCount())
	}

	h.removeClient(client)
	if h.GetClientCount() != 0 {
		t.Errorf("expected 0 clients after remove, got %d", h.GetClientCount())
	}
}

func TestWSHandler_AddRemoveMultipleClients(t *testing.T) {
	h := NewWSHandler()
	clients := make([]*wsClient, 10)

	for i := 0; i < 10; i++ {
		clients[i] = &wsClient{
			ID:   fmt.Sprintf("client-%d", i),
			conn: nil,
			send: make(chan []byte, 256),
		}
		h.addClient(clients[i])
	}

	if h.GetClientCount() != 10 {
		t.Errorf("expected 10 clients, got %d", h.GetClientCount())
	}

	for i := 0; i < 10; i++ {
		h.removeClient(clients[i])
	}

	if h.GetClientCount() != 0 {
		t.Errorf("expected 0 clients after all removed, got %d", h.GetClientCount())
	}
}

func TestWSHandler_RemoveNonExistent(t *testing.T) {
	h := NewWSHandler()
	client := &wsClient{
		ID:   "nonexistent",
		conn: nil,
		send: make(chan []byte, 256),
	}

	// Should not panic
	h.removeClient(client)
}

func TestWSHandler_RemoveThenAdd(t *testing.T) {
	h := NewWSHandler()
	client := &wsClient{
		ID:   "transient-client",
		conn: nil,
		send: make(chan []byte, 256),
	}

	h.addClient(client)
	h.removeClient(client)
	h.addClient(client)

	if h.GetClientCount() != 1 {
		t.Errorf("expected 1 client after re-add, got %d", h.GetClientCount())
	}
}

// ============================================================
// Context Cancellation Tests
// ============================================================

func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan bool, 1)
	go func() {
		select {
		case <-ctx.Done():
			done <- true
		case <-time.After(1 * time.Second):
			done <- false
		}
	}()

	cancel()

	select {
	case wasCancelled := <-done:
		if !wasCancelled {
			t.Error("context was not cancelled")
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for cancellation")
	}
}

func TestContextCancellation_WithinLoop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	iterations := 0
	maxIterations := 10000

	go func() {
		for i := 0; i < maxIterations; i++ {
			select {
			case <-ctx.Done():
				return
			default:
				time.Sleep(time.Microsecond)
			}
			iterations++
		}
	}()

	time.Sleep(10 * time.Millisecond)
	cancel()

	time.Sleep(20 * time.Millisecond)

	if iterations >= maxIterations {
		t.Errorf("loop did not exit after cancellation, iterations: %d", iterations)
	}
}

func TestContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	select {
	case <-ctx.Done():
		// Expected
	case <-time.After(200 * time.Millisecond):
		t.Error("timeout did not trigger")
	}
}

func TestContextDone_CloseChannel(t *testing.T) {
	ch := make(chan struct{})
	close(ch)

	select {
	case <-ch:
		// Expected - channel closed
	case <-time.After(1 * time.Second):
		t.Error("channel was not closed")
	}
}

// ============================================================
// Generate ID Tests
// ============================================================

func TestGenerateClientID_Unique(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := generateClientID()
		if ids[id] {
			t.Errorf("duplicate ID generated: %s", id)
		}
		ids[id] = true
		time.Sleep(time.Nanosecond) // Ensure unique timestamps
	}
}

func TestGenerateClientID_Format(t *testing.T) {
	id := generateClientID()
	if len(id) < 10 {
		t.Errorf("ID too short: %s", id)
	}
}

func TestGenerateClientID_ContainsClient(t *testing.T) {
	id := generateClientID()
	if len(id) < 7 || id[:7] != "client-" {
		t.Errorf("ID should start with 'client-': %s", id)
	}
}

// ============================================================
// Race Condition Simulation Tests
// ============================================================

func TestRaceCondition_ClientMapConcurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping race test in short mode")
	}

	h := NewWSHandler()
	client := &wsClient{
		ID:   "race-test-map",
		conn: nil,
		send: make(chan []byte, 256),
	}

	h.addClient(client)

	var wg sync.WaitGroup
	iterations := 500

	// Concurrent readers
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = h.GetClientCount()
		}()
	}

	// Concurrent writers
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			newClient := &wsClient{
				ID:   fmt.Sprintf("client-%d", id),
				conn: nil,
				send: make(chan []byte, 256),
			}
			h.addClient(newClient)
			h.removeClient(newClient)
		}(i)
	}

	wg.Wait()
}

func TestRaceCondition_SendJSONConcurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping race test in short mode")
	}

	h := NewWSHandler()
	client := &wsClient{
		ID:     "race-test-send",
		conn:   nil,
		send:   make(chan []byte, 256),
		closed: false,
	}

	h.addClient(client)

	var wg sync.WaitGroup
	iterations := 200

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			h.sendJSON(client, map[string]int{"n": n})
		}(i)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(50 * time.Millisecond)
		h.mu.Lock()
		client.closed = true
		h.mu.Unlock()
	}()

	wg.Wait()
}

func TestRaceCondition_BroadcastWithClients(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping race test in short mode")
	}

	h := NewWSHandler()

	// Add some clients
	for i := 0; i < 5; i++ {
		client := &wsClient{
			ID:   fmt.Sprintf("broadcast-client-%d", i),
			conn: nil,
			send: make(chan []byte, 256),
		}
		h.addClient(client)
	}

	var wg sync.WaitGroup
	iterations := 100

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			h.BroadcastToAll("test", map[string]int{"id": id})
		}(i)
	}

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_ = h.GetClientCount()
		}(i)
	}

	wg.Wait()
}

// ============================================================
// Edge Cases
// ============================================================

func TestSendJSON_ClosedClient(t *testing.T) {
	h := NewWSHandler()
	client := &wsClient{
		ID:     "closed-client",
		conn:   nil,
		send:   make(chan []byte, 256),
		closed: true,
	}

	// Should not panic
	h.sendJSON(client, map[string]string{"msg": "test"})
}

func TestSendJSON_FullBuffer(t *testing.T) {
	h := NewWSHandler()
	client := &wsClient{
		ID:     "full-buffer",
		conn:   nil,
		send:   make(chan []byte, 1),
		closed: false,
	}

	h.addClient(client)

	// Fill the buffer
	client.send <- []byte("first")

	// This will block if buffer is full, so use non-blocking send
	select {
	case client.send <- []byte("second"):
		// Success
	case <-time.After(1 * time.Millisecond):
		// Buffer full, expected
	}

	// Should not panic, just skip
	h.sendJSON(client, map[string]string{"msg": "test"})
}

func TestBroadcast_EmptyClients(t *testing.T) {
	h := NewWSHandler()
	// No clients, should not panic
	h.BroadcastToAll("test", map[string]string{"msg": "test"})
}

func TestBroadcast_LargePayload(t *testing.T) {
	h := NewWSHandler()

	// Add a client
	client := &wsClient{
		ID:   "large-payload-client",
		conn: nil,
		send: make(chan []byte, 256),
	}
	h.addClient(client)

	// Large payload
	largeData := make([]byte, 1024*1024) // 1MB
	for i := range largeData {
		largeData[i] = 'x'
	}

	// Should not panic
	h.BroadcastToAll("large", map[string]interface{}{
		"data": string(largeData),
	})
}

// ============================================================
// Memory Leak Detection Tests
// ============================================================

func TestNoGoroutineLeak_AddRemove(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping leak test in short mode")
	}

	h := NewWSHandler()
	initialGoroutines := runtime.NumGoroutine()

	for i := 0; i < 100; i++ {
		client := &wsClient{
			ID:   fmt.Sprintf("client-%d", i),
			conn: nil,
			send: make(chan []byte, 256),
		}
		h.addClient(client)
		h.removeClient(client)
	}

	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	finalGoroutines := runtime.NumGoroutine()
	leaked := finalGoroutines - initialGoroutines

	// Allow some variance for test infrastructure
	if leaked > 10 {
		t.Errorf("potential goroutine leak: started with %d, ended with %d (leaked: %d)",
			initialGoroutines, finalGoroutines, leaked)
	}
}

func TestNoGoroutineLeak_Broadcast(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping leak test in short mode")
	}

	h := NewWSHandler()
	initialGoroutines := runtime.NumGoroutine()

	for i := 0; i < 50; i++ {
		client := &wsClient{
			ID:   fmt.Sprintf("broadcast-leak-client-%d", i),
			conn: nil,
			send: make(chan []byte, 256),
		}
		h.addClient(client)
	}

	for i := 0; i < 50; i++ {
		h.BroadcastToAll("test", map[string]int{"i": i})
	}

	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	finalGoroutines := runtime.NumGoroutine()
	leaked := finalGoroutines - initialGoroutines

	if leaked > 10 {
		t.Errorf("potential goroutine leak from broadcast: started with %d, ended with %d (leaked: %d)",
			initialGoroutines, finalGoroutines, leaked)
	}
}

// ============================================================
// Handler Execution Tests (context-based)
// ============================================================

func TestHandlerExecute_Execution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping execution test in short mode")
	}

	h := NewWSHandler()
	client := &wsClient{
		ID:   "execute-test-client",
		conn: nil,
		send: make(chan []byte, 256),
	}
	h.addClient(client)

	// Execute with a prompt
	go h.handleExecuteWS(client, json.RawMessage(`{"prompt":"test"}`))

	// Give time for execution
	time.Sleep(200 * time.Millisecond)

	// Check that messages were sent
	if len(client.send) == 0 {
		t.Error("no messages sent to client")
	}
}

// ============================================================
// Stress Tests
// ============================================================

func TestStress_ManyClients(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	h := NewWSHandler()
	clientCount := 200

	// Add many clients
	for i := 0; i < clientCount; i++ {
		client := &wsClient{
			ID:   fmt.Sprintf("stress-client-%d", i),
			conn: nil,
			send: make(chan []byte, 256),
		}
		h.addClient(client)
	}

	if h.GetClientCount() != clientCount {
		t.Errorf("expected %d clients, got %d", clientCount, h.GetClientCount())
	}

	// Broadcast to all
	h.BroadcastToAll("stress", map[string]int{"count": clientCount})

	// Remove all
	for i := 0; i < clientCount; i++ {
		client := &wsClient{
			ID:   fmt.Sprintf("stress-client-%d", i),
			conn: nil,
			send: make(chan []byte, 256),
		}
		h.removeClient(client)
	}

	if h.GetClientCount() != 0 {
		t.Errorf("expected 0 clients, got %d", h.GetClientCount())
	}
}

func TestStress_ConcurrentAddRemove(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	h := NewWSHandler()
	var wg sync.WaitGroup
	iterations := 100

	// Alternating add/remove
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			client := &wsClient{
				ID:   fmt.Sprintf("concurrent-client-%d", id),
				conn: nil,
				send: make(chan []byte, 256),
			}
			h.addClient(client)
			h.removeClient(client)
		}(i)
	}

	wg.Wait()

	if h.GetClientCount() != 0 {
		t.Errorf("expected 0 clients, got %d", h.GetClientCount())
	}
}

// Note: json import is used via json.RawMessage in the code
// The actual json package is used via json.Marshal/Unmarshal calls
var _ = json.Marshal
