package logging

import (
	"fmt"
	"log/slog"
	"os"
	// "path/filepath"
	"encoding/json"
	"runtime"
	"runtime/debug"
	"sync"
	"time"
)

func getCaller() string {
	var caller string
	if _, file, line, ok := runtime.Caller(2); ok {
		// caller = fmt.Sprintf("%s:%d", filepath.Base(file), line)
		caller = fmt.Sprintf("%s:%d", file, line)
	} else {
		caller = "unknown"
	}
	return caller
}
func Info(msg string, args ...any) {
	source := getCaller()
	slog.Info(msg, append([]any{"source", source}, args...)...)
}

func Debug(msg string, args ...any) {
	// slog.Debug(msg, args...)
	source := getCaller()
	slog.Debug(msg, append([]any{"source", source}, args...)...)
}

func Warn(msg string, args ...any) {
	slog.Warn(msg, args...)
}

func Error(msg string, args ...any) {
	slog.Error(msg, args...)
}

func InfoPersist(msg string, args ...any) {
	args = append(args, persistKeyArg, true)
	slog.Info(msg, args...)
}

func DebugPersist(msg string, args ...any) {
	args = append(args, persistKeyArg, true)
	slog.Debug(msg, args...)
}

func WarnPersist(msg string, args ...any) {
	args = append(args, persistKeyArg, true)
	slog.Warn(msg, args...)
}

func ErrorPersist(msg string, args ...any) {
	args = append(args, persistKeyArg, true)
	slog.Error(msg, args...)
}

// RecoverPanic is a common function to handle panics gracefully.
// It logs the error, creates a panic log file with stack trace,
// and executes an optional cleanup function before returning.
func RecoverPanic(name string, cleanup func()) {
	if r := recover(); r != nil {
		// Log the panic
		ErrorPersist(fmt.Sprintf("Panic in %s: %v", name, r))

		// Create a timestamped panic log file
		timestamp := time.Now().Format("20060102-150405")
		filename := fmt.Sprintf("opencode-panic-%s-%s.log", name, timestamp)

		file, err := os.Create(filename)
		if err != nil {
			ErrorPersist(fmt.Sprintf("Failed to create panic log: %v", err))
		} else {
			defer file.Close()

			// Write panic information and stack trace
			fmt.Fprintf(file, "Panic in %s: %v\n\n", name, r)
			fmt.Fprintf(file, "Time: %s\n\n", time.Now().Format(time.RFC3339))
			fmt.Fprintf(file, "Stack Trace:\n%s\n", debug.Stack())

			InfoPersist(fmt.Sprintf("Panic details written to %s", filename))
		}

		// Execute cleanup function if provided
		if cleanup != nil {
			cleanup()
		}
	}
}

// Message Logging for Debug
var MessageDir string

func GetSessionPrefix(sessionId string) string {
	return sessionId[:8]
}

var sessionLogMutex sync.Mutex

func AppendToSessionLogFile(sessionId string, filename string, content string) string {
	if MessageDir == "" || sessionId == "" {
		return ""
	}
	sessionPrefix := GetSessionPrefix(sessionId)

	sessionLogMutex.Lock()
	defer sessionLogMutex.Unlock()

	sessionPath := fmt.Sprintf("%s/%s", MessageDir, sessionPrefix)
	if _, err := os.Stat(sessionPath); os.IsNotExist(err) {
		if err := os.MkdirAll(sessionPath, 0o766); err != nil {
			Error("Failed to create session directory", "dirpath", sessionPath, "error", err)
			return ""
		}
	}

	filePath := fmt.Sprintf("%s/%s", sessionPath, filename)

	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		Error("Failed to open session log file", "filepath", filePath, "error", err)
		return ""
	}
	defer f.Close()

	// Append chunk to file
	_, err = f.WriteString(content)
	if err != nil {
		Error("Failed to write chunk to session log file", "filepath", filePath, "error", err)
		return ""
	}
	return filePath
}

func WriteRequestMessageJson(sessionId string, requestSeqId int, message any) string {
	if MessageDir == "" || sessionId == "" || requestSeqId <= 0 {
		return ""
	}
	msgJson, err := json.Marshal(message)
	if err != nil {
		Error("Failed to marshal message", "session_id", sessionId, "request_seq_id", requestSeqId, "error", err)
		return ""
	}
	return WriteRequestMessage(sessionId, requestSeqId, string(msgJson))
}

func WriteRequestMessage(sessionId string, requestSeqId int, message string) string {
	if MessageDir == "" || sessionId == "" || requestSeqId <= 0 {
		return ""
	}
	filename := fmt.Sprintf("%d_request.json", requestSeqId)

	return AppendToSessionLogFile(sessionId, filename, message)
}

func AppendToStreamSessionLogJson(sessionId string, requestSeqId int, jsonableChunk any) string {
	if MessageDir == "" || sessionId == "" || requestSeqId <= 0 {
		return ""
	}
	chunkJson, err := json.Marshal(jsonableChunk)
	if err != nil {
		Error("Failed to marshal message", "session_id", sessionId, "request_seq_id", requestSeqId, "error", err)
		return ""
	}
	return AppendToStreamSessionLog(sessionId, requestSeqId, string(chunkJson))
}

func AppendToStreamSessionLog(sessionId string, requestSeqId int, chunk string) string {
	if MessageDir == "" || sessionId == "" || requestSeqId <= 0 {
		return ""
	}
	filename := fmt.Sprintf("%d_response_stream.log", requestSeqId)
	return AppendToSessionLogFile(sessionId, filename, chunk)
}

func WriteChatResponseJson(sessionId string, requestSeqId int, response any) string {
	if MessageDir == "" || sessionId == "" || requestSeqId <= 0 {
		return ""
	}
	responseJson, err := json.Marshal(response)
	if err != nil {
		Error("Failed to marshal response", "session_id", sessionId, "request_seq_id", requestSeqId, "error", err)
		return ""
	}
	filename := fmt.Sprintf("%d_response.json", requestSeqId)

	return AppendToSessionLogFile(sessionId, filename, string(responseJson))
}

func WriteToolResultsJson(sessionId string, requestSeqId int, toolResults any) string {
	if MessageDir == "" || sessionId == "" || requestSeqId <= 0 {
		return ""
	}
	toolResultsJson, err := json.Marshal(toolResults)
	if err != nil {
		Error("Failed to marshal tool results", "session_id", sessionId, "request_seq_id", requestSeqId, "error", err)
		return ""
	}
	filename := fmt.Sprintf("%d_tool_results.json", requestSeqId)
	return AppendToSessionLogFile(sessionId, filename, string(toolResultsJson))
}
