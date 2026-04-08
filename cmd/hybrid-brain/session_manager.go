// Copyright 2026 CrushCL. All rights reserved.
//
// Session Manager - HybridBrain 會話管理
// 追蹤多輪對話歷史，維護上下文

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// SessionState 會話狀態
type SessionState struct {
	ID            string            `json:"id"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
	Messages      []SessionMessage  `json:"messages"`
	TaskHistory   []TaskRecord      `json:"task_history"`
	TotalCostUSD  float64           `json:"total_cost_usd"`
	TotalTokens   int               `json:"total_tokens"`
	Metadata      map[string]string `json:"metadata"`
}

// SessionMessage 會話消息
type SessionMessage struct {
	Role      string    `json:"role"` // user, assistant, system
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// SessionManager 會話管理器
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*SessionState
	current  *SessionState

	// 配置
	persistPath string
	maxHistory  int
}

// NewSessionManager 創建新的會話管理器
func NewSessionManager(persistPath string) *SessionManager {
	return &SessionManager{
		sessions:  make(map[string]*SessionState),
		maxHistory: 100,
		persistPath: persistPath,
	}
}

// CreateSession 創建新會話
func (sm *SessionManager) CreateSession(id string) *SessionState {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session := &SessionState{
		ID:        id,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Messages:  make([]SessionMessage, 0),
		TaskHistory: make([]TaskRecord, 0),
		Metadata:  make(map[string]string),
	}

	sm.sessions[id] = session
	sm.current = session

	return session
}

// GetSession 獲取會話
func (sm *SessionManager) GetSession(id string) *SessionState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.sessions[id]
}

// CurrentSession 獲取當前會話
func (sm *SessionManager) CurrentSession() *SessionState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.current
}

// SetCurrentSession 設置當前會話
func (sm *SessionManager) SetCurrentSession(id string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if session, ok := sm.sessions[id]; ok {
		sm.current = session
		return true
	}
	return false
}

// AddMessage 添加消息到會話
func (sm *SessionManager) AddMessage(sessionID string, role, content string, metadata map[string]interface{}) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, ok := sm.sessions[sessionID]
	if !ok {
		session = sm.CreateSessionLocked(sessionID)
	}

	msg := SessionMessage{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
		Metadata:  metadata,
	}

	session.Messages = append(session.Messages, msg)
	session.UpdatedAt = time.Now()

	// 修剪歷史
	if len(session.Messages) > sm.maxHistory {
		session.Messages = session.Messages[len(session.Messages)-sm.maxHistory:]
	}

	return nil
}

// CreateSessionLocked 創建會話（已持有鎖）
func (sm *SessionManager) CreateSessionLocked(id string) *SessionState {
	session := &SessionState{
		ID:        id,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Messages:  make([]SessionMessage, 0),
		TaskHistory: make([]TaskRecord, 0),
		Metadata:  make(map[string]string),
	}
	sm.sessions[id] = session
	return session
}

// AddTaskRecord 添加任務記錄
func (sm *SessionManager) AddTaskRecord(sessionID string, record TaskRecord) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, ok := sm.sessions[sessionID]
	if !ok {
		session = sm.CreateSessionLocked(sessionID)
	}

	session.TaskHistory = append(session.TaskHistory, record)
	session.TotalCostUSD += record.Result.Cost
	session.TotalTokens += record.Result.Tokens
	session.UpdatedAt = time.Now()

	return nil
}

// GetMessages 獲取會話消息
func (sm *SessionManager) GetMessages(sessionID string) []SessionMessage {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, ok := sm.sessions[sessionID]
	if !ok {
		return nil
	}

	msgs := make([]SessionMessage, len(session.Messages))
	copy(msgs, session.Messages)
	return msgs
}

// GetContextSummary 獲取上下文摘要
func (sm *SessionManager) GetContextSummary(sessionID string) string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, ok := sm.sessions[sessionID]
	if !ok {
		return ""
	}

	var summary strings.Builder
	summary.WriteString("## 會話摘要\n\n")
	summary.WriteString(fmt.Sprintf("會話 ID: %s\n", session.ID))
	summary.WriteString(fmt.Sprintf("創建時間: %s\n", session.CreatedAt.Format("2006-01-02 15:04:05")))
	summary.WriteString(fmt.Sprintf("消息數量: %d\n", len(session.Messages)))
	summary.WriteString(fmt.Sprintf("任務數量: %d\n", len(session.TaskHistory)))
	summary.WriteString(fmt.Sprintf("總成本: $%.4f\n", session.TotalCostUSD))
	summary.WriteString(fmt.Sprintf("總 Tokens: %d\n", session.TotalTokens))

	if len(session.TaskHistory) > 0 {
		summary.WriteString("\n### 最近任務\n")
		recentTasks := session.TaskHistory
		if len(recentTasks) > 5 {
			recentTasks = recentTasks[len(recentTasks)-5:]
		}
		for _, task := range recentTasks {
			summary.WriteString(fmt.Sprintf("- [%s] %s (Cost: $%.4f)\n",
				task.Classification.Executor, truncate(task.Task, 50), task.Result.Cost))
		}
	}

	return summary.String()
}

// ListSessions 列出所有會話
func (sm *SessionManager) ListSessions() []*SessionState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sessions := make([]*SessionState, 0, len(sm.sessions))
	for _, s := range sm.sessions {
		sessions = append(sessions, s)
	}
	return sessions
}

// DeleteSession 刪除會話
func (sm *SessionManager) DeleteSession(id string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, ok := sm.sessions[id]; ok {
		delete(sm.sessions, id)
		if sm.current != nil && sm.current.ID == id {
			sm.current = nil
		}
		return true
	}
	return false
}

// Persist 保存會話到磁盤
func (sm *SessionManager) Persist(sessionID string) error {
	if sm.persistPath == "" {
		return nil
	}

	sm.mu.RLock()
	session, ok := sm.sessions[sessionID]
	sm.mu.RUnlock()

	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// 確保目錄存在
	dir := filepath.Dir(sm.persistPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// 序列化
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	// 寫入文件
	filename := filepath.Join(dir, fmt.Sprintf("session_%s.json", sessionID))
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	return nil
}

// Load 從磁盤加載會話
func (sm *SessionManager) Load(sessionID string) error {
	if sm.persistPath == "" {
		return nil
	}

	filename := filepath.Join(filepath.Dir(sm.persistPath), fmt.Sprintf("session_%s.json", sessionID))
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read session file: %w", err)
	}

	var session SessionState
	if err := json.Unmarshal(data, &session); err != nil {
		return fmt.Errorf("failed to unmarshal session: %w", err)
	}

	sm.mu.Lock()
	sm.sessions[sessionID] = &session
	sm.mu.Unlock()

	return nil
}

// GetStats 返回會話統計
func (sm *SessionManager) GetStats() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var totalCost float64
	var totalTokens int
	var totalTasks int

	for _, s := range sm.sessions {
		totalCost += s.TotalCostUSD
		totalTokens += s.TotalTokens
		totalTasks += len(s.TaskHistory)
	}

	return map[string]interface{}{
		"session_count":    len(sm.sessions),
		"total_cost_usd":   totalCost,
		"total_tokens":     totalTokens,
		"total_tasks":      totalTasks,
		"current_session":  sm.current,
	}
}
