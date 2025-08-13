package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/crush/internal/todo"
)

// TodoWriteParams represents parameters for creating/updating TODOs
type TodoWriteParams struct {
	Todos []TodoItem `json:"todos"`
}

type TodoItem struct {
	ID      string `json:"id,omitempty"` // Optional for updates
	Content string `json:"content"`      // Required
	Status  string `json:"status"`       // Required: pending, in_progress, completed
}

// TodoListParams represents parameters for listing TODOs
type TodoListParams struct {
	FilterStatus string `json:"filter_status,omitempty"` // Optional: pending, in_progress, completed
}

// TodoDeleteParams represents parameters for deleting TODOs
type TodoDeleteParams struct {
	IDs []string `json:"ids"` // Required: array of TODO IDs to delete
}

const (
	TodoWriteToolName  = "todo_write"
	TodoListToolName   = "todo_list"
	TodoDeleteToolName = "todo_delete"
)

const todoWriteDescription = `Create or update TODO task items to track your planned course of action and progress.

WHEN TO USE THIS TOOL:
- Use when planning multi-step tasks to keep track of what needs to be done
- Perfect for breaking down complex problems into manageable steps
- Helpful for tracking progress through implementation phases
- Use when you want to maintain context across conversation turns

HOW TO USE:
- Provide an array of TODO items with content and status
- For new TODOs, omit the ID field (will be auto-generated)
- For updates, include the ID of the existing TODO item
- Set appropriate status: 'pending', 'in_progress', or 'completed'

FEATURES:
- TODOs are scoped to the current session and project directory
- Status tracking helps monitor progress (pending → in_progress → completed)
- Persisted across application restarts
- Helps maintain context and planning state

LIMITATIONS:
- TODOs are tied to the current session and working directory
- Cannot share TODOs across different sessions or projects
- Limited to text-based task descriptions

TIPS:
- Use descriptive content that explains what needs to be accomplished
- Mark items as 'in_progress' when actively working on them
- Mark as 'completed' only when fully done
- Use 'pending' for planned but not yet started tasks`

const todoListDescription = `List TODO task items for the current session and project directory.

WHEN TO USE THIS TOOL:
- Use to see what TODOs are currently tracked for this session
- Helpful for checking progress and deciding what to work on next
- Good for reviewing planned tasks before starting work

HOW TO USE:
- Call without parameters to list all TODOs for the current session
- Optionally filter by status (pending, in_progress, completed)

FEATURES:
- Shows all TODOs for the current session and project directory
- Optional status filtering to focus on specific types of tasks
- Displays creation and update timestamps
- Ordered by creation time (oldest first)

LIMITATIONS:
- Only shows TODOs for the current session and project
- Cannot list TODOs from other sessions or projects

TIPS:
- Use status filters to focus on specific types of tasks
- Review regularly to track progress and plan next steps
- Combine with todo_write to update task statuses`

const todoDeleteDescription = `Remove TODO task items that are no longer needed or relevant.

WHEN TO USE THIS TOOL:
- Use to clean up completed TODOs that are no longer needed
- Remove irrelevant or outdated tasks
- Clear TODOs that were created by mistake

HOW TO USE:
- Provide an array of TODO IDs to delete
- Get TODO IDs from the todo_list tool first

FEATURES:
- Permanently removes TODOs from the current session
- Can delete multiple TODOs in a single operation
- Immediate removal with no recovery option

LIMITATIONS:
- Deletion is permanent - cannot be undone
- Must know the exact TODO IDs to delete
- Only affects TODOs in the current session and project

TIPS:
- Use todo_list first to see available TODOs and their IDs
- Consider updating status to 'completed' instead of deleting
- Only delete TODOs that are truly no longer needed`

// TodoWriteTool handles creating and updating TODOs
type todoWriteTool struct {
	todoService todo.Service
}

// TodoListTool handles listing TODOs
type todoListTool struct {
	todoService todo.Service
}

// TodoDeleteTool handles deleting TODOs
type todoDeleteTool struct {
	todoService todo.Service
}

