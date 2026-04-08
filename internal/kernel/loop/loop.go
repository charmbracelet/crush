package loop

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// Transition represents possible state transitions in the turn loop
type Transition string

const (
	TransitionContinue         Transition = "continue"
	TransitionCompact          Transition = "compact"
	TransitionRetry           Transition = "retry"
	TransitionFallback        Transition = "fallback"
	TransitionError          Transition = "error"
	TransitionComplete       Transition = "complete"
	TransitionSpawnSubagent  Transition = "spawn_subagent"
	TransitionContinueSubagent Transition = "continue_subagent"
)

// State represents the current state of the turn loop
type State struct {
	TurnCount                      int
	Transition                     Transition
	AutoCompactTracking            int
	HasAttemptedReactiveCompact    bool
	MaxOutputTokensOverride        int
	LastToolSignature              string
	ConsecutiveIdenticalToolCalls  int
	ErrorCount                     int
	LastError                      error
}

// NewState creates a new turn loop state
func NewState() *State {
	return &State{
		TurnCount:               0,
		Transition:              TransitionContinue,
		MaxOutputTokensOverride: 0,
	}
}

// ToolCallSignature computes a signature for tool call sequence
func ToolCallSignature(messages []string) string {
	h := sha256.New()
	for _, msg := range messages {
		h.Write([]byte(msg))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// ShouldTransition determines if a state transition should occur
func (s *State) ShouldTransition(transition Transition) bool {
	if s.Transition == transition {
		return false
	}
	s.Transition = transition
	return true
}

// RecordToolCall records a tool call and checks for loops
func (s *State) RecordToolCall(signature string) bool {
	if s.LastToolSignature == signature {
		s.ConsecutiveIdenticalToolCalls++
		if s.ConsecutiveIdenticalToolCalls >= 3 {
			return true // Loop detected
		}
	} else {
		s.ConsecutiveIdenticalToolCalls = 0
		s.LastToolSignature = signature
	}
	return false
}

// IncrementTurn advances the turn counter
func (s *State) IncrementTurn() {
	s.TurnCount++
}

// RecordError records an error and increments error count
func (s *State) RecordError(err error) {
	s.ErrorCount++
	s.LastError = err
}

// ShouldRetry determines if the loop should retry
func (s *State) ShouldRetry(maxRetries int) bool {
	return s.ErrorCount < maxRetries && s.Transition == TransitionRetry
}

// CircuitBreakerState represents the circuit breaker state
type CircuitBreakerState int

const (
	CircuitBreakerClosed CircuitBreakerState = iota
	CircuitBreakerOpen
	CircuitBreakerHalfOpen
)

// String returns the string representation of CircuitBreakerState
func (s CircuitBreakerState) String() string {
	switch s {
	case CircuitBreakerClosed:
		return "closed"
	case CircuitBreakerOpen:
		return "open"
	case CircuitBreakerHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreaker implements the circuit breaker pattern for the loop
type CircuitBreaker struct {
	FailureThreshold int
	SuccessThreshold int
	Timeout          time.Duration
	State            CircuitBreakerState
	FailureCount     int
	SuccessCount     int
	LastFailureTime  time.Time
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(failureThreshold, successThreshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		FailureThreshold: failureThreshold,
		SuccessThreshold: successThreshold,
		Timeout:         timeout,
		State:           CircuitBreakerClosed,
	}
}

// RecordFailure records a failure and potentially opens the circuit
func (cb *CircuitBreaker) RecordFailure() {
	cb.FailureCount++
	cb.SuccessCount = 0
	cb.LastFailureTime = time.Now()

	if cb.FailureCount >= cb.FailureThreshold {
		cb.State = CircuitBreakerOpen
	}
}

// RecordSuccess records a success and potentially closes the circuit
func (cb *CircuitBreaker) RecordSuccess() {
	cb.SuccessCount++
	cb.FailureCount = 0

	if cb.SuccessCount >= cb.SuccessThreshold {
		cb.State = CircuitBreakerClosed
	}
}

// AllowRequest checks if a request should be allowed
func (cb *CircuitBreaker) AllowRequest() bool {
	switch cb.State {
	case CircuitBreakerClosed:
		return true
	case CircuitBreakerOpen:
		if time.Since(cb.LastFailureTime) > cb.Timeout {
			cb.State = CircuitBreakerHalfOpen
			return true
		}
		return false
	case CircuitBreakerHalfOpen:
		return true
	default:
		return false
	}
}

// String returns the string representation of the circuit breaker state
func (cb *CircuitBreaker) String() string {
	switch cb.State {
	case CircuitBreakerClosed:
		return "closed"
	case CircuitBreakerOpen:
		return "open"
	case CircuitBreakerHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// ErrorWithholdingMiddleware provides error withholding capability
type ErrorWithholdingMiddleware struct {
	ErrorDelay     time.Duration
	MaxErrors      int
	Errors         []error
	SuppressErrors bool
}

// NewErrorWithholdingMiddleware creates a new error withholding middleware
func NewErrorWithholdingMiddleware(errorDelay time.Duration, maxErrors int) *ErrorWithholdingMiddleware {
	return &ErrorWithholdingMiddleware{
		ErrorDelay:     errorDelay,
		MaxErrors:      maxErrors,
		Errors:         make([]error, 0),
		SuppressErrors: false,
	}
}

// RecordError records an error with optional delay
func (ew *ErrorWithholdingMiddleware) RecordError(err error) {
	ew.Errors = append(ew.Errors, err)
	if len(ew.Errors) > ew.MaxErrors {
		ew.SuppressErrors = true
	}
}

// GetErrors returns recorded errors
func (ew *ErrorWithholdingMiddleware) GetErrors() []error {
	return ew.Errors
}

// ShouldSuppress returns whether errors should be suppressed
func (ew *ErrorWithholdingMiddleware) ShouldSuppress() bool {
	return ew.SuppressErrors
}

// Clear clears all recorded errors
func (ew *ErrorWithholdingMiddleware) Clear() {
	ew.Errors = make([]error, 0)
	ew.SuppressErrors = false
}

// TurnLoop implements the explicit state machine loop
type TurnLoop struct {
	State            *State
	CircuitBreaker   *CircuitBreaker
	ErrorWithholding *ErrorWithholdingMiddleware
	Hooks            []TurnHook
}

// TurnHook is a function that runs at each turn
type TurnHook func(*State) error

// NewTurnLoop creates a new turn loop
func NewTurnLoop() *TurnLoop {
	return &TurnLoop{
		State:            NewState(),
		CircuitBreaker:   NewCircuitBreaker(3, 2, 30*time.Second),
		ErrorWithholding: NewErrorWithholdingMiddleware(100*time.Millisecond, 5),
		Hooks:            make([]TurnHook, 0),
	}
}

// AddHook adds a hook to the turn loop
func (tl *TurnLoop) AddHook(hook TurnHook) {
	tl.Hooks = append(tl.Hooks, hook)
}

// ExecuteTurn executes a single turn
func (tl *TurnLoop) ExecuteTurn(messages []string, transition Transition) error {
	// Check circuit breaker
	if !tl.CircuitBreaker.AllowRequest() {
		return fmt.Errorf("circuit breaker is %s", tl.CircuitBreaker.State)
	}

	// Update state
	tl.State.ShouldTransition(transition)
	tl.State.IncrementTurn()

	// Execute hooks
	for _, hook := range tl.Hooks {
		if err := hook(tl.State); err != nil {
			tl.State.RecordError(err)
			tl.CircuitBreaker.RecordFailure()
			return err
		}
	}

	// Check for loops
	sig := ToolCallSignature(messages)
	if tl.State.RecordToolCall(sig) {
		tl.CircuitBreaker.RecordFailure()
		return fmt.Errorf("loop detected: identical tool calls repeated %d times", tl.State.ConsecutiveIdenticalToolCalls)
	}

	tl.CircuitBreaker.RecordSuccess()
	return nil
}

// GetState returns the current state
func (tl *TurnLoop) GetState() *State {
	return tl.State
}

// Reset resets the turn loop state
func (tl *TurnLoop) Reset() {
	tl.State = NewState()
	tl.CircuitBreaker.State = CircuitBreakerClosed
	tl.CircuitBreaker.FailureCount = 0
	tl.CircuitBreaker.SuccessCount = 0
	tl.ErrorWithholding.Clear()
}
