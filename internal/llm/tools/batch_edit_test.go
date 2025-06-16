package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock implementations for testing
type mockPermissionService struct {
	allowed bool
}

func (m *mockPermissionService) Request(req permission.CreatePermissionRequest) bool {
	return m.allowed
}

func (m *mockPermissionService) AutoApproveSession(sessionID string) {}

func (m *mockPermissionService) Subscribe(ctx context.Context) <-chan pubsub.Event[permission.PermissionRequest] {
	ch := make(chan pubsub.Event[permission.PermissionRequest])
	close(ch)
	return ch
}

func (m *mockPermissionService) GrantPersistent(permission permission.PermissionRequest) {}

func (m *mockPermissionService) Grant(permission permission.PermissionRequest) {}

func (m *mockPermissionService) Deny(permission permission.PermissionRequest) {}

type mockHistoryService struct{}

func (m *mockHistoryService) Create(ctx context.Context, sessionID, path, content string) (history.File, error) {
	return history.File{}, nil
}

func (m *mockHistoryService) CreateVersion(ctx context.Context, sessionID, path, content string) (history.File, error) {
	return history.File{}, nil
}

func (m *mockHistoryService) GetByPathAndSession(ctx context.Context, path, sessionID string) (history.File, error) {
	return history.File{Content: ""}, nil
}

func (m *mockHistoryService) Subscribe(ctx context.Context) <-chan pubsub.Event[history.File] {
	ch := make(chan pubsub.Event[history.File])
	close(ch)
	return ch
}

func (m *mockHistoryService) Get(ctx context.Context, id string) (history.File, error) {
	return history.File{}, nil
}

func (m *mockHistoryService) ListBySession(ctx context.Context, sessionID string) ([]history.File, error) {
	return []history.File{}, nil
}

func (m *mockHistoryService) ListLatestSessionFiles(ctx context.Context, sessionID string) ([]history.File, error) {
	return []history.File{}, nil
}

func (m *mockHistoryService) Update(ctx context.Context, file history.File) (history.File, error) {
	return file, nil
}

func (m *mockHistoryService) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *mockHistoryService) DeleteSessionFiles(ctx context.Context, sessionID string) error {
	return nil
}

func setupTestEnvironment(t *testing.T) (string, func()) {
	tmpDir, err := os.MkdirTemp("", "batch-edit-test")
	require.NoError(t, err)

	// Initialize config with the temp directory
	_, err = config.Load(tmpDir, false)
	require.NoError(t, err)

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return tmpDir, cleanup
}

func createTestContext() context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, SessionIDContextKey, "test-session")
	ctx = context.WithValue(ctx, MessageIDContextKey, "test-message")
	return ctx
}

func createBatchEditTool(allowed bool) *batchEditTool {
	return &batchEditTool{
		lspClients:  make(map[string]*lsp.Client),
		permissions: &mockPermissionService{allowed: allowed},
		files:       &mockHistoryService{},
	}
}

func TestBatchEditTool_Info(t *testing.T) {
	tool := createBatchEditTool(true)
	info := tool.Info()

	assert.Equal(t, BatchEditToolName, info.Name)
	assert.Contains(t, info.Description, "multiple edit operations")
	assert.Contains(t, info.Parameters, "operations")
	assert.Equal(t, []string{"operations"}, info.Required)
}

func TestBatchEditTool_SingleFileMultipleOperations(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	tool := createBatchEditTool(true)
	ctx := createTestContext()

	// Create test file
	testFile := filepath.Join(tmpDir, "test.txt")
	content := `line 1
line 2
line 3
line 4`
	err := os.WriteFile(testFile, []byte(content), 0o644)
	require.NoError(t, err)

	// Record file read
	recordFileRead(testFile)

	// Prepare batch operations
	operations := []BatchEditOperation{
		{
			FilePath:  testFile,
			OldString: "line 1",
			NewString: "modified line 1",
		},
		{
			FilePath:  testFile,
			OldString: "line 3",
			NewString: "modified line 3",
		},
	}

	params := BatchEditParams{Operations: operations}
	input, err := json.Marshal(params)
	require.NoError(t, err)

	call := ToolCall{
		ID:    "test",
		Name:  BatchEditToolName,
		Input: string(input),
	}

	// Execute batch edit
	response, err := tool.Run(ctx, call)
	require.NoError(t, err)
	assert.False(t, response.IsError)

	// Verify file content
	resultContent, err := os.ReadFile(testFile)
	require.NoError(t, err)

	expected := `modified line 1
line 2
modified line 3
line 4`
	assert.Equal(t, expected, string(resultContent))

	// Verify metadata
	var metadata BatchEditResponseMetadata
	err = json.Unmarshal([]byte(response.Metadata), &metadata)
	require.NoError(t, err)

	assert.Equal(t, 2, metadata.TotalSuccess)
	assert.Equal(t, 0, metadata.TotalFailed)
	assert.Equal(t, 1, metadata.TotalFiles)
	assert.Len(t, metadata.Results, 2)
}

