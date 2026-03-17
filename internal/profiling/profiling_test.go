package profiling

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStartCPUProfile(t *testing.T) {
	tmpDir := t.TempDir()
	cpuFile := filepath.Join(tmpDir, "cpu.prof")

	cfg := &Config{
		CPUProfile: cpuFile,
	}

	cleanup, err := Start(cfg)
	if err != nil {
		t.Fatalf("Failed to start profiling: %v", err)
	}

	// Simulate some work
	sum := 0
	for i := 0; i < 1000000; i++ {
		sum += i
	}

	cleanup()

	// Verify file was created
	if _, err := os.Stat(cpuFile); os.IsNotExist(err) {
		t.Errorf("CPU profile file was not created")
	}
}

func TestStartMemProfile(t *testing.T) {
	tmpDir := t.TempDir()
	memFile := filepath.Join(tmpDir, "mem.prof")

	cfg := &Config{
		MemProfile: memFile,
	}

	cleanup, err := Start(cfg)
	if err != nil {
		t.Fatalf("Failed to start profiling: %v", err)
	}

	// Simulate some allocations
	data := make([][]byte, 1000)
	for i := range data {
		data[i] = make([]byte, 1024)
	}

	cleanup()

	// Verify file was created
	if _, err := os.Stat(memFile); os.IsNotExist(err) {
		t.Errorf("Memory profile file was not created")
	}
}

func TestStartTrace(t *testing.T) {
	tmpDir := t.TempDir()
	traceFile := filepath.Join(tmpDir, "trace.out")

	cfg := &Config{
		TraceFile: traceFile,
	}

	cleanup, err := Start(cfg)
	if err != nil {
		t.Fatalf("Failed to start profiling: %v", err)
	}

	// Simulate some work
	done := make(chan bool)
	go func() {
		sum := 0
		for i := 0; i < 100000; i++ {
			sum += i
		}
		done <- true
	}()
	<-done

	cleanup()

	// Verify file was created
	if _, err := os.Stat(traceFile); os.IsNotExist(err) {
		t.Errorf("Trace file was not created")
	}
}

func TestMultipleProfiles(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		CPUProfile: filepath.Join(tmpDir, "cpu.prof"),
		MemProfile: filepath.Join(tmpDir, "mem.prof"),
		TraceFile:  filepath.Join(tmpDir, "trace.out"),
		BlockRate:  1,
		MutexFrac:  1,
	}

	cleanup, err := Start(cfg)
	if err != nil {
		t.Fatalf("Failed to start profiling: %v", err)
	}

	// Simulate work
	sum := 0
	for i := 0; i < 100000; i++ {
		sum += i
	}

	cleanup()

	// Verify all files were created
	files := []string{"cpu.prof", "mem.prof", "trace.out"}
	for _, file := range files {
		path := filepath.Join(tmpDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Profile file %s was not created", file)
		}
	}
}

func TestWriteProfile(t *testing.T) {
	tmpDir := t.TempDir()
	goroutineFile := filepath.Join(tmpDir, "goroutine.prof")

	// Create some goroutines
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			<-done
		}()
	}

	err := WriteProfile("goroutine", goroutineFile)
	if err != nil {
		t.Fatalf("Failed to write goroutine profile: %v", err)
	}

	close(done)

	// Verify file was created
	if _, err := os.Stat(goroutineFile); os.IsNotExist(err) {
		t.Errorf("Goroutine profile file was not created")
	}
}
