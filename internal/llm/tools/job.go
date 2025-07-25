package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/shell"
	"github.com/charmbracelet/hotdiva2000"
)

type JobStartParams struct {
	Command   string `json:"command"`
	Directory string `json:"directory"`
}

type JobEndParams struct {
	JobID string `json:"job_id"`
}

type JobViewParams struct {
	JobID string `json:"job_id"`
	Lines int    `json:"lines"`
}

type JobStartResponse struct {
	JobID         string `json:"job_id"`
	Command       string `json:"command"`
	Directory     string `json:"directory"`
	InitialOutput string `json:"initial_output"`
}

type JobViewResponse struct {
	JobID     string `json:"job_id"`
	Command   string `json:"command"`
	Directory string `json:"directory"`
	Output    string `json:"output"`
	IsActive  bool   `json:"is_active"`
	Runtime   string `json:"runtime"`
	ExitError string `json:"exit_error,omitempty"`
}

type JobEndResponse struct {
	JobID     string `json:"job_id"`
	Command   string `json:"command"`
	Directory string `json:"directory"`
	Runtime   string `json:"runtime"`
}

type jobInstance struct {
	id           string
	shell        *shell.JobShell
	outputFile   *os.File
	cancel       context.CancelFunc
	startTime    time.Time
	completeTime *time.Time // When the job completed (nil if still running)
	command      string
	directory    string
	completed    bool
	exitError    error // Store the exit error if any
	mu           sync.RWMutex
}

type jobManager struct {
	jobs map[string]*jobInstance
	mu   sync.RWMutex
}

var (
	jobMgr       *jobManager
	jobMgrOnce   sync.Once
	jobCounter   int
	jobCounterMu sync.Mutex
)

func getJobManager() *jobManager {
	jobMgrOnce.Do(func() {
		jobMgr = &jobManager{
			jobs: make(map[string]*jobInstance),
		}
		// Start cleanup goroutine
		go jobMgr.cleanupCompletedJobs()
	})
	return jobMgr
}

// cleanupCompletedJobs periodically removes completed jobs older than 1 hour
func (jm *jobManager) cleanupCompletedJobs() {
	ticker := time.NewTicker(10 * time.Minute) // Check every 10 minutes
	defer ticker.Stop()

	for range ticker.C {
		jm.mu.Lock()
		for jobID, job := range jm.jobs {
			job.mu.RLock()
			shouldCleanup := job.completed && job.completeTime != nil &&
				time.Since(*job.completeTime) > time.Hour
			job.mu.RUnlock()

			if shouldCleanup {
				job.mu.Lock()
				// Clean up resources
				if err := job.outputFile.Close(); err != nil {
					fmt.Printf("Error closing output file for job %s: %v\n", jobID, err)
				}
				if err := os.Remove(job.outputFile.Name()); err != nil {
					fmt.Printf("Error removing output file for job %s: %v\n", jobID, err)
				}
				job.mu.Unlock()
				delete(jm.jobs, jobID)
				fmt.Printf("Cleaned up completed job %s\n", jobID)
			}
		}
		jm.mu.Unlock()
	}
}

type jobStartTool struct {
	permissions permission.Service
}

type jobEndTool struct{}

type jobViewTool struct{}

const (
	JobStartToolName = "job_start"
	JobEndToolName   = "job_end"
	JobViewToolName  = "job_view"
	DefaultViewLines = 100
)

func NewJobStartTool(permissions permission.Service) BaseTool {
	return &jobStartTool{permissions: permissions}
}

func NewJobEndTool() BaseTool {
	return &jobEndTool{}
}

func NewJobViewTool() BaseTool {
	return &jobViewTool{}
}

func (j *jobStartTool) Name() string {
	return JobStartToolName
}