func TestBatchEditTool_MultipleFiles(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	tool := createBatchEditTool(true)
	ctx := createTestContext()

	// Create test files
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	content1 := "hello world"
	content2 := "foo bar"

	err := os.WriteFile(file1, []byte(content1), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(file2, []byte(content2), 0o644)
	require.NoError(t, err)

	// Record file reads
	recordFileRead(file1)
	recordFileRead(file2)

	// Prepare batch operations
	operations := []BatchEditOperation{
		{
			FilePath:  file1,
			OldString: "world",
			NewString: "universe",
		},
		{
			FilePath:  file2,
			OldString: "bar",
			NewString: "baz",
		},
	}

	params := BatchEditParams{Operations: operations}
	input, err := json.Marshal(params)
	require.NoError(t, err)

	call := ToolCall{
		ID:    "test",
		Name:  BatchEditToolName,
		Input: string(input),
	}

	// Execute batch edit
	response, err := tool.Run(ctx, call)
	require.NoError(t, err)
	assert.False(t, response.IsError)

	// Verify file contents
	result1, err := os.ReadFile(file1)
	require.NoError(t, err)
	assert.Equal(t, "hello universe", string(result1))

	result2, err := os.ReadFile(file2)
	require.NoError(t, err)
	assert.Equal(t, "foo baz", string(result2))

	// Verify metadata
	var metadata BatchEditResponseMetadata
	err = json.Unmarshal([]byte(response.Metadata), &metadata)
	require.NoError(t, err)

	assert.Equal(t, 2, metadata.TotalSuccess)
	assert.Equal(t, 0, metadata.TotalFailed)
	assert.Equal(t, 2, metadata.TotalFiles)
}

func TestBatchEditTool_FileCreation(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	tool := createBatchEditTool(true)
	ctx := createTestContext()

	// Create new file operation
	newFile := filepath.Join(tmpDir, "new.txt")
	operations := []BatchEditOperation{
		{
			FilePath:  newFile,
			OldString: "", // Empty for file creation
			NewString: "new file content",
		},
	}

	params := BatchEditParams{Operations: operations}
	input, err := json.Marshal(params)
	require.NoError(t, err)

	call := ToolCall{
		ID:    "test",
		Name:  BatchEditToolName,
		Input: string(input),
	}

	// Execute batch edit
	response, err := tool.Run(ctx, call)
	require.NoError(t, err)
	assert.False(t, response.IsError)

	// Verify file was created
	content, err := os.ReadFile(newFile)
	require.NoError(t, err)
	assert.Equal(t, "new file content", string(content))
}

func TestBatchEditTool_ContentDeletion(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	tool := createBatchEditTool(true)
	ctx := createTestContext()

	// Create test file
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "before DELETE_ME after"
	err := os.WriteFile(testFile, []byte(content), 0o644)
	require.NoError(t, err)

	recordFileRead(testFile)

	// Delete content operation
	operations := []BatchEditOperation{
		{
			FilePath:  testFile,
			OldString: "DELETE_ME ",
			NewString: "", // Empty for deletion
		},
	}

	params := BatchEditParams{Operations: operations}
	input, err := json.Marshal(params)
	require.NoError(t, err)

	call := ToolCall{
		ID:    "test",
		Name:  BatchEditToolName,
		Input: string(input),
	}

	// Execute batch edit
	response, err := tool.Run(ctx, call)
	require.NoError(t, err)
	assert.False(t, response.IsError)

	// Verify content was deleted
	result, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, "before after", string(result))
}

func TestBatchEditTool_PermissionDenied(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	tool := createBatchEditTool(false) // Permissions denied
	ctx := createTestContext()

	// Create test file
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("content"), 0o644)
	require.NoError(t, err)

	recordFileRead(testFile)

	operations := []BatchEditOperation{
		{
			FilePath:  testFile,
			OldString: "content",
			NewString: "modified",
		},
	}

	params := BatchEditParams{Operations: operations}
	input, err := json.Marshal(params)
	require.NoError(t, err)

	call := ToolCall{
		ID:    "test",
		Name:  BatchEditToolName,
		Input: string(input),
	}

	// Execute batch edit
	_, err = tool.Run(ctx, call)
	require.Error(t, err)
	assert.Equal(t, permission.ErrorPermissionDenied, err)
}

