package projects

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRegisterAndList(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir := t.TempDir()

	// Override the projects file path for testing
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("CRUSH_GLOBAL_DATA", filepath.Join(tmpDir, "crush"))

	// Test registering a project
	err := Register("/home/user/project1", "/home/user/project1/.crush")
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// List projects
	projects, err := List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(projects) != 1 {
		t.Fatalf("Expected 1 project, got %d", len(projects))
	}

	if projects[0].Path != "/home/user/project1" {
		t.Errorf("Expected path /home/user/project1, got %s", projects[0].Path)
	}

	if projects[0].DataDir != "/home/user/project1/.crush" {
		t.Errorf("Expected data_dir /home/user/project1/.crush, got %s", projects[0].DataDir)
	}

	// Register another project
	err = Register("/home/user/project2", "/home/user/project2/.crush")
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	projects, err = List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(projects) != 2 {
		t.Fatalf("Expected 2 projects, got %d", len(projects))
	}

	// Most recent should be first
	if projects[0].Path != "/home/user/project2" {
		t.Errorf("Expected most recent project first, got %s", projects[0].Path)
	}
}

func TestRegisterUpdatesExisting(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("CRUSH_GLOBAL_DATA", filepath.Join(tmpDir, "crush"))

	// Register a project
	err := Register("/home/user/project1", "/home/user/project1/.crush")
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	projects, _ := List()
	firstAccess := projects[0].LastAccessed

	// Wait a bit and re-register
	time.Sleep(10 * time.Millisecond)

	err = Register("/home/user/project1", "/home/user/project1/.crush-new")
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	projects, _ = List()

	if len(projects) != 1 {
		t.Fatalf("Expected 1 project after update, got %d", len(projects))
	}

	if projects[0].DataDir != "/home/user/project1/.crush-new" {
		t.Errorf("Expected updated data_dir, got %s", projects[0].DataDir)
	}

	if !projects[0].LastAccessed.After(firstAccess) {
		t.Error("Expected LastAccessed to be updated")
	}
}

func TestLoadEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("CRUSH_GLOBAL_DATA", filepath.Join(tmpDir, "crush"))

	// List before any projects exist
	projects, err := List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(projects) != 0 {
		t.Errorf("Expected 0 projects, got %d", len(projects))
	}
}

func TestProjectsFilePath(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("CRUSH_GLOBAL_DATA", filepath.Join(tmpDir, "crush"))

	expected := filepath.Join(tmpDir, "crush", "projects.json")
	actual := projectsFilePath()

	if actual != expected {
		t.Errorf("Expected %s, got %s", expected, actual)
	}
}

func TestRegisterWithParentDataDir(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("CRUSH_GLOBAL_DATA", filepath.Join(tmpDir, "crush"))

	// Register a project where .crush is in a parent directory.
	// e.g., working in /home/user/monorepo/packages/app but .crush is at /home/user/monorepo/.crush
	err := Register("/home/user/monorepo/packages/app", "/home/user/monorepo/.crush")
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	projects, err := List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(projects) != 1 {
		t.Fatalf("Expected 1 project, got %d", len(projects))
	}

	if projects[0].Path != "/home/user/monorepo/packages/app" {
		t.Errorf("Expected path /home/user/monorepo/packages/app, got %s", projects[0].Path)
	}

	if projects[0].DataDir != "/home/user/monorepo/.crush" {
		t.Errorf("Expected data_dir /home/user/monorepo/.crush, got %s", projects[0].DataDir)
	}
}

func TestRegisterWithExternalDataDir(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("CRUSH_GLOBAL_DATA", filepath.Join(tmpDir, "crush"))

	// Register a project where .crush is in a completely different location.
	// e.g., project at /home/user/project but data stored at /var/data/crush/myproject
	err := Register("/home/user/project", "/var/data/crush/myproject")
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	projects, err := List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(projects) != 1 {
		t.Fatalf("Expected 1 project, got %d", len(projects))
	}

	if projects[0].Path != "/home/user/project" {
		t.Errorf("Expected path /home/user/project, got %s", projects[0].Path)
	}

	if projects[0].DataDir != "/var/data/crush/myproject" {
		t.Errorf("Expected data_dir /var/data/crush/myproject, got %s", projects[0].DataDir)
	}
}

func TestRegisterOrdersByRecency(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("CRUSH_GLOBAL_DATA", filepath.Join(tmpDir, "crush"))

	for _, path := range []string{"/p1", "/p2", "/p3", "/p1"} {
		if err := Register(path, path+"/.crush"); err != nil {
			t.Fatalf("Register failed: %v", err)
		}
	}

	projects, err := List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	var got []string
	for _, p := range projects {
		got = append(got, p.Path)
	}
	want := []string{"/p1", "/p3", "/p2"}
	if !slices.Equal(got, want) {
		t.Fatalf("Expected order %v, got %v", want, got)
	}
}

// TestRegister_ConcurrentProcesses registers projects from several
// processes at once, as crush processes started together do, and checks
// that projects.json stays valid and keeps every project.
func TestRegister_ConcurrentProcesses(t *testing.T) {
	const (
		dirEnv   = "CRUSH_TEST_PROJECTS_DIR"
		idEnv    = "CRUSH_TEST_PROJECTS_CHILD"
		children = 8
		paths    = 5
		rounds   = 2
	)

	if dir := os.Getenv(dirEnv); dir != "" {
		// Child process. internal/home clears CRUSH_GLOBAL_DATA in test
		// binaries, so the parent passes the directory under another name.
		t.Setenv("CRUSH_GLOBAL_DATA", dir)
		// Wait until the parent has started every child so they overlap.
		_, err := io.Copy(io.Discard, os.Stdin)
		require.NoError(t, err)
		// Keep going after a failure, as crush does, so the parent sees
		// what the file is left in.
		var errs []error
		for i := range paths * rounds {
			path := fmt.Sprintf("/project/%s/%d", os.Getenv(idEnv), i%paths)
			errs = append(errs, Register(path, path+"/.crush"))
		}
		require.NoError(t, errors.Join(errs...))
		return
	}

	dir := filepath.Join(t.TempDir(), "crush")
	r, w, err := os.Pipe()
	require.NoError(t, err)
	var cmds []*exec.Cmd
	outputs := make([]bytes.Buffer, children)
	for i := range children {
		cmd := exec.CommandContext(t.Context(), os.Args[0],
			"-test.run=^TestRegister_ConcurrentProcesses$")
		cmd.Env = append(os.Environ(), dirEnv+"="+dir, fmt.Sprintf("%s=%d", idEnv, i))
		cmd.Stdin = r
		cmd.Stdout = &outputs[i]
		cmd.Stderr = &outputs[i]
		if err = cmd.Start(); err != nil {
			break
		}
		cmds = append(cmds, cmd)
	}
	// Closing the pipe releases every child at once. Reap them all before
	// asserting so none outlives the test.
	r.Close()
	w.Close()
	errs := make([]error, len(cmds))
	for i, cmd := range cmds {
		errs[i] = cmd.Wait()
	}
	require.NoError(t, err, "start child")

	data, err := os.ReadFile(filepath.Join(dir, projectsFileName))
	require.NoError(t, err)
	var list ProjectList
	require.NoError(t, json.Unmarshal(data, &list), "projects.json:\n%s", data)
	for i, err := range errs {
		require.NoError(t, err, "child %d:\n%s", i, &outputs[i])
	}
	require.Len(t, list.Projects, children*paths)
}
