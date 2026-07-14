// Package goal owns durable Goal mode state and continuation policy.
package goal

import "strings"

const MaxAutomaticTurns = 200

type Status string

const (
	StatusActive   Status = "active"
	StatusComplete Status = "complete"
	StatusBlocked  Status = "blocked"
	StatusPaused   Status = "paused"
)

type State struct {
	Objective string `json:"objective"`
	Status    Status `json:"status"`
	Turns     int    `json:"turns"`
	Summary   string `json:"summary,omitempty"`
}

func Start(objective string) State {
	return State{
		Objective: strings.TrimSpace(objective),
		Status:    StatusActive,
	}
}

func (s State) Active() bool {
	return s.Status == StatusActive && s.Objective != ""
}

func (s State) WithStatus(status Status, summary string) State {
	s.Status = status
	s.Summary = strings.TrimSpace(summary)
	return s
}

func (s State) NextTurn() State {
	s.Turns++
	return s
}

func (s State) Resume() State {
	s.Status = StatusActive
	return s
}