func TestBatchEditTool_ValidationErrors(t *testing.T) {
	tool := createBatchEditTool(true)
	ctx := createTestContext()

	tests := []struct {
		name        string
		operations  []BatchEditOperation
		expectError string
	}{
		{
			name:        "empty operations",
			operations:  []BatchEditOperation{},
			expectError: "at least one operation is required",
		},
		{
			name: "missing file path",
			operations: []BatchEditOperation{
				{FilePath: "", OldString: "old", NewString: "new"},
			},
			expectError: "file_path is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := BatchEditParams{Operations: tt.operations}
			input, err := json.Marshal(params)
			require.NoError(t, err)

			call := ToolCall{
				ID:    "test",
				Name:  BatchEditToolName,
				Input: string(input),
			}

			response, err := tool.Run(ctx, call)
			require.NoError(t, err)
			assert.True(t, response.IsError)
			assert.Contains(t, response.Content, tt.expectError)
		})
	}
}

func TestBatchEditTool_NonUniqueString(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	tool := createBatchEditTool(true)
	ctx := createTestContext()

	// Create test file with duplicate content
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "duplicate\nsome content\nduplicate"
	err := os.WriteFile(testFile, []byte(content), 0o644)
	require.NoError(t, err)

	recordFileRead(testFile)

	operations := []BatchEditOperation{
		{
			FilePath:  testFile,
			OldString: "duplicate", // This appears twice
			NewString: "unique",
		},
	}

	params := BatchEditParams{Operations: operations}
	input, err := json.Marshal(params)
	require.NoError(t, err)

	call := ToolCall{
		ID:    "test",
		Name:  BatchEditToolName,
		Input: string(input),
	}

	response, err := tool.Run(ctx, call)
	require.NoError(t, err)
	assert.True(t, response.IsError)
	
	// Check metadata for the specific error
	var metadata BatchEditResponseMetadata
	err = json.Unmarshal([]byte(response.Metadata), &metadata)
	require.NoError(t, err)
	
	assert.Equal(t, 1, metadata.TotalFailed)
	assert.Len(t, metadata.Results, 1)
	assert.False(t, metadata.Results[0].Success)
	assert.Contains(t, metadata.Results[0].Error, "appears multiple times")
}

// Benchmark tests
func BenchmarkBatchEditVsSingleEdits(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "batch-edit-bench")
	require.NoError(b, err)
	defer os.RemoveAll(tmpDir)

	// Initialize config
	_, err = config.Load(tmpDir, false)
	require.NoError(b, err)

	ctx := createTestContext()

	// Create test content with multiple replaceable sections
	content := strings.Repeat("line 1\nline 2\nline 3\nline 4\n", 100)

	b.Run("BatchEdit", func(b *testing.B) {
		for b.Loop() {
			// Create fresh test file
			testFile := filepath.Join(tmpDir, "batch_test.txt")
			err := os.WriteFile(testFile, []byte(content), 0o644)
			require.NoError(b, err)
			recordFileRead(testFile)

			tool := createBatchEditTool(true)

			// Prepare 10 operations
			operations := make([]BatchEditOperation, 10)
			for i := 0; i < 10; i++ {
				operations[i] = BatchEditOperation{
					FilePath:  testFile,
					OldString: "line 1",
					NewString: "modified line 1",
				}
			}

			params := BatchEditParams{Operations: operations}
			input, err := json.Marshal(params)
			require.NoError(b, err)

			call := ToolCall{
				ID:    "test",
				Name:  BatchEditToolName,
				Input: string(input),
			}

			_, err = tool.Run(ctx, call)
			require.NoError(b, err)

			// Cleanup
			os.Remove(testFile)
		}
	})

	b.Run("SingleEdits", func(b *testing.B) {
		for b.Loop() {
			// Create fresh test file
			testFile := filepath.Join(tmpDir, "single_test.txt")
			err := os.WriteFile(testFile, []byte(content), 0o644)
			require.NoError(b, err)

			// Apply 10 individual operations
			for i := 0; i < 10; i++ {
				recordFileRead(testFile)

				params := EditParams{
					FilePath:  testFile,
					OldString: "line 1",
					NewString: "modified line 1",
				}
				input, err := json.Marshal(params)
				require.NoError(b, err)

				call := ToolCall{
					ID:    "test",
					Name:  EditToolName,
					Input: string(input),
				}

				editTool := &editTool{
					lspClients:  make(map[string]*lsp.Client),
					permissions: &mockPermissionService{allowed: true},
					files:       &mockHistoryService{},
				}

				_, err = editTool.Run(ctx, call)
				require.NoError(b, err)
			}

			// Cleanup
			os.Remove(testFile)
		}
	})
}

