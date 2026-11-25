// Package trial defines the seam between Crush and a free-trial service.
//
// Proposal E of the first-run redesign lets a machine with no provider
// configured use a Hyper model without an account, for a limited number of
// turns. Nothing in Crush can grant that on its own: it needs a service that
// hands out an anonymous credential, counts turns per machine and knows when
// a machine has already used its trial. This package only describes that
// service. Until one exists, [Disabled] is the implementation in use and
// every machine takes the "no key, no trial" path.
package trial

import (
	"context"
	"errors"
)

// ErrUnavailable means no trial can be offered: the service is offline, the
// machine already used its trial, or the trial is switched off.
var ErrUnavailable = errors.New("free trial is not available")

// Offer describes a trial a machine can start.
type Offer struct {
	// Provider is the provider the trial runs on, such as "hyper".
	Provider string
	// Model is the model ID the trial uses.
	Model string
	// ModelName is the display name of Model.
	ModelName string
	// Turns is how many turns the trial allows in total.
	Turns int
}

// Service is the trial backend.
type Service interface {
	// Check reports whether this machine can start the trial. It performs
	// network I/O, so callers must run it off the UI goroutine. It returns
	// [ErrUnavailable] when there is nothing to offer.
	Check(ctx context.Context) (Offer, error)
}

// Disabled is a [Service] that never offers a trial.
type Disabled struct{}

var _ Service = Disabled{}

// Check implements [Service].
func (Disabled) Check(context.Context) (Offer, error) {
	return Offer{}, ErrUnavailable
}
