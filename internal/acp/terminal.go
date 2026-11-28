package acp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	acp "github.com/coder/acp-go-sdk"
)

type terminalSysStatus interface {
	Signaled() bool
	Signal() os.Signal
	ExitStatus() int
}

type terminalProcess struct {
	mu        sync.Mutex
	cmd       *exec.Cmd
	output    []byte
	limit     int64
	truncated bool
	done      chan struct{}
	exitCode  *int64
	signal    *string
}

type terminalRegistry struct {
	mu        sync.Mutex
	processes map[acp.TerminalId]*terminalProcess
}

func newTerminalRegistry() *terminalRegistry {
	return &terminalRegistry{processes: make(map[acp.TerminalId]*terminalProcess)}
}

func (r *terminalRegistry) create(
	ctx context.Context,
	id acp.TerminalId,
	command string,
	args []string,
	env []acp.EnvVariable,
	cwd string,
	outputByteLimit *int64,
) (*terminalProcess, error) {
	if !filepath.IsAbs(cwd) {
		return nil, fmt.Errorf("cwd must be absolute: %q", cwd)
	}

	p := &terminalProcess{done: make(chan struct{})}
	if outputByteLimit != nil && *outputByteLimit > 0 {
		p.limit = *outputByteLimit
	}

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = cwd
	cmd.Env = buildTerminalEnv(env)
	cmd.Stdout = &terminalOutputWriter{p: p}
	cmd.Stderr = cmd.Stdout
	p.cmd = cmd

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start terminal: %w", err)
	}

	go p.wait()

	r.mu.Lock()
	r.processes[id] = p
	r.mu.Unlock()

	return p, nil
}

func terminalExitCode(exitErr *exec.ExitError) int64 {
	if status, ok := exitErr.Sys().(interface{ ExitStatus() int }); ok {
		return int64(status.ExitStatus())
	}
	return -1
}

func (p *terminalProcess) wait() {
	err := p.cmd.Wait()

	var code *int64
	var signal *string
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if status, ok := exitErr.Sys().(terminalSysStatus); ok && status.Signaled() {
				name := status.Signal().String()
				signal = &name
			} else {
				value := terminalExitCode(exitErr)
				code = &value
			}
		}
	}
	if code == nil && signal == nil {
		value := int64(0)
		code = &value
	}

	p.mu.Lock()
	p.exitCode, p.signal = code, signal
	p.mu.Unlock()
	close(p.done)
}

func (p *terminalProcess) appendOutput(b []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.output = append(p.output, b...)
	if p.limit <= 0 || int64(len(p.output)) <= p.limit {
		return
	}

	cut := int(int64(len(p.output)) - p.limit)
	for cut < len(p.output) && !utf8.RuneStart(p.output[cut]) {
		cut++
	}
	p.output = append([]byte(nil), p.output[cut:]...)
	p.truncated = true
}

type terminalOutputWriter struct {
	p *terminalProcess
}

func (w *terminalOutputWriter) Write(b []byte) (int, error) {
	w.p.appendOutput(b)
	return len(b), nil
}

func (r *terminalRegistry) get(id acp.TerminalId) *terminalProcess {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.processes[id]
}

func (r *terminalRegistry) output(id acp.TerminalId) (string, bool, *acp.TerminalExitStatus, bool) {
	p := r.get(id)
	if p == nil {
		return "", false, nil, false
	}

	p.mu.Lock()
	text := string(p.output)
	truncated := p.truncated
	var status *acp.TerminalExitStatus
	if p.exitCode != nil || p.signal != nil {
		status = &acp.TerminalExitStatus{ExitCode: p.exitCode, Signal: p.signal}
	}
	p.mu.Unlock()

	return text, truncated, status, true
}

func (r *terminalRegistry) wait(ctx context.Context, id acp.TerminalId) (*int64, *string, bool) {
	p := r.get(id)
	if p == nil {
		return nil, nil, false
	}

	select {
	case <-ctx.Done():
		return nil, nil, false
	case <-p.done:
		p.mu.Lock()
		code, signal := p.exitCode, p.signal
		p.mu.Unlock()
		return code, signal, true
	}
}

func (p *terminalProcess) running() bool {
	select {
	case <-p.done:
		return false
	default:
		return true
	}
}

func (r *terminalRegistry) kill(id acp.TerminalId) bool {
	p := r.get(id)
	if p == nil || !p.running() {
		return false
	}
	return p.cmd.Process.Kill() == nil
}

func (r *terminalRegistry) release(id acp.TerminalId) bool {
	r.mu.Lock()
	p := r.processes[id]
	delete(r.processes, id)
	r.mu.Unlock()

	if p == nil {
		return false
	}
	if p.running() {
		_ = p.cmd.Process.Kill()
	}
	return true
}

func buildTerminalEnv(overrides []acp.EnvVariable) []string {
	if len(overrides) == 0 {
		return os.Environ()
	}

	env := os.Environ()
	for _, v := range overrides {
		prefix := v.Name + "="
		for i := 0; i < len(env); {
			if strings.HasPrefix(env[i], prefix) {
				env = append(env[:i], env[i+1:]...)
				continue
			}
			i++
		}
		env = append(env, prefix+v.Value)
	}
	return env
}