func BenchmarkBatchEditFileOperations(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "batch-edit-file-bench")
	require.NoError(b, err)
	defer os.RemoveAll(tmpDir)

	// Initialize config
	_, err = config.Load(tmpDir, false)
	require.NoError(b, err)

	ctx := createTestContext()
	tool := createBatchEditTool(true)

	// Test different file sizes
	fileSizes := []int{1000, 10000, 100000}

	for _, size := range fileSizes {
		content := strings.Repeat("line of text\n", size/13) // Approximate size

		b.Run(fmt.Sprintf("FileSize_%d", size), func(b *testing.B) {
			b.ReportAllocs()

			for b.Loop() {
				testFile := filepath.Join(tmpDir, fmt.Sprintf("test_%d.txt", size))
				err := os.WriteFile(testFile, []byte(content), 0o644)
				require.NoError(b, err)
				recordFileRead(testFile)

				operations := []BatchEditOperation{
					{
						FilePath:  testFile,
						OldString: "line of text",
						NewString: "modified line",
					},
				}

				params := BatchEditParams{Operations: operations}
				input, err := json.Marshal(params)
				require.NoError(b, err)

				call := ToolCall{
					ID:    "test",
					Name:  BatchEditToolName,
					Input: string(input),
				}

				_, err = tool.Run(ctx, call)
				require.NoError(b, err)

				os.Remove(testFile)
			}
		})
	}
}

func BenchmarkBatchEditOperationCount(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "batch-edit-ops-bench")
	require.NoError(b, err)
	defer os.RemoveAll(tmpDir)

	// Initialize config
	_, err = config.Load(tmpDir, false)
	require.NoError(b, err)

	ctx := createTestContext()
	tool := createBatchEditTool(true)

	// Test different numbers of operations
	opCounts := []int{1, 5, 10, 25, 50}

	for _, count := range opCounts {
		b.Run(fmt.Sprintf("Operations_%d", count), func(b *testing.B) {
			b.ReportAllocs()

			for b.Loop() {
				// Create content with enough unique strings to replace
				var contentParts []string
				for i := 0; i < count; i++ {
					contentParts = append(contentParts, fmt.Sprintf("replace_me_%d\n", i))
				}
				content := strings.Join(contentParts, "")

				testFile := filepath.Join(tmpDir, fmt.Sprintf("test_%d.txt", count))
				err := os.WriteFile(testFile, []byte(content), 0o644)
				require.NoError(b, err)
				recordFileRead(testFile)

				// Create operations
				operations := make([]BatchEditOperation, count)
				for i := 0; i < count; i++ {
					operations[i] = BatchEditOperation{
						FilePath:  testFile,
						OldString: fmt.Sprintf("replace_me_%d", i),
						NewString: fmt.Sprintf("replaced_%d", i),
					}
				}

				params := BatchEditParams{Operations: operations}
				input, err := json.Marshal(params)
				require.NoError(b, err)

				call := ToolCall{
					ID:    "test",
					Name:  BatchEditToolName,
					Input: string(input),
				}

				_, err = tool.Run(ctx, call)
				require.NoError(b, err)

				os.Remove(testFile)
			}
		})
	}
}

func BenchmarkBatchEditMemoryUsage(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "batch-edit-memory-bench")
	require.NoError(b, err)
	defer os.RemoveAll(tmpDir)

	// Initialize config
	_, err = config.Load(tmpDir, false)
	require.NoError(b, err)

	ctx := createTestContext()
	tool := createBatchEditTool(true)

	// Large file content
	largeContent := strings.Repeat("This is a line of text that will be repeated many times.\n", 10000)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		testFile := filepath.Join(tmpDir, "large_test.txt")
		err := os.WriteFile(testFile, []byte(largeContent), 0o644)
		require.NoError(b, err)
		recordFileRead(testFile)

		operations := []BatchEditOperation{
			{
				FilePath:  testFile,
				OldString: "This is a line",
				NewString: "This is a modified line",
			},
		}

		params := BatchEditParams{Operations: operations}
		input, err := json.Marshal(params)
		require.NoError(b, err)

		call := ToolCall{
			ID:    "test",
			Name:  BatchEditToolName,
			Input: string(input),
		}

		_, err = tool.Run(ctx, call)
		require.NoError(b, err)

		os.Remove(testFile)
	}
}