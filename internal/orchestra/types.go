package orchestra

import (
	"time"
)

// Phase represents a development phase in the orchestration system.
type Phase struct {
	ID             string      `yaml:"id" json:"id"`
	Name           string      `yaml:"name" json:"name"`
	Project        string      `yaml:"project" json:"project"`
	SpecVersion    string      `yaml:"spec_version" json:"specVersion"`
	Description    string      `yaml:"description" json:"description"`
	Dependencies   []string    `yaml:"dependencies" json:"dependencies"`
	Requirements   []string    `yaml:"requirements" json:"requirements"`
	Deliverables   []string    `yaml:"deliverables" json:"deliverables"`
	EstimatedHours int         `yaml:"estimated_hours" json:"estimatedHours"`
	Status         PhaseStatus `yaml:"status" json:"status"`
	Tasks          []string    `yaml:"tasks" json:"tasks"`
}

// PhaseStatus represents the status of a phase.
type PhaseStatus string

const (
	PhaseStatusPending    PhaseStatus = "pending"
	PhaseStatusInProgress PhaseStatus = "in_progress"
	PhaseStatusBlocked    PhaseStatus = "blocked"
	PhaseStatusCompleted  PhaseStatus = "completed"
)

// Task represents a granular work item.
type Task struct {
	ID                  string        `yaml:"id" json:"id"`
	Title               string        `yaml:"title" json:"title"`
	Phase               string        `yaml:"phase" json:"phase"`
	Requirement         string        `yaml:"requirement" json:"requirement"`
	Status              TaskStatus    `yaml:"status" json:"status"`
	Created             time.Time     `yaml:"created" json:"created"`
	Started             *time.Time    `yaml:"started,omitempty" json:"started,omitempty"`
	Completed           *time.Time    `yaml:"completed,omitempty" json:"completed,omitempty"`
	Links               TaskLinks     `yaml:"links" json:"links"`
	AcceptanceCriteria  []string      `yaml:"acceptance_criteria" json:"acceptanceCriteria"`
	ImplementationHints []string      `yaml:"implementation_hints" json:"implementationHints"`
	Tests               []TaskTest    `yaml:"tests" json:"tests"`
	AssignedAgent       string        `yaml:"assigned_agent" json:"assignedAgent"`
	Branch              string        `yaml:"branch" json:"branch"`
	Worktree            string        `yaml:"worktree" json:"worktree"`
	Messages            []TaskMessage `yaml:"messages" json:"messages"`
}

// TaskStatus represents the status of a task.
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusReady      TaskStatus = "ready"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusReview     TaskStatus = "review"
	TaskStatusBlocked    TaskStatus = "blocked"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
)

// TaskLinks contains links to related documents.
type TaskLinks struct {
	Spec         string `yaml:"spec" json:"spec"`
	Blueprint    string `yaml:"blueprint" json:"blueprint"`
	Construction string `yaml:"construction" json:"construction"`
	Prisma       string `yaml:"prisma" json:"prisma"`
}

// TaskTest represents a test for a task.
type TaskTest struct {
	Type string `yaml:"type" json:"type"` // unit, integration, e2e
	Name string `yaml:"name" json:"name"`
}

// TaskMessage represents a message on a task.
type TaskMessage struct {
	Author    string    `yaml:"author" json:"author"`
	Content   string    `yaml:"content" json:"content"`
	Timestamp time.Time `yaml:"timestamp" json:"timestamp"`
}

// TaskLedger contains the current state of a task's execution.
type TaskLedger struct {
	TaskID         string     `yaml:"task_id" json:"taskId"`
	OriginalTask   string     `yaml:"original_task" json:"originalTask"`
	Facts          []string   `yaml:"facts" json:"facts"`
	Plan           []PlanStep `yaml:"plan" json:"plan"`
	CompletedSteps []string   `yaml:"completed_steps" json:"completedSteps"`
}

// PlanStep represents a step in the execution plan.
type PlanStep struct {
	ID          string `yaml:"id" json:"id"`
	Description string `yaml:"description" json:"description"`
	Status      string `yaml:"status" json:"status"`
	Agent       string `yaml:"agent" json:"agent"`
}

// ProgressLedger tracks the progress of agent collaboration.
type ProgressLedger struct {
	TaskID       string    `yaml:"task_id" json:"taskId"`
	CurrentAgent string    `yaml:"current_agent" json:"currentAgent"`
	TurnCount    int       `yaml:"turn_count" json:"turnCount"`
	StallCount   int       `yaml:"stall_count" json:"stallCount"`
	Messages     []Message `yaml:"messages" json:"messages"`
	IsComplete   bool      `yaml:"is_complete" json:"isComplete"`
	LastUpdated  time.Time `yaml:"last_updated" json:"lastUpdated"`
}

// Message represents a message in the progress ledger.
type Message struct {
	Agent     string    `yaml:"agent" json:"agent"`
	Content   string    `yaml:"content" json:"content"`
	Timestamp time.Time `yaml:"timestamp" json:"timestamp"`
	Action    string    `yaml:"action" json:"action"` // proposal, execution, result, handoff
}

// Worktree represents a git worktree for agent isolation.
type Worktree struct {
	Path      string    `yaml:"path" json:"path"`
	Branch    string    `yaml:"branch" json:"branch"`
	AgentName string    `yaml:"agent_name" json:"agentName"`
	TaskID    string    `yaml:"task_id" json:"taskId"`
	Created   time.Time `yaml:"created" json:"created"`
	Status    string    `yaml:"status" json:"status"` // active, completed, abandoned
}

// Agent represents an AI agent in the orchestra.
type Agent struct {
	Name         string   `yaml:"name" json:"name"`
	Role         string   `yaml:"role" json:"role"`
	Description  string   `yaml:"description" json:"description"`
	Instructions string   `yaml:"instructions" json:"instructions"`
	Tools        []string `yaml:"tools" json:"tools"`
	CLI          string   `yaml:"cli" json:"cli"` // "crush", "codex", "claude"
	Model        string   `yaml:"model" json:"model"`
	Handoffs     []string `yaml:"handoffs" json:"handoffs"` // Agents this one can hand off to
	IsParallel   bool     `yaml:"is_parallel" json:"isParallel"`
}

// Config represents the orchestra configuration.
type Config struct {
	Project     string            `yaml:"project" json:"project"`
	SpecVersion string            `yaml:"spec_version" json:"specVersion"`
	Agents      []Agent           `yaml:"agents" json:"agents"`
	Settings    OrchestraSettings `yaml:"settings" json:"settings"`
}

// OrchestraSettings contains settings for the orchestra.
type OrchestraSettings struct {
	MaxTurns         int    `yaml:"max_turns" json:"maxTurns"`
	StallThreshold   int    `yaml:"stall_threshold" json:"stallThreshold"`
	AutoHandoff      bool   `yaml:"auto_handoff" json:"autoHandoff"`
	RequireApproval  bool   `yaml:"require_approval" json:"requireApproval"`
	ParallelAgents   int    `yaml:"parallel_agents" json:"parallelAgents"`
	WorktreeBasePath string `yaml:"worktree_base_path" json:"worktreeBasePath"`
}