func (j *jobStartTool) Info() ToolInfo {
	return ToolInfo{
		Name: JobStartToolName,
		Description: `Starts a long-running shell command as a background job and returns a job ID for management.

This tool is designed for commands that run continuously like servers, build watchers, or monitoring processes.

WHEN TO USE:
- Starting development servers (e.g., "npm run dev", "go run main.go")
- Running build watchers (e.g., "npm run watch", "tsc --watch")
- Starting background processes that need to run while you work on other tasks
- Any command that doesn't exit immediately and produces ongoing output

IMPORTANT NOTES:
- The command runs in the background and continues until explicitly stopped with job_end
- All output (stdout and stderr) is captured and can be viewed with job_view
- Jobs are automatically cleaned up when the main program exits
- You must use job_end to properly terminate jobs when done
- The command runs in the specified directory (or current working directory if not specified)

SECURITY:
- Requires user permission for command execution
- No banned commands list (unlike bash tool) since these are typically development commands

EXAMPLES:
- Start a web server: {"command": "npm run dev"}
- Start a Go application in specific directory: {"command": "go run main.go", "directory": "/path/to/project"}
- Watch for file changes: {"command": "npm run watch"}`,
		Parameters: map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "The shell command to run as a background job",
			},
			"directory": map[string]any{
				"type":        "string",
				"description": "The directory to run the command in (must be absolute path, defaults to current working directory)",
			},
		},
		Required: []string{"command"},
	}
}

func (j *jobEndTool) Name() string {
	return JobEndToolName
}

func (j *jobEndTool) Info() ToolInfo {
	return ToolInfo{
		Name: JobEndToolName,
		Description: `Terminates a running background job by its job ID.

This tool stops a job that was started with job_start and cleans up all associated resources.

WHEN TO USE:
- Stop a development server when switching tasks
- Terminate a build watcher when no longer needed
- Clean up any background job that's no longer required

IMPORTANT NOTES:
- The job process is terminated gracefully (SIGTERM) first, then forcefully (SIGKILL) if needed
- All output files and resources are cleaned up automatically
- Once ended, the job ID becomes invalid and cannot be reused
- If the job has already exited, this command will still clean up resources`,
		Parameters: map[string]any{
			"job_id": map[string]any{
				"type":        "string",
				"description": "The job ID returned by job_start",
			},
		},
		Required: []string{"job_id"},
	}
}

func (j *jobViewTool) Name() string {
	return JobViewToolName
}

func (j *jobViewTool) Info() ToolInfo {
	return ToolInfo{
		Name: JobViewToolName,
		Description: `Views the recent output from a running or completed background job.

This tool shows the last N lines of output (both stdout and stderr) from a job started with job_start.

WHEN TO USE:
- Check if a development server started successfully
- Monitor build output or error messages
- Debug issues with background processes
- View logs from long-running commands

IMPORTANT NOTES:
- Shows the most recent output lines (default: 100 lines)
- Includes both stdout and stderr output mixed in chronological order
- Works for both active and recently completed jobs
- Output is captured from the moment the job started`,
		Parameters: map[string]any{
			"job_id": map[string]any{
				"type":        "string",
				"description": "The job ID returned by job_start",
			},
			"lines": map[string]any{
				"type":        "number",
				"description": "Number of recent output lines to show (default: 100)",
			},
		},
		Required: []string{"job_id"},
	}
}

