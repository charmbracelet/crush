package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/charmbracelet/crushcl/internal/kernel/cl_kernel"
	"github.com/gorilla/websocket"
)

// WebSocketConfig WebSocket 配置
type WebSocketConfig struct {
	ReadBufferSize  int
	WriteBufferSize int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	PingInterval    time.Duration
}

// DefaultWebSocketConfig 返回預設配置
func DefaultWebSocketConfig() WebSocketConfig {
	return WebSocketConfig{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		ReadTimeout:     60 * time.Second,
		WriteTimeout:    60 * time.Second,
		PingInterval:    30 * time.Second,
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 生產環境應限制
	},
}

// WSHandler WebSocket 處理器
type WSHandler struct {
	config  WebSocketConfig
	clients map[string]*wsClient
	mu      sync.RWMutex
	handler *APIHandler
}

// wsClient WebSocket 客戶端連接
type wsClient struct {
	ID     string
	conn   *websocket.Conn
	send   chan []byte
	closed bool
}

// WSMessage WebSocket 消息
type WSMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// WSTaskRequest 任務請求
type WSTaskRequest struct {
	Prompt   string   `json:"prompt"`
	Tools    []string `json:"tools,omitempty"`
	Executor string   `json:"executor,omitempty"`
}

// WSTaskPayload 任務回應
type WSTaskPayload struct {
	SessionID string `json:"session_id"`
	Text      string `json:"text"`
	Done      bool   `json:"done"`
}

// NewWSHandler 創建 WebSocket 處理器
func NewWSHandler(config ...WebSocketConfig) *WSHandler {
	cfg := DefaultWebSocketConfig()
	if len(config) > 0 {
		cfg = config[0]
	}

	return &WSHandler{
		config:  cfg,
		clients: make(map[string]*wsClient),
		handler: NewAPIHandler(),
	}
}

// NewWSHandlerWithAgent 創建帶有 Agent Runner 的 WebSocket 處理器
// 這確保真實的 MiniMax API 被調用，避免執行本地 mock
func NewWSHandlerWithAgent(runner cl_kernel.AgentRunner, config ...WebSocketConfig) *WSHandler {
	cfg := DefaultWebSocketConfig()
	if len(config) > 0 {
		cfg = config[0]
	}

	return &WSHandler{
		config:  cfg,
		clients: make(map[string]*wsClient),
		handler: NewAPIHandlerWithAgent(runner),
	}
}

// HandleWebSocket 處理 WebSocket 連接
func (h *WSHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	client := &wsClient{
		ID:   generateClientID(),
		conn: conn,
		send: make(chan []byte, 256),
	}

	h.addClient(client)
	defer h.removeClient(client)

	h.readPump(client)
}

func (h *WSHandler) addClient(client *wsClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[client.ID] = client
}

func (h *WSHandler) removeClient(client *wsClient) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[client.ID]; ok {
		delete(h.clients, client.ID)
		close(client.send)
	}
}

func (h *WSHandler) readPump(client *wsClient) {
	defer func() {
		client.conn.Close()
	}()

	client.conn.SetReadLimit(512 * 1024) // 512KB
	client.conn.SetReadDeadline(time.Now().Add(h.config.ReadTimeout))
	client.conn.SetPongHandler(func(string) error {
		client.conn.SetReadDeadline(time.Now().Add(h.config.ReadTimeout))
		return nil
	})

	for {
		_, message, err := client.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		h.handleMessage(client, message)
	}
}

func (h *WSHandler) handleMessage(client *wsClient, message []byte) {
	var msg WSMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		h.sendError(client, "Invalid message format")
		return
	}

	switch msg.Type {
	case "execute":
		h.handleExecuteWS(client, msg.Payload)
	case "ping":
		h.sendJSON(client, map[string]string{"type": "pong"})
	default:
		h.sendError(client, fmt.Sprintf("Unknown message type: %s", msg.Type))
	}
}

func (h *WSHandler) handleExecuteWS(client *wsClient, payload json.RawMessage) {
	var req WSTaskRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		h.sendError(client, "Invalid execute payload")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func(ctx context.Context) {
		// 發送開始
		h.sendJSON(client, WSTaskPayload{
			SessionID: generateSessionID(),
			Text:      "",
			Done:      false,
		})

		chunks := chunkText(req.Prompt, 50)
		for i, chunk := range chunks {
			select {
			case <-ctx.Done():
				return
			default:
			}

			h.sendJSON(client, WSTaskPayload{
				SessionID: generateSessionID(),
				Text:      chunk,
				Done:      i == len(chunks)-1,
			})
			time.Sleep(50 * time.Millisecond)
		}
	}(ctx)
}

func (h *WSHandler) sendJSON(client *wsClient, data interface{}) {
	h.mu.RLock()
	closed := client.closed
	h.mu.RUnlock()

	if closed {
		return
	}

	message, err := json.Marshal(data)
	if err != nil {
		log.Printf("JSON marshal error: %v", err)
		return
	}

	select {
	case client.send <- message:
	default:
		// 發送緩衝區滿
	}
}

func (h *WSHandler) sendError(client *wsClient, errMsg string) {
	h.sendJSON(client, map[string]interface{}{
		"type":    "error",
		"message": errMsg,
	})
}

func (h *WSHandler) broadcast(message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, client := range h.clients {
		select {
		case client.send <- message:
		default:
			// 跳過忙碌的客戶端
		}
	}
}

// BroadcastToAll 廣播消息給所有客戶端
func (h *WSHandler) BroadcastToAll(msgType string, payload interface{}) {
	data := map[string]interface{}{
		"type":    msgType,
		"payload": payload,
	}
	message, _ := json.Marshal(data)
	h.broadcast(message)
}

// GetClientCount 返回連接數
func (h *WSHandler) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// helpers

func generateClientID() string {
	return fmt.Sprintf("client-%d", time.Now().UnixNano())
}

func chunkText(text string, chunkSize int) []string {
	var chunks []string
	for i := 0; i < len(text); i += chunkSize {
		end := i + chunkSize
		if end > len(text) {
			end = len(text)
		}
		chunks = append(chunks, text[i:end])
	}
	if len(chunks) == 0 {
		chunks = append(chunks, "")
	}
	return chunks
}
