package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// parseTasks reads tasks from various input sources: CLI args, STDIN, or file
func parseTasks(args []string, tasksFile string, tasksJSON bool) ([]string, error) {
	// Priority 1: Tasks from file
	if tasksFile != "" {
		return parseTasksFromFile(tasksFile, tasksJSON)
	}

	// Priority 2: Tasks from STDIN (when args == ["-"])
	if len(args) == 1 && args[0] == "-" {
		return parseTasksFromReader(os.Stdin, tasksJSON)
	}

	// Priority 3: Tasks from CLI arguments (default)
	if len(args) > 0 {
		return args, nil
	}

	return nil, fmt.Errorf("no tasks provided")
}

// parseTasksFromFile reads tasks from a file
func parseTasksFromFile(filepath string, asJSON bool) ([]string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open tasks file: %w", err)
	}
	defer file.Close()

	return parseTasksFromReader(file, asJSON)
}

// parseTasksFromReader reads tasks from an io.Reader
func parseTasksFromReader(r io.Reader, asJSON bool) ([]string, error) {
	if asJSON {
		return parseJSONTasks(r)
	}
	return parseLineTasks(r)
}

// parseJSONTasks parses tasks from JSON array format
func parseJSONTasks(r io.Reader) ([]string, error) {
	var tasks []string
	decoder := json.NewDecoder(r)

	if err := decoder.Decode(&tasks); err != nil {
		return nil, fmt.Errorf("failed to parse JSON tasks: %w", err)
	}

	// Filter out empty strings
	filtered := make([]string, 0, len(tasks))
	for _, task := range tasks {
		task = strings.TrimSpace(task)
		if task != "" {
			filtered = append(filtered, task)
		}
	}

	return filtered, nil
}

// parseLineTasks parses tasks from newline-separated format
func parseLineTasks(r io.Reader) ([]string, error) {
	var tasks []string
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line != "" && !strings.HasPrefix(line, "#") {
			tasks = append(tasks, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read tasks: %w", err)
	}

	return tasks, nil
}