func (j *jobStartTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params JobStartParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse("invalid parameters"), nil
	}

	if params.Command == "" {
		return NewTextErrorResponse("missing command"), nil
	}

	// Handle directory parameter
	workingDir := config.Get().WorkingDir()
	if params.Directory != "" {
		// Validate that directory is an absolute path
		if !filepath.IsAbs(params.Directory) {
			return NewTextErrorResponse("directory must be an absolute path"), nil
		}

		// Check if directory exists
		dirInfo, err := os.Stat(params.Directory)
		if err != nil {
			if os.IsNotExist(err) {
				return NewTextErrorResponse(fmt.Sprintf("directory not found: %s", params.Directory)), nil
			}
			return NewTextErrorResponse(fmt.Sprintf("error accessing directory: %v", err)), nil
		}

		// Check if it's actually a directory
		if !dirInfo.IsDir() {
			return NewTextErrorResponse(fmt.Sprintf("path is not a directory: %s", params.Directory)), nil
		}

		workingDir = params.Directory
	}

	sessionID, messageID := GetContextValues(ctx)
	if sessionID == "" || messageID == "" {
		return ToolResponse{}, fmt.Errorf("session ID and message ID are required for job management")
	}

	// Request permission
	p := j.permissions.Request(
		permission.CreatePermissionRequest{
			SessionID:   sessionID,
			Path:        workingDir,
			ToolName:    JobStartToolName,
			Action:      "execute",
			Description: fmt.Sprintf("Start background job in %s: %s", workingDir, params.Command),
			Params: JobStartParams{
				Command:   params.Command,
				Directory: params.Directory,
			},
		},
	)
	if !p {
		return ToolResponse{}, permission.ErrorPermissionDenied
	}

	jobID := hotdiva2000.GenerateWithOptions(hotdiva2000.Options{
		PrefixThreshold: 0.2,
		SuffixThreshold: 0.2,
		Results:         1,
	})[0]

	// Create temporary file for output
	tempDir := os.TempDir()
	outputFile, err := os.CreateTemp(tempDir, fmt.Sprintf("crush-job-%s-*.log", jobID))
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("failed to create output file: %v", err)), nil
	}

	// Create context for the job
	jobCtx, cancel := context.WithCancel(context.Background())

	// Create job shell that writes to the temp file
	jobShell, err := shell.NewJobShell(workingDir, outputFile, outputFile)
	if err != nil {
		cancel()
		outputFile.Close()
		os.Remove(outputFile.Name())
		return NewTextErrorResponse(fmt.Sprintf("failed to create job shell: %v", err)), nil
	}

	// Start the command asynchronously
	if jobErr := jobShell.ExecAsync(jobCtx, params.Command); jobErr != nil {
		cancel()
		outputFile.Close()
		os.Remove(outputFile.Name())
		return NewTextErrorResponse(fmt.Sprintf("failed to start job: %v", jobErr)), nil
	}

	// Store job instance
	job := &jobInstance{
		id:         jobID,
		shell:      jobShell,
		outputFile: outputFile,
		cancel:     cancel,
		startTime:  time.Now(),
		command:    params.Command,
		directory:  workingDir,
		completed:  false,
	}

	mgr := getJobManager()
	mgr.mu.Lock()
	mgr.jobs[jobID] = job
	mgr.mu.Unlock()

	// Start monitoring for job completion
	go func() {
		// Wait for the job to complete
		exitErr := <-jobShell.Done()

		// Mark job as completed
		now := time.Now()
		job.mu.Lock()
		job.completed = true
		job.exitError = exitErr
		job.completeTime = &now
		job.mu.Unlock()
	}()

	// Wait a moment for initial output
	time.Sleep(500 * time.Millisecond)

	// Read initial output (first few lines)
	initialOutput, _ := readLastLines(outputFile.Name(), 10)
	if initialOutput == "" {
		initialOutput = "Job started, no output yet."
	}

	// Create structured response
	response := JobStartResponse{
		JobID:         jobID,
		Command:       params.Command,
		Directory:     workingDir,
		InitialOutput: initialOutput,
	}

	responseBytes, err := json.Marshal(response)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("failed to marshal response: %v", err)), nil
	}

	return NewTextResponse(string(responseBytes)), nil
}