func NewTodoWriteTool(todoService todo.Service) BaseTool {
	return &todoWriteTool{
		todoService: todoService,
	}
}

func NewTodoListTool(todoService todo.Service) BaseTool {
	return &todoListTool{
		todoService: todoService,
	}
}

func NewTodoDeleteTool(todoService todo.Service) BaseTool {
	return &todoDeleteTool{
		todoService: todoService,
	}
}

func (t *todoWriteTool) Name() string {
	return TodoWriteToolName
}

func (t *todoWriteTool) Info() ToolInfo {
	return ToolInfo{
		Name:        TodoWriteToolName,
		Description: todoWriteDescription,
		Parameters: map[string]any{
			"todos": map[string]any{
				"type":        "array",
				"description": "Array of TODO items to create or update",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id": map[string]any{
							"type":        "string",
							"description": "ID of existing TODO for updates (omit for new TODOs)",
						},
						"content": map[string]any{
							"type":        "string",
							"description": "Description of what needs to be accomplished",
						},
						"status": map[string]any{
							"type":        "string",
							"enum":        []string{"pending", "in_progress", "completed"},
							"description": "Current status of the TODO item",
						},
					},
					"required": []string{"content", "status"},
				},
			},
		},
		Required: []string{"todos"},
	}
}

func (t *todoWriteTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params TodoWriteParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("error parsing parameters: %s", err)), nil
	}

	if len(params.Todos) == 0 {
		return NewTextErrorResponse("todos array cannot be empty"), nil
	}

	sessionID, _ := GetContextValues(ctx)
	if sessionID == "" {
		return NewTextErrorResponse("session ID is required"), nil
	}

	var results []string
	var errors []string

	for _, todoItem := range params.Todos {
		if todoItem.Content == "" {
			errors = append(errors, "TODO content cannot be empty")
			continue
		}

		status := todo.Status(todoItem.Status)
		if status != todo.StatusPending && status != todo.StatusInProgress && status != todo.StatusCompleted {
			errors = append(errors, fmt.Sprintf("invalid status '%s'. Must be: pending, in_progress, or completed", todoItem.Status))
			continue
		}

		if todoItem.ID == "" {
			// Create new TODO
			createdTodo, err := t.todoService.Create(ctx, todo.CreateTodoParams{
				SessionID: sessionID,
				Content:   todoItem.Content,
				Status:    status,
			})
			if err != nil {
				errors = append(errors, fmt.Sprintf("failed to create TODO: %s", err))
				continue
			}
			results = append(results, fmt.Sprintf("Created TODO '%s' with status '%s' (ID: %s)", createdTodo.Content, createdTodo.Status, createdTodo.ID))
		} else {
			// Update existing TODO
			updatedTodo, err := t.todoService.Update(ctx, todoItem.ID, todo.UpdateTodoParams{
				Content: todoItem.Content,
				Status:  status,
			})
			if err != nil {
				errors = append(errors, fmt.Sprintf("failed to update TODO %s: %s", todoItem.ID, err))
				continue
			}
			results = append(results, fmt.Sprintf("Updated TODO '%s' with status '%s' (ID: %s)", updatedTodo.Content, updatedTodo.Status, updatedTodo.ID))
		}
	}

	response := ""
	if len(results) > 0 {
		response = strings.Join(results, "\n")
	}
	if len(errors) > 0 {
		if response != "" {
			response += "\n\nErrors:\n"
		}
		response += strings.Join(errors, "\n")
	}

	if len(errors) > 0 && len(results) == 0 {
		return NewTextErrorResponse(response), nil
	}

	return NewTextResponse(response), nil
}

func (t *todoListTool) Name() string {
	return TodoListToolName
}

func (t *todoListTool) Info() ToolInfo {
	return ToolInfo{
		Name:        TodoListToolName,
		Description: todoListDescription,
		Parameters: map[string]any{
			"filter_status": map[string]any{
				"type":        "string",
				"enum":        []string{"pending", "in_progress", "completed"},
				"description": "Optional filter by status",
			},
		},
		Required: []string{},
	}
}

