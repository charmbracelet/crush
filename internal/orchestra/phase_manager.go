package orchestra

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// PhaseManager manages phases and tasks.
type PhaseManager struct {
	phasesDir string
	tasksDir  string
}

// NewPhaseManager creates a new phase manager.
func NewPhaseManager(phasesDir, tasksDir string) *PhaseManager {
	return &PhaseManager{
		phasesDir: phasesDir,
		tasksDir:  tasksDir,
	}
}

// LoadPhase loads a phase by ID.
func (m *PhaseManager) LoadPhase(id string) (*Phase, error) {
	path := filepath.Join(m.phasesDir, id+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read phase file: %w", err)
	}

	var phase Phase
	if err := yaml.Unmarshal(data, &phase); err != nil {
		return nil, fmt.Errorf("failed to parse phase: %w", err)
	}

	return &phase, nil
}

// LoadPhasesForProject loads all phases for a project.
func (m *PhaseManager) LoadPhasesForProject(project string) ([]Phase, error) {
	entries, err := os.ReadDir(m.phasesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read phases directory: %w", err)
	}

	var phases []Phase
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		phase, err := m.LoadPhase(strings.TrimSuffix(entry.Name(), ".yaml"))
		if err != nil {
			return nil, err
		}

		if project == "" || phase.Project == project {
			phases = append(phases, *phase)
		}
	}

	// Sort by phase ID
	sort.Slice(phases, func(i, j int) bool {
		return phases[i].ID < phases[j].ID
	})

	return phases, nil
}

// SavePhase saves a phase.
func (m *PhaseManager) SavePhase(phase *Phase) error {
	if err := os.MkdirAll(m.phasesDir, 0o755); err != nil {
		return fmt.Errorf("failed to create phases directory: %w", err)
	}

	data, err := yaml.Marshal(phase)
	if err != nil {
		return fmt.Errorf("failed to marshal phase: %w", err)
	}

	path := filepath.Join(m.phasesDir, phase.ID+".yaml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write phase file: %w", err)
	}

	return nil
}

// LoadTask loads a task by ID.
func (m *PhaseManager) LoadTask(id string) (*Task, error) {
	path := filepath.Join(m.tasksDir, id+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read task file: %w", err)
	}

	var task Task
	if err := yaml.Unmarshal(data, &task); err != nil {
		return nil, fmt.Errorf("failed to parse task: %w", err)
	}

	return &task, nil
}

// LoadTasksForPhase loads all tasks for a phase.
func (m *PhaseManager) LoadTasksForPhase(phaseID string) ([]Task, error) {
	phase, err := m.LoadPhase(phaseID)
	if err != nil {
		return nil, err
	}

	var tasks []Task
	for _, taskID := range phase.Tasks {
		task, err := m.LoadTask(taskID)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, *task)
	}

	return tasks, nil
}

// SaveTask saves a task.
func (m *PhaseManager) SaveTask(task *Task) error {
	if err := os.MkdirAll(m.tasksDir, 0o755); err != nil {
		return fmt.Errorf("failed to create tasks directory: %w", err)
	}

	data, err := yaml.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	path := filepath.Join(m.tasksDir, task.ID+".yaml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write task file: %w", err)
	}

	return nil
}

// CreateTask creates a new task.
func (m *PhaseManager) CreateTask(phaseID, title, requirement string) (*Task, error) {
	phase, err := m.LoadPhase(phaseID)
	if err != nil {
		return nil, err
	}

	taskID := m.generateTaskID(phase)
	task := &Task{
		ID:                  taskID,
		Title:               title,
		Phase:               phaseID,
		Requirement:         requirement,
		Status:              TaskStatusPending,
		Created:             time.Now(),
		AcceptanceCriteria:  []string{},
		ImplementationHints: []string{},
		Tests:               []TaskTest{},
		Messages:            []TaskMessage{},
	}

	if err := m.SaveTask(task); err != nil {
		return nil, err
	}

	// Add task to phase
	phase.Tasks = append(phase.Tasks, taskID)
	if err := m.SavePhase(phase); err != nil {
		return nil, err
	}

	return task, nil
}

// UpdateTaskStatus updates a task's status.
func (m *PhaseManager) UpdateTaskStatus(taskID string, status TaskStatus) error {
	task, err := m.LoadTask(taskID)
	if err != nil {
		return err
	}

	task.Status = status
	now := time.Now()

	switch status {
	case TaskStatusInProgress:
		task.Started = &now
	case TaskStatusCompleted:
		task.Completed = &now
	}

	return m.SaveTask(task)
}

// AddTaskMessage adds a message to a task.
func (m *PhaseManager) AddTaskMessage(taskID, author, content string) error {
	task, err := m.LoadTask(taskID)
	if err != nil {
		return err
	}

	task.Messages = append(task.Messages, TaskMessage{
		Author:    author,
		Content:   content,
		Timestamp: time.Now(),
	})

	return m.SaveTask(task)
}

// GetPhaseProgress returns the progress of a phase.
func (m *PhaseManager) GetPhaseProgress(phaseID string) (completed, total int, err error) {
	tasks, err := m.LoadTasksForPhase(phaseID)
	if err != nil {
		return 0, 0, err
	}

	total = len(tasks)
	for _, task := range tasks {
		if task.Status == TaskStatusCompleted {
			completed++
		}
	}

	return completed, total, nil
}

// GetReadyTasks returns tasks that are ready to be worked on.
func (m *PhaseManager) GetReadyTasks() ([]Task, error) {
	entries, err := os.ReadDir(m.tasksDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read tasks directory: %w", err)
	}

	var readyTasks []Task
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		task, err := m.LoadTask(strings.TrimSuffix(entry.Name(), ".yaml"))
		if err != nil {
			return nil, err
		}

		if task.Status == TaskStatusReady || task.Status == TaskStatusPending {
			// Check if dependencies are complete
			ready, err := m.isTaskReady(task)
			if err != nil {
				return nil, err
			}
			if ready {
				readyTasks = append(readyTasks, *task)
			}
		}
	}

	return readyTasks, nil
}

func (m *PhaseManager) isTaskReady(task *Task) (bool, error) {
	// Load the phase to check dependencies
	phase, err := m.LoadPhase(task.Phase)
	if err != nil {
		return false, err
	}

	// Check if all phase dependencies are complete
	for _, depPhaseID := range phase.Dependencies {
		depPhase, err := m.LoadPhase(depPhaseID)
		if err != nil {
			return false, err
		}

		if depPhase.Status != PhaseStatusCompleted {
			return false, nil
		}
	}

	return true, nil
}

func (m *PhaseManager) generateTaskID(phase *Phase) string {
	// Generate T-001, T-002, etc. based on existing tasks
	num := len(phase.Tasks) + 1
	return fmt.Sprintf("T-%03d", num)
}