func (j *jobEndTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params JobEndParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse("invalid parameters"), nil
	}

	if params.JobID == "" {
		return NewTextErrorResponse("missing job_id"), nil
	}

	mgr := getJobManager()
	mgr.mu.Lock()
	job, exists := mgr.jobs[params.JobID]
	if !exists {
		mgr.mu.Unlock()
		return NewTextErrorResponse(fmt.Sprintf("job with ID %s not found", params.JobID)), nil
	}

	// Remove from jobs map first
	delete(mgr.jobs, params.JobID)
	mgr.mu.Unlock()

	// Cancel the job context (this will stop the shell execution)
	job.cancel()

	// Mark job as completed
	job.mu.Lock()
	job.completed = true
	job.mu.Unlock()

	// Wait a moment for the shell to finish
	time.Sleep(100 * time.Millisecond)

	// Clean up resources
	job.outputFile.Close()
	os.Remove(job.outputFile.Name())

	duration := time.Since(job.startTime)

	// Create structured response
	response := JobEndResponse{
		JobID:     params.JobID,
		Command:   job.command,
		Directory: job.directory,
		Runtime:   duration.Round(time.Second).String(),
	}

	responseBytes, err := json.Marshal(response)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("failed to marshal response: %v", err)), nil
	}

	return NewTextResponse(string(responseBytes)), nil
}

func (j *jobViewTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params JobViewParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse("invalid parameters"), nil
	}

	if params.JobID == "" {
		return NewTextErrorResponse("missing job_id"), nil
	}

	if params.Lines <= 0 {
		params.Lines = DefaultViewLines
	}

	mgr := getJobManager()
	mgr.mu.RLock()
	job, exists := mgr.jobs[params.JobID]
	mgr.mu.RUnlock()

	if !exists {
		return NewTextErrorResponse(fmt.Sprintf("job with ID %s not found", params.JobID)), nil
	}

	// Check if job is still active
	job.mu.RLock()
	isActive := !job.completed
	exitError := job.exitError
	job.mu.RUnlock()

	// Read the last N lines from the output file
	output, err := readLastLines(job.outputFile.Name(), params.Lines)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("failed to read job output: %v", err)), nil
	}

	// Create structured response for better rendering
	duration := time.Since(job.startTime)
	response := JobViewResponse{
		JobID:     params.JobID,
		Command:   job.command,
		Directory: job.directory,
		Output:    output,
		IsActive:  isActive,
		Runtime:   duration.Round(time.Second).String(),
	}

	// Add exit error if the job completed with an error
	if !isActive && exitError != nil {
		response.ExitError = exitError.Error()
	}

	responseBytes, err := json.Marshal(response)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("failed to marshal response: %v", err)), nil
	}

	return NewTextResponse(string(responseBytes)), nil
}

func readLastLines(filename string, n int) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Get file size
	stat, err := file.Stat()
	if err != nil {
		return "", err
	}

	if stat.Size() == 0 {
		return "No output yet", nil
	}

	// For small files, just read everything
	if stat.Size() < 8192 {
		content, err := io.ReadAll(file)
		if err != nil {
			return "", err
		}
		lines := strings.Split(string(content), "\n")
		if len(lines) <= n {
			return string(content), nil
		}
		return strings.Join(lines[len(lines)-n-1:], "\n"), nil
	}

	// For larger files, read from the end
	var lines []string
	scanner := bufio.NewScanner(file)

	// Read all lines (this could be optimized for very large files)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	if len(lines) <= n {
		return strings.Join(lines, "\n"), nil
	}

	return strings.Join(lines[len(lines)-n:], "\n"), nil
}

// CleanupAllJobs terminates all running jobs - called when the program exits
func CleanupAllJobs() {
	mgr := getJobManager()
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	for jobID, job := range mgr.jobs {
		job.cancel()

		// Mark job as completed
		job.mu.Lock()
		job.completed = true
		job.mu.Unlock()

		// Wait briefly for graceful shutdown
		time.Sleep(100 * time.Millisecond)

		job.outputFile.Close()
		os.Remove(job.outputFile.Name())
		delete(mgr.jobs, jobID)
	}
}