func (t *todoListTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params TodoListParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("error parsing parameters: %s", err)), nil
	}

	sessionID, _ := GetContextValues(ctx)
	if sessionID == "" {
		return NewTextErrorResponse("session ID is required"), nil
	}

	listParams := todo.ListTodosParams{
		SessionID: sessionID,
	}

	if params.FilterStatus != "" {
		status := todo.Status(params.FilterStatus)
		if status != todo.StatusPending && status != todo.StatusInProgress && status != todo.StatusCompleted {
			return NewTextErrorResponse(fmt.Sprintf("invalid filter_status '%s'. Must be: pending, in_progress, or completed", params.FilterStatus)), nil
		}
		listParams.Status = &status
	}

	todos, err := t.todoService.List(ctx, listParams)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("failed to list TODOs: %s", err)), nil
	}

	if len(todos) == 0 {
		message := "No TODOs found for the current session and project"
		if params.FilterStatus != "" {
			message += fmt.Sprintf(" with status '%s'", params.FilterStatus)
		}
		return NewTextResponse(message + "."), nil
	}

	var response strings.Builder
	response.WriteString(fmt.Sprintf("TODOs for current session and project (%d total):\n\n", len(todos)))

	for _, todoItem := range todos {
		statusIcon := getStatusIcon(todoItem.Status)
		response.WriteString(fmt.Sprintf("%s %s (ID: %s)\n", statusIcon, todoItem.Content, todoItem.ID))
		response.WriteString(fmt.Sprintf("   Status: %s\n", todoItem.Status))
		response.WriteString(fmt.Sprintf("   Created: %s\n", formatTimestamp(todoItem.CreatedAt)))
		if todoItem.UpdatedAt != todoItem.CreatedAt {
			response.WriteString(fmt.Sprintf("   Updated: %s\n", formatTimestamp(todoItem.UpdatedAt)))
		}
		response.WriteString("\n")
	}

	return NewTextResponse(response.String()), nil
}

func (t *todoDeleteTool) Name() string {
	return TodoDeleteToolName
}

func (t *todoDeleteTool) Info() ToolInfo {
	return ToolInfo{
		Name:        TodoDeleteToolName,
		Description: todoDeleteDescription,
		Parameters: map[string]any{
			"ids": map[string]any{
				"type":        "array",
				"description": "Array of TODO IDs to delete",
				"items": map[string]any{
					"type": "string",
				},
			},
		},
		Required: []string{"ids"},
	}
}

func (t *todoDeleteTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params TodoDeleteParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("error parsing parameters: %s", err)), nil
	}

	if len(params.IDs) == 0 {
		return NewTextErrorResponse("ids array cannot be empty"), nil
	}

	var results []string
	var errors []string

	for _, id := range params.IDs {
		if id == "" {
			errors = append(errors, "TODO ID cannot be empty")
			continue
		}

		err := t.todoService.Delete(ctx, id)
		if err != nil {
			errors = append(errors, fmt.Sprintf("failed to delete TODO %s: %s", id, err))
			continue
		}
		results = append(results, fmt.Sprintf("Deleted TODO with ID: %s", id))
	}

	response := ""
	if len(results) > 0 {
		response = strings.Join(results, "\n")
	}
	if len(errors) > 0 {
		if response != "" {
			response += "\n\nErrors:\n"
		}
		response += strings.Join(errors, "\n")
	}

	if len(errors) > 0 && len(results) == 0 {
		return NewTextErrorResponse(response), nil
	}

	return NewTextResponse(response), nil
}

func getStatusIcon(status todo.Status) string {
	switch status {
	case todo.StatusPending:
		return "⏳"
	case todo.StatusInProgress:
		return "🔄"
	case todo.StatusCompleted:
		return "✅"
	default:
		return "❓"
	}
}

func formatTimestamp(timestamp int64) string {
	// Convert milliseconds to seconds for time formatting
	return fmt.Sprintf("%d", timestamp)
}
