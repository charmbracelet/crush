// Package profiling provides performance profiling utilities for the crush CLI.
package profiling

import (
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"sync"
)

var (
	cpuFile   *os.File
	memFile   *os.File
	traceFile *os.File
	mu        sync.Mutex
)

// Config holds profiling configuration.
type Config struct {
	CPUProfile string // Path to CPU profile file
	MemProfile string // Path to memory profile file
	TraceFile  string // Path to trace file
	BlockRate  int    // Block profiling rate (0 to disable)
	MutexFrac  int    // Mutex profiling fraction (0 to disable)
}

// Start begins profiling based on the provided configuration.
// Returns a cleanup function that should be deferred.
func Start(cfg *Config) (cleanup func(), err error) {
	mu.Lock()
	defer mu.Unlock()

	cleanups := []func(){}

	// CPU profiling
	if cfg.CPUProfile != "" {
		f, err := os.Create(cfg.CPUProfile)
		if err != nil {
			return nil, fmt.Errorf("creating CPU profile: %w", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			f.Close()
			return nil, fmt.Errorf("starting CPU profile: %w", err)
		}
		cpuFile = f
		slog.Info("CPU profiling started", "file", cfg.CPUProfile)
		cleanups = append(cleanups, func() {
			pprof.StopCPUProfile()
			if err := cpuFile.Close(); err != nil {
				slog.Error("Failed to close CPU profile", "error", err)
			}
			slog.Info("CPU profile written", "file", cfg.CPUProfile)
		})
	}

	// Trace
	if cfg.TraceFile != "" {
		f, err := os.Create(cfg.TraceFile)
		if err != nil {
			return nil, fmt.Errorf("creating trace file: %w", err)
		}
		if err := trace.Start(f); err != nil {
			f.Close()
			return nil, fmt.Errorf("starting trace: %w", err)
		}
		traceFile = f
		slog.Info("Execution trace started", "file", cfg.TraceFile)
		cleanups = append(cleanups, func() {
			trace.Stop()
			if err := traceFile.Close(); err != nil {
				slog.Error("Failed to close trace file", "error", err)
			}
			slog.Info("Trace written", "file", cfg.TraceFile)
		})
	}

	// Block profiling
	if cfg.BlockRate > 0 {
		runtime.SetBlockProfileRate(cfg.BlockRate)
		slog.Info("Block profiling enabled", "rate", cfg.BlockRate)
		cleanups = append(cleanups, func() {
			runtime.SetBlockProfileRate(0)
		})
	}

	// Mutex profiling
	if cfg.MutexFrac > 0 {
		runtime.SetMutexProfileFraction(cfg.MutexFrac)
		slog.Info("Mutex profiling enabled", "fraction", cfg.MutexFrac)
		cleanups = append(cleanups, func() {
			runtime.SetMutexProfileFraction(0)
		})
	}

	// Memory profiling (written at cleanup time)
	if cfg.MemProfile != "" {
		f, err := os.Create(cfg.MemProfile)
		if err != nil {
			return nil, fmt.Errorf("creating memory profile: %w", err)
		}
		memFile = f
		slog.Info("Memory profiling will be written at exit", "file", cfg.MemProfile)
		cleanups = append(cleanups, func() {
			runtime.GC() // Force GC before writing memory profile
			if err := pprof.WriteHeapProfile(memFile); err != nil {
				slog.Error("Failed to write memory profile", "error", err)
			}
			if err := memFile.Close(); err != nil {
				slog.Error("Failed to close memory profile", "error", err)
			}
			slog.Info("Memory profile written", "file", cfg.MemProfile)
		})
	}

	// Return a composite cleanup function
	return func() {
		mu.Lock()
		defer mu.Unlock()
		// Execute cleanups in reverse order (LIFO)
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}, nil
}

// WriteProfile writes a named profile to a file.
// Useful for on-demand profiling of specific profile types.
func WriteProfile(name, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("creating profile file: %w", err)
	}
	defer f.Close()

	profile := pprof.Lookup(name)
	if profile == nil {
		return fmt.Errorf("profile %q not found", name)
	}

	if err := profile.WriteTo(f, 0); err != nil {
		return fmt.Errorf("writing profile: %w", err)
	}

	slog.Info("Profile written", "name", name, "file", filename)
	return nil
}

// Available profile types:
// - "goroutine" - stack traces of all current goroutines
// - "heap" - sampling of memory allocations of live objects
// - "allocs" - sampling of all past memory allocations
// - "threadcreate" - stack traces that led to the creation of new OS threads
// - "block" - stack traces that led to blocking on synchronization primitives
// - "mutex" - stack traces of holders of contended mutexes
